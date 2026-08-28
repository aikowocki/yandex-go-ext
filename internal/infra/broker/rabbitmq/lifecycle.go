package rabbitmq

import (
	"context"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Close закрывает соединения брокера.
func (b *Broker) Close() error {
	var closeErr error
	b.closeOnce.Do(func() {
		if b.cancel != nil {
			b.cancel()
		}
		consumerDone := make(chan struct{})
		go func() {
			b.consumerWG.Wait()
			close(consumerDone)
		}()
		select {
		case <-consumerDone:
		case <-time.After(rabbitMQShutdownTimeout):
			logging.Warn(b.ctx, "rabbitmq consumer shutdown timed out")
		}
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
		if b.consumerChannel != nil {
			if err := b.consumerChannel.Close(); err != nil && err != amqp.ErrClosed {
				closeErr = err
			}
		}
		if b.consumerConn != nil {
			if err := b.consumerConn.Close(); err != nil && closeErr == nil && err != amqp.ErrClosed {
				closeErr = err
			}
		}
		if b.publisherChannel != nil {
			if err := b.publisherChannel.Close(); err != nil && closeErr == nil && err != amqp.ErrClosed {
				closeErr = err
			}
		}
		if b.publisherConn != nil {
			if err := b.publisherConn.Close(); err != nil && closeErr == nil && err != amqp.ErrClosed {
				closeErr = err
			}
		}
		logCtx := b.ctx
		if logCtx == nil {
			logCtx = context.Background()
		}
		logging.Info(logCtx, "rabbitmq broker closed")
	})
	return closeErr
}
