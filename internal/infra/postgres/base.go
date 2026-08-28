package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

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

// uniqueViolation возвращает имя нарушенного unique constraint.
func uniqueViolation(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return pgErr.ConstraintName, true
	}
	return "", false
}
