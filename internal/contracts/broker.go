package contracts

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// MessageBroker объединяет публикацию, подписку и закрытие брокера.
type MessageBroker interface {
	Publisher
	Consumer
	io.Closer
}

// Publisher публикует сообщения в брокер.
type Publisher interface {
	Publish(ctx context.Context, topic string, msg *Message) error
}

// Consumer подписывает обработчик на сообщения топика.
type Consumer interface {
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
}

// MessageHandler обрабатывает одно сообщение брокера.
type MessageHandler func(ctx context.Context, msg *Message) error

const (
	// DeliveryAttemptHeader и MaxDeliveryAttemptsHeader — метаданные доставки,
	// независимые от конкретного брокера. Брокеры могут заполнять их для
	// обработчиков с поддержкой повторных попыток, не раскрывая в этом пакете
	// API, специфичные для конкретного брокера.
	DeliveryAttemptHeader = "x-delivery-attempt"
	// MaxDeliveryAttemptsHeader хранит максимальное число попыток доставки.
	MaxDeliveryAttemptsHeader = "x-max-delivery-attempts"

	// ContentTypeHeader и SchemaVersionHeader описывают контракт сообщения
	// независимо от реализации конкретного брокера.
	ContentTypeHeader = "content-type"
	// SchemaVersionHeader хранит версию схемы сообщения.
	SchemaVersionHeader = "schema-version"
)

// Message содержит данные сообщения брокера.
type Message struct {
	ID        string
	Topic     string
	Payload   []byte
	Headers   map[string]string
	Timestamp time.Time
}

// NewMessage создаёт сообщение с новым идентификатором.
func NewMessage(topic string, payload []byte) *Message {
	return NewMessageWithID(uuid.NewString(), topic, payload)
}

// NewMessageWithID создаёт сообщение с указанным идентификатором.
func NewMessageWithID(id, topic string, payload []byte) *Message {
	return &Message{
		ID:      id,
		Topic:   topic,
		Payload: payload,
		Headers: map[string]string{
			ContentTypeHeader:   "application/json",
			SchemaVersionHeader: "v1",
		},
		Timestamp: time.Now().UTC(),
	}
}
