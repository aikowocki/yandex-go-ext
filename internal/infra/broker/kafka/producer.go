package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// Publish отправляет сообщение в Kafka.
func (b *Broker) Publish(ctx context.Context, topic string, msg *contracts.Message) error {
	if msg == nil || topic == "" {
		return fmt.Errorf("topic and message are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

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
	partition, offset, err := b.producer.SendMessage(kafkaMessage)
	if err != nil {
		return fmt.Errorf("send kafka message: %w", err)
	}
	logging.Debug(ctx, "published broker message", logging.String("broker", "kafka"), logging.String("topic", topic), logging.String("message_id", msg.ID), logging.Int32("partition", partition), logging.Int64("offset", offset))
	return nil
}
