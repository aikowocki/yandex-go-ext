package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
)

type retryingDB struct {
	pool *pgxpool.Pool
}

var _ gen.DBTX = retryingDB{}

func (d retryingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	err := withRetry(ctx, isConnRetryable, func() error {
		var err error
		tag, err = d.pool.Exec(ctx, sql, args...)
		return err
	})
	return tag, err
}

func (d retryingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	var rows pgx.Rows
	err := withRetry(ctx, isConnRetryable, func() error {
		var err error
		rows, err = d.pool.Query(ctx, sql, args...)
		return err
	})
	return rows, err
}

func (d retryingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	rows, err := d.Query(ctx, sql, args...)
	return queryRow{rows: rows, err: err}
}

type queryRow struct {
	rows pgx.Rows
	err  error
}

func (r queryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	defer r.rows.Close()
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return err
		}
		return pgx.ErrNoRows
	}
	if err := r.rows.Scan(dest...); err != nil {
		return err
	}
	return r.rows.Err()
}
