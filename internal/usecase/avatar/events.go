package avatar

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

func publishEvent(ctx context.Context, broker contracts.Publisher, outbox contracts.OutboxRepository, topic string, payload []byte) error {
	message := contracts.NewMessage(topic, payload)
	observability.InjectMessageContext(ctx, message.Headers)
	if outbox == nil {
		return broker.Publish(ctx, topic, message)
	}

	now := time.Now().UTC()
	event := &contracts.OutboxEvent{
		ID:            message.ID,
		Topic:         topic,
		Payload:       payload,
		Headers:       observability.CloneHeaders(message.Headers),
		CreatedAt:     now,
		NextAttemptAt: now,
	}
	if err := outbox.Save(ctx, event); err != nil {
		return fmt.Errorf("save outbox event: %w", err)
	}

	message = contracts.NewMessageWithID(event.ID, topic, payload)
	message.Headers = observability.CloneHeaders(event.Headers)
	if err := broker.Publish(ctx, topic, message); err != nil {
		nextAttempt := time.Now().UTC().Add(time.Second)
		if markErr := outbox.MarkFailed(ctx, event.ID, err.Error(), nextAttempt); markErr != nil {
			logging.Warn(ctx, "failed to update outbox retry state", logging.Err(markErr), logging.String("event_id", event.ID))
		}
		logging.Warn(ctx, "event stored in outbox after publish failure", logging.Err(err), logging.String("event_id", event.ID), logging.String("topic", topic))
		return nil
	}
	if err := outbox.MarkPublished(ctx, event.ID); err != nil {
		logging.Warn(ctx, "event published but outbox state was not updated", logging.Err(err), logging.String("event_id", event.ID))
	}
	return nil
}

func marshalEvent(event any) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	return payload, nil
}
