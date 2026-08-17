package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Subscribe подписывает обработчик на тему RabbitMQ.
func (b *Broker) Subscribe(ctx context.Context, topic string, handler contracts.MessageHandler) error {
	if topic == "" || handler == nil {
		return fmt.Errorf("topic and handler are required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	if _, exists := b.subscriptions[topic]; exists {
		b.mu.Unlock()
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}
	b.mu.Unlock()

	b.consumerMu.Lock()
	queue, err := b.declareSubscriptionTopology(topic)
	var deliveries <-chan amqp.Delivery
	consumerTag := consumerTag(topic)
	if err == nil {
		err = b.consumerChannel.Qos(b.prefetch, 0, false)
	}
	if err == nil {
		deliveries, err = b.consumerChannel.Consume(queue, consumerTag, false, false, false, false, nil)
	}
	b.consumerMu.Unlock()
	if err != nil {
		return fmt.Errorf("subscribe to RabbitMQ topic %q: %w", topic, err)
	}

	subCtx, cancel := context.WithCancel(b.ctx)
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-subCtx.Done():
		}
	}()
	b.mu.Lock()
	b.subscriptions[topic] = struct{}{}
	b.mu.Unlock()
	b.consumerWG.Add(1)
	go func() {
		defer b.consumerWG.Done()
		defer cancel()
		b.consumeMessages(subCtx, topic, consumerTag, deliveries, handler)
	}()
	logging.Info(ctx, "subscribed to broker topic", logging.String("broker", "rabbitmq"), logging.String("topic", topic))
	return nil
}

func (b *Broker) consumeMessages(ctx context.Context, topic, consumerName string, deliveries <-chan amqp.Delivery, handler contracts.MessageHandler) {
	parallelism := b.prefetch
	if parallelism < 1 {
		parallelism = 1
	}
	semaphore := make(chan struct{}, parallelism)
	var handlers sync.WaitGroup
	defer handlers.Wait()

	for {
		select {
		case <-ctx.Done():
			logging.Info(ctx, "stopping broker consumer", logging.String("broker", "rabbitmq"), logging.String("topic", topic))
			return
		case delivery, ok := <-deliveries:
			if !ok {
				logging.Warn(ctx, "RabbitMQ delivery channel closed", logging.String("topic", topic))
				return
			}
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			handlers.Add(1)
			go func(delivery amqp.Delivery) {
				defer handlers.Done()
				defer func() { <-semaphore }()
				b.handleDelivery(ctx, topic, delivery, handler)
			}(delivery)
		}
	}
}
