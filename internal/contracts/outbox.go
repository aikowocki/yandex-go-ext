package contracts

import (
	"context"
	"time"
)

// OutboxEvent содержит событие, ожидающее публикации.
type OutboxEvent struct {
	ID            string
	Topic         string
	Payload       []byte
	Headers       map[string]string
	CreatedAt     time.Time
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
}

// OutboxRepository сохраняет и публикует события через transactional outbox.
type OutboxRepository interface {
	Save(ctx context.Context, event *OutboxEvent) error
	ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]*OutboxEvent, error)
	ListPending(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, reason string, nextAttemptAt time.Time) error
}
