package postgres

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isConnRetryable(err error) bool {
	return pgconn.SafeToRetry(err)
}

func isTxRetryable(err error) bool {
	if isConnRetryable(err) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgerrcode.SerializationFailure || pgErr.Code == pgerrcode.DeadlockDetected
	}
	return false
}
