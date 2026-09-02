package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// Publish отправляет сообщение в Kafka.
func (b *Broker) Publish(ctx context.Context, topic string, msg *contracts.Message) (err error) {
	if msg == nil || topic == "" {
		return fmt.Errorf("topic and message are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	ctx, span := observability.StartProducerSpan(ctx, "kafka", topic)
	started := time.Now()
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		duration := time.Since(started)
		observability.RecordMessagingPublish(ctx, "kafka", topic, status, duration)
		logging.LogMessagingPublish(ctx, "kafka", topic, msg.ID, status, duration, err)
		observability.FinishMessagingSpan(span, err)
	}()
	observability.InjectMessageContext(ctx, msg.Headers)

	headers := make([]sarama.RecordHeader, 0, len(msg.Headers)+2)
	headers = append(headers,
		sarama.RecordHeader{Key: []byte("message_id"), Value: []byte(msg.ID)},
		sarama.RecordHeader{Key: []byte("timestamp"), Value: []byte(msg.Timestamp.UTC().Format(time.RFC3339Nano))},
	)
	for key, value := range msg.Headers {
		if key == "message_id" || key == "timestamp" || key == "headers" {
			continue
		}
		headers = append(headers, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
	}

	kafkaMessage := &sarama.ProducerMessage{
		Topic:   topic,
		Key:     sarama.StringEncoder(msg.ID),
		Value:   sarama.ByteEncoder(msg.Payload),
		Headers: headers,
	}
	if _, _, err := b.producer.SendMessage(kafkaMessage); err != nil {
		return fmt.Errorf("send kafka message: %w", err)
	}
	return nil
}
