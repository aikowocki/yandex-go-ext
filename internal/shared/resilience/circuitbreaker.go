package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen означает, что зависимость временно недоступна, а вызовы
// отклоняются до истечения времени сброса.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Config управляет переходами circuit breaker между состояниями.
type Config struct {
	MaxFailures  int
	ResetTimeout time.Duration
}

const (
	defaultMaxFailures  = 5
	defaultResetTimeout = 30 * time.Second
)

// Breaker предотвращает повторные вызовы неисправной зависимости.
type Breaker struct {
	mu           sync.Mutex
	maxFailures  int
	resetTimeout time.Duration
	failures     int
	openedAt     time.Time
	probeActive  bool
}

// New создаёт circuit breaker в закрытом состоянии.
func New(cfg Config) *Breaker {
	maxFailures := cfg.MaxFailures
	if maxFailures <= 0 {
		maxFailures = defaultMaxFailures
	}
	resetTimeout := cfg.ResetTimeout
	if resetTimeout <= 0 {
		resetTimeout = defaultResetTimeout
	}
	return &Breaker{maxFailures: maxFailures, resetTimeout: resetTimeout}
}

// Do выполняет fn, если circuit breaker не открыт. classifyFailure определяет,
// какие ошибки указывают на сбой зависимости; ошибки валидации и вызывающего
// кода не должны открывать circuit breaker.
func (b *Breaker) Do(ctx context.Context, classifyFailure func(error) bool, fn func() error) error {
	if b == nil {
		return fn()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return errors.New("circuit breaker function is nil")
	}

	if !b.allow() {
		return ErrCircuitOpen
	}
	err := fn()
	if err == nil {
		b.success()
		return nil
	}
	if classifyFailure == nil || classifyFailure(err) {
		b.failure()
	}
	return err
}

func (b *Breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openedAt.IsZero() {
		return true
	}
	if time.Since(b.openedAt) < b.resetTimeout {
		return false
	}
	if b.probeActive {
		return false
	}
	b.probeActive = true
	return true
}

func (b *Breaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openedAt = time.Time{}
	b.probeActive = false
}

func (b *Breaker) failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probeActive = false
	b.failures++
	if b.failures >= b.maxFailures {
		b.openedAt = time.Now()
	}
}
