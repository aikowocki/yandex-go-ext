package kafka

import (
	"context"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
)

func (b *Broker) publishToDLQ(ctx context.Context, topic string, message *contracts.Message, cause error) error {
	dlqHeaders := make(map[string]string, len(message.Headers)+1)
	for key, value := range message.Headers {
		dlqHeaders[key] = value
	}
	dlqHeaders["x-error"] = cause.Error()
	dlqMessage := &contracts.Message{
		ID:        message.ID,
		Topic:     topic + b.dlqSuffix,
		Payload:   message.Payload,
		Headers:   dlqHeaders,
		Timestamp: message.Timestamp,
	}
	return b.Publish(ctx, dlqMessage.Topic, dlqMessage)
}
