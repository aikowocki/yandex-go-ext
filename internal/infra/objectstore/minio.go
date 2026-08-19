package objectstore

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO работает с объектами в MinIO.
type MinIO struct {
	client *minio.Client
	bucket string
}

// NewMinIO создаёт клиент MinIO и проверяет bucket.
func NewMinIO(ctx context.Context, cfg config.S3Config) (*MinIO, error) {
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

	storage := &MinIO{client: client, bucket: cfg.Bucket}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check storage bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("create storage bucket: %w", err)
		}
		logging.Info(ctx, "created object storage bucket", logging.String("bucket", cfg.Bucket))
	}
	return storage, nil
}

// Upload загружает объект в MinIO.
func (s *MinIO) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	if _, err := s.client.PutObject(ctx, s.bucket, key, data, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("upload object %q: %w", key, err)
	}
	return nil
}

// Download открывает объект из MinIO.
func (s *MinIO) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("download object %q: %w", key, err)
	}
	return object, nil
}

// Delete удаляет объект из MinIO.
func (s *MinIO) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

// GetURL создаёт временную ссылку на объект.
func (s *MinIO) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if expires <= 0 {
		return "", fmt.Errorf("url expiration must be positive")
	}
	url, err := s.client.PresignedGetObject(ctx, s.bucket, key, expires, nil)
	if err != nil {
		return "", fmt.Errorf("create object url %q: %w", key, err)
	}
	return url.String(), nil
}

// HealthCheck проверяет доступность bucket-а MinIO.
func (s *MinIO) HealthCheck(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("object storage client is not initialized")
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check storage bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("storage bucket %q does not exist", s.bucket)
	}
	return nil
}

// List возвращает объекты MinIO по префиксу.
func (s *MinIO) List(ctx context.Context, prefix string, olderThan time.Time) ([]contracts.StoredObject, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("list storage objects: storage client is not initialized")
	}
	objects := make([]contracts.StoredObject, 0)
	for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, fmt.Errorf("list storage objects: %w", object.Err)
		}
		if !olderThan.IsZero() && !object.LastModified.Before(olderThan) {
			continue
		}
		objects = append(objects, contracts.StoredObject{Key: object.Key, LastModified: object.LastModified})
	}
	return objects, nil
}
