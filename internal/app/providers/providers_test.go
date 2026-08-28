package providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers/components"
	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type providerHealthStub struct{}

func (providerHealthStub) HealthCheck(context.Context) error { return nil }

func TestProviderConstructorsRejectNilConfig(t *testing.T) {
	ctx := context.Background()
	if _, err := NewDatabase(ctx, nil); err == nil {
		t.Error("NewDatabase accepted nil config")
	}
	if _, err := NewObjectStore(ctx, nil, nil); err == nil {
		t.Error("NewObjectStore accepted nil config")
	}
	if _, err := NewBroker(ctx, nil, nil); err == nil {
		t.Error("NewBroker accepted nil config")
	}
	if _, err := NewREST(nil, nil, nil); err == nil {
		t.Error("NewREST accepted nil config")
	}
	if _, err := NewREST(&config.Config{}, nil, nil); err == nil {
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
	if _, err := NewBroker(context.Background(), &config.Config{Broker: config.BrokerConfig{Type: "unknown"}}, nil); err == nil {
		t.Fatal("unsupported broker accepted")
	}
	if _, err := NewObjectStore(context.Background(), &config.Config{}, nil); err == nil {
		t.Fatal("empty object storage config accepted")
	}
	server, err := NewREST(&config.Config{Server: config.ServerConfig{RateLimitPerSecond: 1, RateLimitBurst: 1}}, &components.Avatar{DB: &postgres.DB{Pool: &pgxpool.Pool{}}}, nil)
	if err != nil || server == nil {
		t.Fatalf("NewREST() = %v, %v", server, err)
	}
}

func TestNewBrokerRejectsInvalidWorkerConcurrency(t *testing.T) {
	if _, err := NewBroker(context.Background(), &config.Config{Worker: config.WorkerConfig{Concurrency: 0}}, nil); err == nil {
		t.Fatal("NewBroker accepted invalid worker concurrency")
	}
}

func TestNewBrokerDelegatesToBrokerFactory(t *testing.T) {
	cfg := &config.Config{
		Worker: config.WorkerConfig{Concurrency: 1},
		Broker: config.BrokerConfig{Type: "unknown"},
	}
	if _, err := NewBroker(context.Background(), cfg, nil); err == nil {
		t.Fatal("NewBroker accepted unsupported broker type")
	}
}

func TestNewRESTRejectsInvalidServerConfig(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{RateLimitPerSecond: 0, RateLimitBurst: 1}}
	avatarComponents := &components.Avatar{DB: &postgres.DB{Pool: &pgxpool.Pool{}}}
	if _, err := NewREST(cfg, avatarComponents, nil); err == nil {
		t.Fatal("NewREST accepted invalid server config")
	}
}
