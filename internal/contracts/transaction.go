package contracts

import "context"

// TxManager выполняет fn как одну атомарную единицу работы в хранилище.
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
