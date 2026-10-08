package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunWithRetryEventuallySucceeds(t *testing.T) {
	attempts := 0
	err := runWithRetry(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("database is not ready")
		}
		return nil
	}, 3, time.Millisecond)
	if err != nil {
		t.Fatalf("runWithRetry() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRunWithRetryStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runWithRetry(ctx, func() error {
		t.Fatal("operation should not run after cancellation")
		return nil
	}, 3, time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runWithRetry() error = %v, want context.Canceled", err)
	}
}

func TestRunWithRetryRejectsInvalidAttempts(t *testing.T) {
	err := runWithRetry(context.Background(), func() error { return nil }, 0, time.Millisecond)
	if err == nil {
		t.Fatal("runWithRetry() accepted zero attempts")
	}
}
