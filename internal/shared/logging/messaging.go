package logging

import (
	"context"
	"time"
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
		String("messaging.system", safeMessagingValue(system)),
		String("messaging.destination.name", safeMessagingValue(destination)),
		String("outcome", safeMessagingValue(outcome)),
		Duration("duration", duration),
	}
	if messageID != "" {
		attrs = append(attrs, String("message_id", messageID))
	}
	return attrs
}

func safeMessagingValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
