package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/migrations"
)

func TestNewPoolRejectsInvalidDSNAndCancelledPing(t *testing.T) {
	if _, err := NewPool(context.Background(), config.DatabaseConfig{DSN: "not a dsn"}); err == nil || !strings.Contains(err.Error(), "parse database dsn") {
		t.Fatalf("invalid DSN error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewPool(ctx, config.DatabaseConfig{DSN: "postgres://user:pass@localhost:5432/db"})
	if err == nil {
		t.Fatal("cancelled database context accepted")
	}
}

func TestMigrationsRejectEmptyDSN(t *testing.T) {
	if err := migrations.Up(""); err == nil {
		t.Fatal("empty database DSN accepted")
	}
}
