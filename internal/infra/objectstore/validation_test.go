package objectstore

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/minio/minio-go/v7"
)

func TestValidateConfig(t *testing.T) {
	valid := config.S3Config{Endpoint: "localhost:9000", AccessKeyID: "key", SecretAccessKey: "secret", Bucket: "avatars"}
	tests := []struct {
		name  string
		cfg   config.S3Config
		valid bool
	}{
		{name: "empty endpoint", cfg: config.S3Config{AccessKeyID: valid.AccessKeyID, SecretAccessKey: valid.SecretAccessKey, Bucket: valid.Bucket}},
		{name: "empty access key", cfg: config.S3Config{Endpoint: valid.Endpoint, SecretAccessKey: valid.SecretAccessKey, Bucket: valid.Bucket}},
		{name: "empty secret key", cfg: config.S3Config{Endpoint: valid.Endpoint, AccessKeyID: valid.AccessKeyID, Bucket: valid.Bucket}},
		{name: "empty bucket", cfg: config.S3Config{Endpoint: valid.Endpoint, AccessKeyID: valid.AccessKeyID, SecretAccessKey: valid.SecretAccessKey}},
		{name: "valid", cfg: valid, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.valid && err != nil {
				t.Fatalf("validateConfig() error = %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("validateConfig() accepted invalid config")
			}
		})
	}
}

func TestIsBucketAlreadyOwned(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "owned by current account", code: "BucketAlreadyOwnedByYou", want: true},
		{name: "owned by another account", code: "BucketAlreadyExists", want: false},
		{name: "other error", code: "AccessDenied", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBucketAlreadyOwned(minio.ErrorResponse{Code: tt.code})
			if got != tt.want {
				t.Fatalf("isBucketAlreadyOwned() = %v, want %v", got, tt.want)
			}
		})
	}
}
