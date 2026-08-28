package contracts

import (
	"context"
	"io"
	"time"
)

// ObjectStorage предоставляет операции с объектным хранилищем.
type ObjectStorage interface {
	Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// StoredObject описывает объект, найденный в хранилище.
type StoredObject struct {
	Key          string
	LastModified time.Time
}

// ObjectStorageLister перечисляет объекты хранилища по префиксу.
type ObjectStorageLister interface {
	List(ctx context.Context, prefix string, olderThan time.Time) ([]StoredObject, error)
}
