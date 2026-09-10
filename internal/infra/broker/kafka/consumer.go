package kafka

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

func (b *Broker) consumeErrors() {
	defer b.wg.Done()
	errorsCh := b.consumerGroup.Errors()
	for {
		select {
		case <-b.ctx.Done():
			return
		case err, ok := <-errorsCh:
			if !ok {
				return
			}
			if err != nil {
				logging.Error(b.ctx, "Kafka consumer error", logging.Err(err), logging.String("group_id", b.groupID))
			}
		}
	}
}

// Subscribe подписывает обработчик на тему Kafka.
func (b *Broker) Subscribe(ctx context.Context, topic string, handler contracts.MessageHandler) error {
	ctx = logging.WithLogger(ctx, b.logger)
	if topic == "" || handler == nil {
		return fmt.Errorf("topic and handler are required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.handlers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}
	b.handlers[topic] = handler
	if !b.started {
		b.started = true
		b.wg.Add(1)
		go b.consumeLoop()
	} else if b.consumeCancel != nil {
		b.consumeCancel()
	}
	logging.Info(ctx, "subscribed to broker topic", logging.String("broker", "kafka"), logging.String("topic", topic))
	return nil
}

func (b *Broker) consumeLoop() {
	defer b.wg.Done()
	for {
		if b.ctx.Err() != nil {
			return
		}
		topics := b.topicList()
		if len(topics) == 0 {
			return
		}
		sessionCtx, cancel := context.WithCancel(b.ctx)
		b.mu.Lock()
		b.consumeCancel = cancel
		b.mu.Unlock()
		err := b.consumerGroup.Consume(sessionCtx, topics, &consumerGroupHandler{broker: b})
		cancel()
		b.mu.Lock()
		b.consumeCancel = nil
		b.mu.Unlock()
		if b.ctx.Err() != nil {
			return
		}
		if err != nil {
			logging.Error(b.ctx, "Kafka consume error", logging.Err(err))
			select {
			case <-b.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func (b *Broker) topicList() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	topics := make([]string, 0, len(b.handlers))
	for topic := range b.handlers {
		topics = append(topics, topic)
	}
	return topics
}

func (b *Broker) handlerFor(topic string) contracts.MessageHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.handlers[topic]
}

type consumerGroupHandler struct {
	broker *Broker
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			contractMessage := parseMessage(message)
			handler := h.broker.handlerFor(message.Topic)
			if handler == nil {
				logging.Warn(session.Context(), "Kafka message has no handler", logging.String("topic", message.Topic))
				session.MarkMessage(message, "")
				continue
			}
			messageCtx := observability.ExtractMessageContext(session.Context(), contractMessage.Headers)
			messageCtx, span := observability.StartConsumerSpan(messageCtx, "kafka", message.Topic, deliveryAttempt(contractMessage))
			observability.ChangeQueueDepth("kafka", message.Topic, 1)
			started := time.Now()
			err := contracts.HandleWithRetryObserved(messageCtx, contractMessage, handler, h.broker.maxAttempts, func(int) {
				observability.RecordMessagingRetry(messageCtx, "kafka", message.Topic, false)
			})
			observability.ChangeQueueDepth("kafka", message.Topic, -1)
			status := "success"
			if err != nil {
				status = "error"
			}
			duration := time.Since(started)
			observability.RecordMessagingConsume(messageCtx, "kafka", message.Topic, status, duration)
			logging.LogMessagingConsume(messageCtx, "kafka", message.Topic, contractMessage.ID, status, duration, err)
			observability.FinishMessagingSpan(span, err)
			if err != nil {
				observability.RecordMessagingRetry(messageCtx, "kafka", message.Topic, true)
				if dlqErr := h.broker.publishToDLQ(messageCtx, message.Topic, contractMessage, err); dlqErr != nil {
					return fmt.Errorf("publish Kafka dlq for %s: %w", message.Topic, dlqErr)
				}
				logging.Error(messageCtx, "failed to handle Kafka message; sent to dlq", logging.Err(err), logging.String("topic", message.Topic), logging.String("message_id", contractMessage.ID))
				session.MarkMessage(message, "")
				continue
			}
			session.MarkMessage(message, "")
		}
	}
}

func deliveryAttempt(message *contracts.Message) int {
	if message == nil {
		return 1
	}
	attempt, err := strconv.Atoi(message.Headers[contracts.DeliveryAttemptHeader])
	if err != nil || attempt < 1 {
		return 1
	}
	return attempt
}
