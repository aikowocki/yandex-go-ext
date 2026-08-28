package rabbitmq

import (
	"context"
	"strconv"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

func (b *Broker) handleDelivery(ctx context.Context, topic string, delivery amqp.Delivery, handler contracts.MessageHandler) {
	message := deliveryMessage(topic, delivery, b.maxAttempts)
	if err := handler(ctx, message); err == nil {
		if ackErr := delivery.Ack(false); ackErr != nil {
			logging.Error(ctx, "failed to ack RabbitMQ message", logging.Err(ackErr), logging.String("message_id", message.ID))
		}
		return
	} else {
		logging.Error(ctx, "failed to handle RabbitMQ message", logging.Err(err), logging.String("topic", topic), logging.String("message_id", message.ID), logging.String("attempt", message.Headers[contracts.DeliveryAttemptHeader]))
		attempt := deliveryAttempt(message)
		targetExchange := b.retryExchange
		if attempt >= b.maxAttempts {
			targetExchange = b.deadExchange
		}
		if publishErr := b.publishForward(ctx, topic, message, targetExchange); publishErr != nil {
			logging.Error(ctx, "failed to forward RabbitMQ message", logging.Err(publishErr), logging.String("message_id", message.ID))
			if nackErr := delivery.Nack(false, true); nackErr != nil {
				logging.Error(ctx, "failed to requeue RabbitMQ message", logging.Err(nackErr), logging.String("message_id", message.ID))
			}
			return
		}
		if ackErr := delivery.Ack(false); ackErr != nil {
			logging.Error(ctx, "failed to ack forwarded RabbitMQ message", logging.Err(ackErr), logging.String("message_id", message.ID))
		}
	}
}

func (b *Broker) publishForward(ctx context.Context, topic string, message *contracts.Message, targetExchange string) error {
	headers := cloneHeaders(message.Headers)
	attempt := deliveryAttempt(message)
	if targetExchange == b.retryExchange {
		headers[contracts.DeliveryAttemptHeader] = strconv.Itoa(attempt + 1)
	} else {
		headers[contracts.DeliveryAttemptHeader] = strconv.Itoa(attempt)
	}
	headers[contracts.MaxDeliveryAttemptsHeader] = strconv.Itoa(b.maxAttempts)
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         message.Payload,
		Headers:      headers,
		Timestamp:    message.Timestamp,
		DeliveryMode: amqp.Persistent,
		MessageId:    message.ID,
	}
	return b.publishConfirmedTo(ctx, targetExchange, topic, message.ID, publishing)
}
