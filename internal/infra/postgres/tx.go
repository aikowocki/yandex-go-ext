package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// txKey хранит активную транзакцию в context.
type txKey struct{}

// TxManager объединяет несколько операций repositories в одну транзакцию БД.
type TxManager struct {
	db *DB
}

// NewTxManager создаёт TxManager поверх DB.
func NewTxManager(db *DB) *TxManager {
	return &TxManager{db: db}
}

// Do выполняет fn внутри одной транзакции и повторно использует уже активную
// транзакцию при вложенном вызове.
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if fn == nil {
		return fmt.Errorf("run transaction: callback is nil")
	}
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}
	if m == nil || m.db == nil {
		return fmt.Errorf("run transaction: database is not configured")
	}

	return withRetry(ctx, isTxRetryable, func() (err error) {
		tx, err := m.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if err := fn(withTx(ctx, tx)); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
		return nil
	})
}

func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
