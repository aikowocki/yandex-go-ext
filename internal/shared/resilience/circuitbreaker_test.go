package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreakerOpensAfterFailuresAndAllowsProbeAfterReset(t *testing.T) {
	breaker := New(Config{MaxFailures: 2, ResetTimeout: 10 * time.Millisecond})
	failure := errors.New("dependency unavailable")
	call := func() error { return breaker.Do(context.Background(), nil, func() error { return failure }) }

	if !errors.Is(call(), failure) || !errors.Is(call(), failure) {
		t.Fatal("expected dependency failures")
	}
	if !errors.Is(call(), ErrCircuitOpen) {
		t.Fatal("expected open circuit")
	}

	time.Sleep(15 * time.Millisecond)
	if err := breaker.Do(context.Background(), nil, func() error { return nil }); err != nil {
		t.Fatalf("probe should close circuit: %v", err)
	}
	if err := breaker.Do(context.Background(), nil, func() error { return nil }); err != nil {
		t.Fatalf("closed circuit rejected call: %v", err)
	}
}

func TestBreakerDoesNotTripForIgnoredErrors(t *testing.T) {
	breaker := New(Config{MaxFailures: 1, ResetTimeout: time.Hour})
	ignored := errors.New("caller error")
	for range 3 {
		if err := breaker.Do(context.Background(), func(error) bool { return false }, func() error { return ignored }); !errors.Is(err, ignored) {
			t.Fatalf("error = %v, want %v", err, ignored)
		}
	}
	if err := breaker.Do(context.Background(), nil, func() error { return nil }); err != nil {
		t.Fatalf("ignored errors opened circuit: %v", err)
	}
}

func TestBreakerHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	breaker := New(Config{})
	if err := breaker.Do(ctx, nil, func() error { called = true; return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("cancelled context called dependency")
	}
}
