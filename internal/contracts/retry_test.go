package contracts

import (
	"context"
	"errors"
	"testing"
)

func TestHandleWithRetryPublishesAttemptMetadata(t *testing.T) {
	message := NewMessage("topic", nil)
	attempts := 0
	if err := HandleWithRetry(context.Background(), message, func(_ context.Context, got *Message) error {
		attempts++
		if got.Headers[DeliveryAttemptHeader] != "2" && attempts == 2 {
			t.Fatalf("attempt header = %q, want 2", got.Headers[DeliveryAttemptHeader])
		}
		if got.Headers[MaxDeliveryAttemptsHeader] != "2" {
			t.Fatalf("max attempt header = %q, want 2", got.Headers[MaxDeliveryAttemptsHeader])
		}
		return errors.New("retry")
	}, 2); err == nil {
		t.Fatal("expected retry error")
	}
	if attempts != 2 {
		t.Fatalf("handler attempts = %d, want 2", attempts)
	}
}
func TestHandleWithRetryGuardAndSuccessPaths(t *testing.T) {
	if err := HandleWithRetry(context.Background(), NewMessage("topic", nil), nil, 1); err == nil {
		t.Fatal("nil handler accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := HandleWithRetry(ctx, NewMessage("topic", nil), func(context.Context, *Message) error { return nil }, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context error = %v", err)
	}
	message := &Message{Topic: "topic"}
	calls := 0
	if err := HandleWithRetry(context.Background(), message, func(_ context.Context, got *Message) error {
		calls++
		if got.Headers[DeliveryAttemptHeader] != "1" || got.Headers[MaxDeliveryAttemptsHeader] != "1" {
			t.Fatalf("retry headers = %v", got.Headers)
		}
		return nil
	}, 0); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d", calls)
	}
}

func TestHandleWithRetryStopsWhenContextCanceledDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	message := NewMessage("topic", nil)
	calls := 0
	err := HandleWithRetry(ctx, message, func(context.Context, *Message) error {
		calls++
		cancel()
		return errors.New("retry")
	}, 2)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("retry cancellation error=%v calls=%d", err, calls)
	}
}

func TestHandleWithRetryWaitsAndSucceedsOnSecondAttempt(t *testing.T) {
	message := NewMessage("topic", nil)
	calls := 0
	err := HandleWithRetry(context.Background(), message, func(context.Context, *Message) error {
		calls++
		if calls == 1 {
			return errors.New("temporary")
		}
		return nil
	}, 2)
	if err != nil || calls != 2 {
		t.Fatalf("retry result=%v calls=%d", err, calls)
	}
}
