package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxRepository хранит события outbox в PostgreSQL.
type OutboxRepository struct {
	pool    *pgxpool.Pool
	queries gen.Querier
}

// NewOutboxRepository создаёт репозиторий outbox.
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	repository := &OutboxRepository{pool: pool}
	if pool != nil {
		repository.queries = gen.New(pool)
	}
	return repository
}

// ClaimPending забирает ожидающие события outbox.
func (r *OutboxRepository) ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]*contracts.OutboxEvent, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("claim pending outbox events: database pool is not configured")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim pending outbox events: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := gen.New(tx).ClaimPendingOutboxEvents(ctx, gen.ClaimPendingOutboxEventsParams{
		Limit:   int32(limit),
		Column2: lease.Seconds(),
	})
	if err != nil {
		return nil, fmt.Errorf("claim pending outbox events: %w", err)
	}
	result := make([]*contracts.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, outboxEventFromValues(fromPGUUID(row.ID), row.Topic, row.Payload, row.Attempts, row.NextAttemptAt.Time, row.LastError, row.CreatedAt.Time))
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claimed outbox events: %w", err)
	}
	return result, nil
}

// Save сохраняет событие outbox.
func (r *OutboxRepository) Save(ctx context.Context, event *contracts.OutboxEvent) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("save outbox event: database pool is not configured")
	}
	if event == nil {
		return fmt.Errorf("save outbox event: event is nil")
	}
	id, err := uuid.Parse(event.ID)
	if err != nil {
		return fmt.Errorf("save outbox event: invalid id: %w", err)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.NextAttemptAt.IsZero() {
		event.NextAttemptAt = event.CreatedAt
	}
	err = r.queries.SaveOutboxEvent(ctx, gen.SaveOutboxEventParams{
		ID:            toPGUUID(id),
		Topic:         event.Topic,
		Payload:       event.Payload,
		Attempts:      int32(event.Attempts),
		NextAttemptAt: toPGTime(&event.NextAttemptAt),
		LastError:     event.LastError,
		CreatedAt:     toPGTime(&event.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("save outbox event: %w", err)
	}
	return nil
}

// ListPending возвращает ожидающие события outbox.
func (r *OutboxRepository) ListPending(ctx context.Context, limit int) ([]*contracts.OutboxEvent, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("list pending outbox events: database pool is not configured")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.queries.ListPendingOutboxEvents(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list pending outbox events: %w", err)
	}
	result := make([]*contracts.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, outboxEventFromValues(fromPGUUID(row.ID), row.Topic, row.Payload, row.Attempts, row.NextAttemptAt.Time, row.LastError, row.CreatedAt.Time))
	}
	return result, nil
}

// MarkPublished отмечает событие опубликованным.
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("mark outbox event published: database pool is not configured")
	}
	eventID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("mark outbox event published: invalid id: %w", err)
	}
	command, err := r.queries.MarkOutboxEventPublished(ctx, toPGUUID(eventID))
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// MarkFailed сохраняет ошибку публикации события.
func (r *OutboxRepository) MarkFailed(ctx context.Context, id string, reason string, nextAttemptAt time.Time) error {
	if r == nil || r.queries == nil {
		return fmt.Errorf("mark outbox event failed: database pool is not configured")
	}
	eventID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: invalid id: %w", err)
	}
	command, err := r.queries.MarkOutboxEventFailed(ctx, gen.MarkOutboxEventFailedParams{
		ID:            toPGUUID(eventID),
		NextAttemptAt: toPGTime(&nextAttemptAt),
		LastError:     reason,
	})
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func outboxEventFromValues(id uuid.UUID, topic string, payload []byte, attempts int32, nextAttemptAt time.Time, lastError string, createdAt time.Time) *contracts.OutboxEvent {
	return &contracts.OutboxEvent{
		ID:            id.String(),
		Topic:         topic,
		Payload:       payload,
		Attempts:      int(attempts),
		NextAttemptAt: nextAttemptAt,
		LastError:     lastError,
		CreatedAt:     createdAt,
	}
}
