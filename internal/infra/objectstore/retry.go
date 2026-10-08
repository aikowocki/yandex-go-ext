package objectstore

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/minio/minio-go/v7"
)

const (
	retryMaxAttempts = 3
	retryBaseDelay   = 50 * time.Millisecond
)

// isTransientError возвращает true для временных сетевых ошибок,
// при которых повтор запроса имеет смысл. Ошибки клиента (403, 404 и т.п.)
// транзиентными не считаются и не инкрементируют счётчик circuit breaker-а.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	resp := minio.ToErrorResponse(err)
	switch resp.Code {
	case "RequestTimeout", "ServiceUnavailable", "SlowDown", "InternalError":
		return true
	}
	return false
}

// withRetry выполняет fn до retryMaxAttempts раз с экспоненциальным backoff,
// повторяя только при транзиентных ошибках.
func withRetry(ctx context.Context, fn func() error) error {
	var err error
	delay := retryBaseDelay
	for attempt := range retryMaxAttempts {
		if err = fn(); err == nil || !isTransientError(err) {
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
