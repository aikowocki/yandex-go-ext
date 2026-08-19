package postgres

import (
	"context"
	"time"
)

const (
	retryMaxAttempts = 3
	retryBaseDelay   = 20 * time.Millisecond
)

func withRetry(ctx context.Context, classify func(error) bool, fn func() error) error {
	var err error
	delay := retryBaseDelay
	for attempt := range retryMaxAttempts {
		if err = fn(); err == nil || !classify(err) {
			return err
		}
		if attempt == retryMaxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			delay *= 2
		}
	}
	return err
}
