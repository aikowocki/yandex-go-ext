package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWithRetryRetriesRetryableError(t *testing.T) {
	var attempts int
	transient := errors.New("transient")

	err := withRetry(context.Background(), func(error) bool { return true }, func() error {
		attempts++
		if attempts < 3 {
			return transient
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withRetry() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("withRetry() attempts = %d, want 3", attempts)
	}
}

func TestWithRetryStopsOnNonRetryableError(t *testing.T) {
	var attempts int
	expected := errors.New("permanent")

	err := withRetry(context.Background(), func(error) bool { return false }, func() error {
		attempts++
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("withRetry() error = %v, want %v", err, expected)
	}
	if attempts != 1 {
		t.Fatalf("withRetry() attempts = %d, want 1", attempts)
	}
}

func TestWithRetryStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var attempts int

	err := withRetry(ctx, func(error) bool { return true }, func() error {
		attempts++
		return errors.New("transient")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("withRetry() error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("withRetry() attempts = %d, want 1", attempts)
	}
}

func TestIsTxRetryable(t *testing.T) {
	for _, code := range []string{pgerrcode.SerializationFailure, pgerrcode.DeadlockDetected} {
		if !isTxRetryable(&pgconn.PgError{Code: code}) {
			t.Fatalf("isTxRetryable(%s) = false", code)
		}
	}
	if isTxRetryable(&pgconn.PgError{Code: pgerrcode.UniqueViolation}) {
		t.Fatal("unique violation must not be transaction-retryable")
	}
}

func TestUniqueViolation(t *testing.T) {
	constraint := "users_login_key"
	name, ok := uniqueViolation(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: constraint})
	if !ok || name != constraint {
		t.Fatalf("uniqueViolation() = %q, %v; want %q, true", name, ok, constraint)
	}
}
