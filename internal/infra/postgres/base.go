package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
)

// baseRepo инкапсулирует общий доступ repositories к DB и sqlc querier.
type baseRepo struct {
	db *DB
}

// q возвращает sqlc querier для активной транзакции из context или пула.
func (r baseRepo) q(ctx context.Context) gen.Querier {
	return gen.New(r.db.querier(ctx))
}

// parseUUIDOr переводит невалидный UUID в переданную доменную ошибку.
func parseUUIDOr(id string, notFound error) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, notFound
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// wrapNotFound переводит pgx.ErrNoRows в доменную sentinel-ошибку.
func wrapNotFound(err error, notFound error, operation string) error {
	if isNoRows(err) {
		return notFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

// uniqueViolation возвращает имя нарушенного unique constraint.
func uniqueViolation(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return pgErr.ConstraintName, true
	}
	return "", false
}
