package rabbitmq

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publish отправляет сообщение в RabbitMQ.
func (b *Broker) Publish(ctx context.Context, topic string, msg *contracts.Message) (err error) {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}
	if topic == "" {
		topic = msg.Topic
	}
	if topic == "" {
		return fmt.Errorf("message topic is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	ctx, span := observability.StartProducerSpan(ctx, "rabbitmq", topic)
	defer func() { observability.FinishMessagingSpan(span, err) }()
	observability.InjectMessageContext(ctx, msg.Headers)

	headers := cloneHeaders(msg.Headers)
	if msg.ID != "" {
		headers["message_id"] = msg.ID
	}
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         msg.Payload,
		Headers:      headers,
		Timestamp:    msg.Timestamp,
		DeliveryMode: amqp.Persistent,
		MessageId:    msg.ID,
	}
	if err := b.publishConfirmed(ctx, topic, msg.ID, publishing); err != nil {
		return fmt.Errorf("publish rabbitmq message: %w", err)
	}
	logging.Debug(ctx, "published broker message", logging.String("broker", "rabbitmq"), logging.String("topic", topic), logging.String("message_id", msg.ID))
	return nil
}

func (b *Broker) publishConfirmed(ctx context.Context, routingKey, messageID string, publishing amqp.Publishing) error {
	b.publishMu.Lock()
	defer b.publishMu.Unlock()

	b.mu.RLock()
	closed := b.closed
	channel := b.publisherChannel
	confirms := b.publisherConfirms
	returns := b.publisherReturns
	exchange := b.exchange
	b.mu.RUnlock()
	if closed || channel == nil {
		return fmt.Errorf("rabbitmq publisher is closed")
	}
	if err := channel.PublishWithContext(ctx, exchange, routingKey, true, false, publishing); err != nil {
		return err
	}

	var returnErr error
	for {
		select {
		case returned, ok := <-returns:
			if !ok {
				return fmt.Errorf("rabbitmq return channel closed")
			}
			if returned.MessageId == messageID {
				returnErr = fmt.Errorf("message was unroutable: %d %s", returned.ReplyCode, returned.ReplyText)
			}
		case confirmation, ok := <-confirms:
			if !ok {
				return fmt.Errorf("rabbitmq confirmation channel closed")
			}
			if !confirmation.Ack {
				return fmt.Errorf("rabbitmq publisher NACK for message %q", messageID)
			}
			return returnErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *Broker) publishConfirmedTo(ctx context.Context, exchange, routingKey, messageID string, publishing amqp.Publishing) error {
	b.publishMu.Lock()
	defer b.publishMu.Unlock()
	b.mu.RLock()
	closed := b.closed
	channel := b.publisherChannel
	confirms := b.publisherConfirms
	returns := b.publisherReturns
	b.mu.RUnlock()
	if closed || channel == nil {
		return fmt.Errorf("rabbitmq publisher is closed")
	}
	if err := channel.PublishWithContext(ctx, exchange, routingKey, true, false, publishing); err != nil {
		return err
	}
	var returnErr error
	for {
		select {
		case returned, ok := <-returns:
			if !ok {
				return fmt.Errorf("rabbitmq return channel closed")
			}
			if returned.MessageId == messageID {
				returnErr = fmt.Errorf("message was unroutable: %d %s", returned.ReplyCode, returned.ReplyText)
			}
		case confirmation, ok := <-confirms:
			if !ok {
				return fmt.Errorf("rabbitmq confirmation channel closed")
			}
			if !confirmation.Ack {
				return fmt.Errorf("rabbitmq publisher NACK for message %q", messageID)
			}
			return returnErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func cloneHeaders(headers map[string]string) amqp.Table {
	result := make(amqp.Table, len(headers)+1)
	for key, value := range headers {
		result[key] = value
	}
	return result
}
