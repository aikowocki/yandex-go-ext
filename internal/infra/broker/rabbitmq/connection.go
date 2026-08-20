package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

func connectPublisherWithRetry(ctx context.Context, cfg *config.RabbitMQConfig) (*amqp.Connection, *amqp.Channel, <-chan amqp.Confirmation, <-chan amqp.Return, error) {
	var lastErr error
	delay := rabbitMQStartupInitialDelay
	for attempt := 1; attempt <= rabbitMQStartupMaxAttempts; attempt++ {
		conn, channel, confirms, returns, err := connectPublisherOnce(cfg)
		if err == nil {
			return conn, channel, confirms, returns, nil
		}
		lastErr = err
		if attempt == rabbitMQStartupMaxAttempts {
			break
		}
		logging.Warn(ctx, "RabbitMQ publisher connection failed, retrying", logging.Int("attempt", attempt), logging.Int("max_attempts", rabbitMQStartupMaxAttempts), logging.Duration("retry_after", delay), logging.Err(err))
		if err := waitRetry(ctx, delay); err != nil {
			return nil, nil, nil, nil, err
		}
		delay = nextRetryDelay(delay)
	}
	return nil, nil, nil, nil, fmt.Errorf("connect RabbitMQ publisher after %d attempts: %w", rabbitMQStartupMaxAttempts, lastErr)
}

func connectConsumerWithRetry(ctx context.Context, cfg *config.RabbitMQConfig) (*amqp.Connection, *amqp.Channel, error) {
	var lastErr error
	delay := rabbitMQStartupInitialDelay
	for attempt := 1; attempt <= rabbitMQStartupMaxAttempts; attempt++ {
		conn, channel, err := connectConsumerOnce(cfg)
		if err == nil {
			return conn, channel, nil
		}
		lastErr = err
		if attempt == rabbitMQStartupMaxAttempts {
			break
		}
		logging.Warn(ctx, "RabbitMQ consumer connection failed, retrying", logging.Int("attempt", attempt), logging.Int("max_attempts", rabbitMQStartupMaxAttempts), logging.Duration("retry_after", delay), logging.Err(err))
		if err := waitRetry(ctx, delay); err != nil {
			return nil, nil, err
		}
		delay = nextRetryDelay(delay)
	}
	return nil, nil, fmt.Errorf("connect RabbitMQ consumer after %d attempts: %w", rabbitMQStartupMaxAttempts, lastErr)
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextRetryDelay(delay time.Duration) time.Duration {
	if delay < rabbitMQStartupMaxDelay/2 {
		return delay * 2
	}
	return rabbitMQStartupMaxDelay
}

func dialWithRecovery(url string) (*amqp.Connection, error) {
	return amqp.DialConfig(url, amqp.Config{Recovery: &amqp.Recovery{}})
}

func connectPublisherOnce(cfg *config.RabbitMQConfig) (*amqp.Connection, *amqp.Channel, <-chan amqp.Confirmation, <-chan amqp.Return, error) {
	conn, err := dialWithRecovery(cfg.URL)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("dial RabbitMQ publisher: %w", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, nil, fmt.Errorf("create RabbitMQ publisher channel: %w", err)
	}
	if err := declareExchanges(channel, cfg.Exchange); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, nil, nil, nil, err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 128))
	returns := channel.NotifyReturn(make(chan amqp.Return, 128))
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, nil, nil, nil, fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}
	return conn, channel, confirms, returns, nil
}

func connectConsumerOnce(cfg *config.RabbitMQConfig) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := dialWithRecovery(cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("dial RabbitMQ consumer: %w", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("create RabbitMQ consumer channel: %w", err)
	}
	if err := declareExchanges(channel, cfg.Exchange); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, channel, nil
}

func watchConnection(ctx context.Context, conn *amqp.Connection, role string) {
	stateChanges := make(chan *amqp.StateChanged, 8)
	conn.NotifyStateChange(stateChanges)
	go func() {
		for state := range stateChanges {
			if state == nil {
				continue
			}
			logging.Info(ctx, "RabbitMQ connection state changed", logging.String("role", role), logging.String("from", state.From.String()), logging.String("to", state.To.String()), logging.Err(state.Err))
		}
	}()
}
