package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	pginfra "github.com/aikowocki/yandex-go-ext/internal/infra/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPostgresRepositories(t *testing.T) {
	if testing.Short() || os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("gophprofile"), tcpostgres.WithUsername("gophprofile"), tcpostgres.WithPassword("gophprofile"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	schemaPath := filepath.Join("..", "..", "internal", "infra", "postgres", "migrations", "000001_init.up.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}

	db := &pginfra.DB{Pool: pool}
	repo := pginfra.NewAvatarRepository(db)
	avatar := &domain.Avatar{
		ID:               uuid.New(),
		UserID:           "integration-user",
		FileName:         "avatar.jpg",
		MimeType:         "image/jpeg",
		SizeBytes:        128,
		Width:            10,
		Height:           10,
		S3KeyOriginal:    "avatars/originals/integration/avatar.jpg",
		UploadStatus:     domain.UploadStatusCompleted,
		ProcessingStatus: domain.ProcessingStatusPending,
	}
	if err := repo.Create(ctx, avatar); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.GetByID(ctx, avatar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.UserID != avatar.UserID || loaded.S3KeyOriginal != avatar.S3KeyOriginal {
		t.Fatalf("loaded avatar differs: %#v", loaded)
	}
}
