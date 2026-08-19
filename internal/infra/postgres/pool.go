package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
)

// DB оборачивает пул соединений и выбирает querier для текущего context.
type DB struct {
	*pgxpool.Pool
}

// NewPool создаёт и проверяет пул PostgreSQL.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// querier возвращает активную транзакцию из context или retrying pool querier.
func (db *DB) querier(ctx context.Context) gen.DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return retryingDB{pool: db.Pool}
}
