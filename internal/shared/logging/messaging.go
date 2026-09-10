package logging

import (
	"context"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/messaging"
)

// LogMessagingPublish записывает результат публикации сообщения.
func LogMessagingPublish(ctx context.Context, system, destination, messageID, outcome string, duration time.Duration, err error) {
	attrs := messagingAttrs(system, destination, messageID, outcome, duration)
	if err != nil {
		attrs = append(attrs, Err(err))
	}
	Info(ctx, "messaging publish finished", attrArgs(attrs)...)
}

// LogMessagingConsume записывает результат обработки сообщения.
func LogMessagingConsume(ctx context.Context, system, destination, messageID, outcome string, duration time.Duration, err error) {
	attrs := messagingAttrs(system, destination, messageID, outcome, duration)
	if err != nil {
		attrs = append(attrs, Err(err))
	}
	Info(ctx, "messaging consume finished", attrArgs(attrs)...)
}

func messagingAttrs(system, destination, messageID, outcome string, duration time.Duration) []Attr {
	attrs := []Attr{
		String("messaging.system", messaging.SafeValue(system)),
		String("messaging.destination.name", messaging.SafeValue(destination)),
		String("outcome", messaging.SafeValue(outcome)),
		Duration("duration", duration),
	}
	if messageID != "" {
		attrs = append(attrs, String("message_id", messageID))
	}
	return attrs
}
