package rabbitmq

import (
	"context"
	"fmt"
)

// HealthCheck проверяет доступность RabbitMQ.
func (b *Broker) HealthCheck(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return fmt.Errorf("broker RabbitMQ is nil")
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed || b.publisherConn == nil || b.consumerConn == nil || b.publisherConn.IsClosed() || b.consumerConn.IsClosed() || b.publisherChannel == nil || b.consumerChannel == nil || b.publisherChannel.IsClosed() || b.consumerChannel.IsClosed() {
		return fmt.Errorf("connection RabbitMQ or channel is closed")
	}
	return nil
}
