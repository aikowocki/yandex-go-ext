package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfigYAML() string {
	return `
server:
  host: 127.0.0.1
  port: 8080
database:
  dsn: postgres://user:pass@localhost/db
s3:
  endpoint: localhost:9000
  access_key_id: key
  secret_access_key: secret
  bucket: avatars
broker:
  type: rabbitmq
  rabbitmq:
    url: amqp://guest:guest@localhost:5672/
    exchange: avatars
worker:
  concurrency: 1
log:
  backend: slog
  level: info
  format: json
`
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, wasSet := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestLoadUsesExplicitFileAndEnvironmentOverrides(t *testing.T) {
	for _, key := range []string{
		"CONFIG_FILE", "ENV", "SERVER_PORT", "DATABASE_URL", "GOPHPROFILE_SERVER_PORT", "GOPHPROFILE_DATABASE_DSN",
	} {
		unsetEnv(t, key)
	}
	path := writeConfigFile(t, validConfigYAML())
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("GOPHPROFILE_SERVER_PORT", "9090")
	t.Setenv("GOPHPROFILE_DATABASE_DSN", "postgres://alias/db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 || cfg.Database.DSN != "postgres://alias/db" {
		t.Fatalf("environment aliases were not applied: %+v", cfg)
	}
	if _, exists := os.LookupEnv("SERVER_PORT"); exists {
		t.Fatal("temporary canonical SERVER_PORT leaked")
	}
	if _, exists := os.LookupEnv("DATABASE_URL"); exists {
		t.Fatal("temporary canonical DATABASE_URL leaked")
	}
}

func TestLoadMissingFileCanUseEnvironment(t *testing.T) {
	unsetEnv(t, "ENV")
	unsetEnv(t, "CONFIG_FILE")
	for _, key := range []string{"DATABASE_URL", "S3_ENDPOINT", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
		unsetEnv(t, key)
	}
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("DATABASE_URL", "postgres://env/db")
	t.Setenv("S3_ENDPOINT", "localhost:9000")
	t.Setenv("S3_ACCESS_KEY_ID", "key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with missing optional file error = %v", err)
	}
	if cfg.Database.DSN != "postgres://env/db" || cfg.S3.Endpoint != "localhost:9000" {
		t.Fatalf("environment config not loaded: %+v", cfg)
	}
}

func TestLoadReportsConfigAndEnvironmentErrors(t *testing.T) {
	t.Run("invalid yaml", func(t *testing.T) {
		unsetEnv(t, "CONFIG_FILE")
		path := writeConfigFile(t, "server: [")
		t.Setenv("CONFIG_FILE", path)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("invalid environment", func(t *testing.T) {
		unsetEnv(t, "CONFIG_FILE")
		path := writeConfigFile(t, validConfigYAML())
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("SERVER_PORT", "not-a-port")
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("component validation is deferred", func(t *testing.T) {
		unsetEnv(t, "CONFIG_FILE")
		path := writeConfigFile(t, strings.Replace(validConfigYAML(), "dsn: postgres://user:pass@localhost/db", "dsn: ''", 1))
		t.Setenv("CONFIG_FILE", path)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Database.DSN != "" {
			t.Fatalf("Load() unexpectedly validated database DSN: %q", cfg.Database.DSN)
		}
	})
}

func TestSetTemporaryEnvRestoresSetAndUnsetValues(t *testing.T) {
	t.Setenv("TEMP_CONFIG_TEST", "old")
	restore := setTemporaryEnv("TEMP_CONFIG_TEST", "new")
	if os.Getenv("TEMP_CONFIG_TEST") != "new" {
		t.Fatal("temporary value was not set")
	}
	restore()
	if os.Getenv("TEMP_CONFIG_TEST") != "old" {
		t.Fatal("previous value was not restored")
	}

	unsetEnv(t, "TEMP_CONFIG_TEST_UNSET")
	restore = setTemporaryEnv("TEMP_CONFIG_TEST_UNSET", "new")
	restore()
	if _, exists := os.LookupEnv("TEMP_CONFIG_TEST_UNSET"); exists {
		t.Fatal("previously unset value was not unset")
	}
}
