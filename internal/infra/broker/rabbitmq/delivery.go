package rabbitmq

import (
	"context"
	"strconv"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

func (b *Broker) handleDelivery(ctx context.Context, topic string, delivery amqp.Delivery, handler contracts.MessageHandler) {
	message := deliveryMessage(topic, delivery, b.maxAttempts)
	messageCtx := observability.ExtractMessageContext(ctx, message.Headers)
	messageCtx, span := observability.StartConsumerSpan(messageCtx, "rabbitmq", topic, deliveryAttempt(message))
	observability.ChangeQueueDepth("rabbitmq", topic, 1)
	started := time.Now()
	err := handler(messageCtx, message)
	observability.ChangeQueueDepth("rabbitmq", topic, -1)
	status := "success"
	if err != nil {
		status = "error"
	}
	duration := time.Since(started)
	observability.RecordMessagingConsume(messageCtx, "rabbitmq", topic, status, duration)
	logging.LogMessagingConsume(messageCtx, "rabbitmq", topic, message.ID, status, duration, err)
	observability.FinishMessagingSpan(span, err)
	if err == nil {
		if ackErr := delivery.Ack(false); ackErr != nil {
			logging.Error(messageCtx, "failed to ack RabbitMQ message", logging.Err(ackErr), logging.String("message_id", message.ID))
		}
		return
	}

	logging.Error(messageCtx, "failed to handle RabbitMQ message", logging.Err(err), logging.String("topic", topic), logging.String("message_id", message.ID), logging.String("attempt", message.Headers[contracts.DeliveryAttemptHeader]))
	attempt := deliveryAttempt(message)
	targetExchange := b.retryExchange
	if attempt >= b.maxAttempts {
		observability.RecordMessagingRetry(messageCtx, "rabbitmq", topic, true)
		targetExchange = b.deadExchange
	} else {
		observability.RecordMessagingRetry(messageCtx, "rabbitmq", topic, false)
	}
	if publishErr := b.publishForward(messageCtx, topic, message, targetExchange); publishErr != nil {
		logging.Error(messageCtx, "failed to forward RabbitMQ message", logging.Err(publishErr), logging.String("message_id", message.ID))
		if nackErr := delivery.Nack(false, true); nackErr != nil {
			logging.Error(messageCtx, "failed to requeue RabbitMQ message", logging.Err(nackErr), logging.String("message_id", message.ID))
		}
		return
	}
	if ackErr := delivery.Ack(false); ackErr != nil {
		logging.Error(messageCtx, "failed to ack forwarded RabbitMQ message", logging.Err(ackErr), logging.String("message_id", message.ID))
	}
}

func (b *Broker) publishForward(ctx context.Context, topic string, message *contracts.Message, targetExchange string) (err error) {
	headers := observability.CloneHeaders(message.Headers)
	attempt := deliveryAttempt(message)
	if targetExchange == b.retryExchange {
		headers[contracts.DeliveryAttemptHeader] = strconv.Itoa(attempt + 1)
	} else {
		headers[contracts.DeliveryAttemptHeader] = strconv.Itoa(attempt)
	}
	headers[contracts.MaxDeliveryAttemptsHeader] = strconv.Itoa(b.maxAttempts)
	ctx, span := observability.StartProducerSpan(ctx, "rabbitmq", topic)
	defer func() { observability.FinishMessagingSpan(span, err) }()
	observability.InjectMessageContext(ctx, headers)
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         message.Payload,
		Headers:      cloneHeaders(headers),
		Timestamp:    message.Timestamp,
		DeliveryMode: amqp.Persistent,
		MessageId:    message.ID,
	}
	return b.publishConfirmedTo(ctx, targetExchange, topic, message.ID, publishing)
}
