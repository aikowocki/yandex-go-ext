package integration_test

import (
	"context"
	"errors"
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

func TestTxManagerCommitRollbackAndNestedReuse(t *testing.T) {
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

	schema, err := os.ReadFile(filepath.Join("..", "..", "internal", "infra", "postgres", "migrations", "000001_init.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}

	db := &pginfra.DB{Pool: pool}
	repo := pginfra.NewAvatarRepository(db)
	txManager := pginfra.NewTxManager(db)
	newAvatar := func() *domain.Avatar {
		return &domain.Avatar{
			ID:               uuid.New(),
			UserID:           "tx-user",
			FileName:         "avatar.jpg",
			MimeType:         "image/jpeg",
			SizeBytes:        128,
			Width:            10,
			Height:           10,
			S3KeyOriginal:    "avatars/originals/tx/avatar.jpg",
			UploadStatus:     domain.UploadStatusCompleted,
			ProcessingStatus: domain.ProcessingStatusPending,
		}
	}

	committed := newAvatar()
	if err := txManager.Do(ctx, func(ctx context.Context) error { return repo.Create(ctx, committed) }); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}
	if _, err := repo.GetByID(ctx, committed.ID); err != nil {
		t.Fatalf("committed avatar not found: %v", err)
	}

	rolledBack := newAvatar()
	boom := errors.New("rollback")
	err = txManager.Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, rolledBack); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("rollback error = %v, want %v", err, boom)
	}
	if _, err := repo.GetByID(ctx, rolledBack.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("rolled back avatar lookup error = %v, want ErrNotFound", err)
	}

	nested := newAvatar()
	if err := txManager.Do(ctx, func(ctx context.Context) error {
		return txManager.Do(ctx, func(ctx context.Context) error {
			return repo.Create(ctx, nested)
		})
	}); err != nil {
		t.Fatalf("nested transaction: %v", err)
	}
	if _, err := repo.GetByID(ctx, nested.ID); err != nil {
		t.Fatalf("nested committed avatar not found: %v", err)
	}
}
