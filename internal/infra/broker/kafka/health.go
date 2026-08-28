package kafka

import (
	"context"
	"fmt"
)

// HealthCheck проверяет доступность Kafka.
func (b *Broker) HealthCheck(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil || b.healthClient == nil {
		return fmt.Errorf("health client for Kafka is not configured")
	}
	if err := b.healthClient.RefreshMetadata(); err != nil {
		return fmt.Errorf("refresh Kafka metadata: %w", err)
	}
	return nil
}
