package contracts

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// HandleWithRetry повторяет обработку сообщения и обновляет retry-заголовки.
func HandleWithRetry(ctx context.Context, msg *Message, handler MessageHandler, maxAttempts int) error {
	return HandleWithRetryObserved(ctx, msg, handler, maxAttempts, nil)
}

// HandleWithRetryObserved повторяет обработку сообщения и вызывает onRetry
// при планировании следующей попытки после ошибки.
func HandleWithRetryObserved(ctx context.Context, msg *Message, handler MessageHandler, maxAttempts int, onRetry func(attempt int)) error {
	if handler == nil {
		return fmt.Errorf("message handler is nil")
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if msg.Headers == nil {
			msg.Headers = make(map[string]string, 2)
		}
		msg.Headers[DeliveryAttemptHeader] = strconv.Itoa(attempt)
		msg.Headers[MaxDeliveryAttemptsHeader] = strconv.Itoa(maxAttempts)
		if err := handler(ctx, msg); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == maxAttempts {
			break
		}
		if onRetry != nil {
			onRetry(attempt + 1)
		}

		delay := min(time.Second*time.Duration(1<<(attempt-1)), 30*time.Second)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
