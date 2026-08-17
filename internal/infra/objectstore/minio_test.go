package objectstore

import (
	"context"
	"testing"
	"time"
)

func TestMinIOGuards(t *testing.T) {
	var storage *MinIO
	if err := storage.HealthCheck(context.Background()); err == nil {
		t.Fatal("nil storage health check succeeded")
	}
	if _, err := storage.List(context.Background(), "blobs/", time.Time{}); err == nil {
		t.Fatal("nil storage list succeeded")
	}
	storage = &MinIO{}
	if err := storage.HealthCheck(context.Background()); err == nil {
		t.Fatal("uninitialized storage health check succeeded")
	}
	if _, err := storage.List(context.Background(), "blobs/", time.Time{}); err == nil {
		t.Fatal("uninitialized storage list succeeded")
	}
	if _, err := storage.GetURL(context.Background(), "key", 0); err == nil {
		t.Fatal("non-positive URL expiration accepted")
	}
}
