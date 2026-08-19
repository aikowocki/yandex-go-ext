package providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers/components"
	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type providerHealthStub struct{}

func (providerHealthStub) HealthCheck(context.Context) error { return nil }

func TestProviderConstructorsRejectNilConfig(t *testing.T) {
	ctx := context.Background()
	if _, err := NewDatabase(ctx, nil); err == nil {
		t.Error("NewDatabase accepted nil config")
	}
	if _, err := NewObjectStore(ctx, nil); err == nil {
		t.Error("NewObjectStore accepted nil config")
	}
	if _, err := NewBroker(ctx, nil); err == nil {
		t.Error("NewBroker accepted nil config")
	}
	if _, err := NewREST(nil, nil); err == nil {
		t.Error("NewREST accepted nil config")
	}
	if _, err := NewREST(&config.Config{}, nil); err == nil {
		t.Error("NewREST accepted nil avatar components")
	}
}

func TestDependencyHealthCheck(t *testing.T) {
	if err := dependencyHealthCheck(providerHealthStub{})(context.Background()); err != nil {
		t.Fatalf("supported health check failed: %v", err)
	}
	if err := dependencyHealthCheck(struct{}{})(context.Background()); err == nil {
		t.Fatal("unsupported health check accepted")
	}
}

func TestNewConfigLoadsExplicitFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "database:\n  dsn: postgres://user:pass@localhost/db\ns3:\n  endpoint: localhost:9000\n  access_key_id: key\n  secret_access_key: secret\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	cfg, err := NewConfig()
	if err != nil || cfg.Database.DSN == "" {
		t.Fatalf("NewConfig() = %+v, %v", cfg, err)
	}
}

func TestProviderFactoryBranches(t *testing.T) {
	if _, err := NewBroker(context.Background(), &config.Config{Broker: config.BrokerConfig{Type: "unknown"}}); err == nil {
		t.Fatal("unsupported broker accepted")
	}
	if _, err := NewObjectStore(context.Background(), &config.Config{}); err == nil {
		t.Fatal("empty object storage config accepted")
	}
	server, err := NewREST(&config.Config{Server: config.ServerConfig{RateLimitPerSecond: 1, RateLimitBurst: 1}}, &components.Avatar{DB: &pgxpool.Pool{}})
	if err != nil || server == nil {
		t.Fatalf("NewREST() = %v, %v", server, err)
	}
}
