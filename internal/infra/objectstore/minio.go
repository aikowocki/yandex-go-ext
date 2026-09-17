package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/aikowocki/yandex-go-ext/internal/shared/resilience"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO работает с объектами в MinIO.
type MinIO struct {
	logger  logging.Logger
	client  *minio.Client
	bucket  string
	breaker *resilience.Breaker
}

// NewMinIO создаёт клиент MinIO и проверяет bucket.
func NewMinIO(ctx context.Context, cfg config.S3Config, logger logging.Logger) (*MinIO, error) {
	if logger == nil {
		logger = logging.ComponentLogger(nil, "storage")
	}
	ctx = logging.WithLogger(ctx, logger)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create object storage client: %w", err)
	}

	storage := &MinIO{logger: logger, client: client, bucket: cfg.Bucket, breaker: resilience.New(resilience.Config{})}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check storage bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			if !isBucketAlreadyOwned(err) {
				return nil, fmt.Errorf("create storage bucket: %w", err)
			}
		} else {
			storage.logger.Info(ctx, "created object storage bucket", logging.String("bucket", cfg.Bucket))
		}
	}
	return storage, nil
}

func isBucketAlreadyOwned(err error) bool {
	response := minio.ToErrorResponse(err)
	return response.Code == "BucketAlreadyOwnedByYou"
}

func (s *MinIO) execute(ctx context.Context, fn func() error) error {
	return s.breaker.Do(ctx, isTransientError, func() error {
		return withRetry(ctx, fn)
	})
}

func (s *MinIO) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) (err error) {
	ctx, span := startStorageSpan(ctx, "upload")
	defer func() { finishStorageSpan(span, err) }()
	if err := s.execute(ctx, func() error {
		_, err := s.client.PutObject(ctx, s.bucket, key, data, size, minio.PutObjectOptions{ContentType: contentType})
		return err
	}); err != nil {
		return fmt.Errorf("upload object %q: %w", key, err)
	}
	return nil
}

// Download открывает объект из MinIO.
func (s *MinIO) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	ctx, span := startStorageSpan(ctx, "download")
	var object *minio.Object
	err := s.execute(ctx, func() error {
		var err error
		object, err = s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
		return err
	})
	if err != nil {
		finishStorageSpan(span, err)
		return nil, fmt.Errorf("download object %q: %w", key, err)
	}
	return &tracedReadCloser{ReadCloser: object, span: span}, nil
}

// Delete удаляет объект из MinIO.
func (s *MinIO) Delete(ctx context.Context, key string) (err error) {
	ctx, span := startStorageSpan(ctx, "delete")
	defer func() { finishStorageSpan(span, err) }()
	if err := s.execute(ctx, func() error {
		return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	}); err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

// GetURL создаёт временную ссылку на объект.
func (s *MinIO) GetURL(ctx context.Context, key string, expires time.Duration) (result string, err error) {
	ctx, span := startStorageSpan(ctx, "get_url")
	defer func() { finishStorageSpan(span, err) }()
	if expires <= 0 {
		return "", fmt.Errorf("url expiration must be positive")
	}
	var objUrl *url.URL
	err = s.execute(ctx, func() error {
		var err error
		objUrl, err = s.client.PresignedGetObject(ctx, s.bucket, key, expires, nil)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("create object url %q: %w", key, err)
	}
	return objUrl.String(), nil
}

// HealthCheck проверяет доступность bucket-а MinIO.
func (s *MinIO) HealthCheck(ctx context.Context) (err error) {
	ctx, span := startStorageSpan(ctx, "health_check")
	defer func() { finishStorageSpan(span, err) }()
	if s == nil || s.client == nil {
		return fmt.Errorf("object storage client is not initialized")
	}
	exists := false
	if err := s.execute(ctx, func() error {
		var err error
		exists, err = s.client.BucketExists(ctx, s.bucket)
		return err
	}); err != nil {
		return fmt.Errorf("check storage bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("storage bucket %q does not exist", s.bucket)
	}
	return nil
}

// List возвращает объекты MinIO по префиксу.
func (s *MinIO) List(ctx context.Context, prefix string, olderThan time.Time) (result []contracts.StoredObject, err error) {
	ctx, span := startStorageSpan(ctx, "list")
	defer func() { finishStorageSpan(span, err) }()
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("list storage objects: storage client is not initialized")
	}
	objects := make([]contracts.StoredObject, 0)
	if err := s.execute(ctx, func() error {
		for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err != nil {
				return object.Err
			}
			if !olderThan.IsZero() && !object.LastModified.Before(olderThan) {
				continue
			}
			objects = append(objects, contracts.StoredObject{Key: object.Key, LastModified: object.LastModified})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list storage objects: %w", err)
	}
	return objects, nil
}
