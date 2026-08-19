package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Broker отправляет и принимает сообщения через RabbitMQ.
type Broker struct {
	ctx    context.Context
	cancel context.CancelFunc

	publisherConn    *amqp.Connection
	publisherChannel *amqp.Channel
	consumerConn     *amqp.Connection
	consumerChannel  *amqp.Channel

	publisherConfirms <-chan amqp.Confirmation
	publisherReturns  <-chan amqp.Return

	exchange      string
	retryExchange string
	deadExchange  string
	prefetch      int
	retryDelay    time.Duration
	maxAttempts   int
	queueType     string

	mu            sync.RWMutex
	publishMu     sync.Mutex
	consumerMu    sync.Mutex
	subscriptions map[string]struct{}
	consumerWG    sync.WaitGroup
	closeOnce     sync.Once
	closed        bool
}

const (
	rabbitMQStartupMaxAttempts  = 12
	rabbitMQStartupInitialDelay = time.Second
	rabbitMQStartupMaxDelay     = 5 * time.Second
	outboxDefaultRetryDelay     = 30 * time.Second
	outboxDefaultMaxAttempts    = 5
	rabbitMQShutdownTimeout     = 10 * time.Second
)

// NewBroker создаёт подключение к RabbitMQ.
func NewBroker(ctx context.Context, cfg *config.RabbitMQConfig, concurrency ...int) (*Broker, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prefetch := 1
	if len(concurrency) > 0 && concurrency[0] > 0 {
		prefetch = concurrency[0]
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = outboxDefaultRetryDelay
	}
	maxAttempts := cfg.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = outboxDefaultMaxAttempts
	}
	queueType := cfg.QueueType
	if queueType == "" {
		queueType = "classic"
	}
	if queueType != "classic" && queueType != "quorum" {
		return nil, fmt.Errorf("unsupported rabbitmq queue type: %q", queueType)
	}

	publisherConn, publisherChannel, confirms, returns, err := connectPublisherWithRetry(ctx, cfg)
	if err != nil {
		return nil, err
	}
	consumerConn, consumerChannel, err := connectConsumerWithRetry(ctx, cfg)
	if err != nil {
		_ = publisherConn.Close()
		return nil, err
	}

	brokerCtx, cancel := context.WithCancel(ctx)
	broker := &Broker{
		ctx:               brokerCtx,
		cancel:            cancel,
		publisherConn:     publisherConn,
		publisherChannel:  publisherChannel,
		consumerConn:      consumerConn,
		consumerChannel:   consumerChannel,
		publisherConfirms: confirms,
		publisherReturns:  returns,
		exchange:          cfg.Exchange,
		retryExchange:     cfg.Exchange + ".retry",
		deadExchange:      cfg.Exchange + ".dlx",
		prefetch:          prefetch,
		retryDelay:        retryDelay,
		maxAttempts:       maxAttempts,
		queueType:         queueType,
		subscriptions:     make(map[string]struct{}),
	}
	watchConnection(brokerCtx, publisherConn, "publisher")
	watchConnection(brokerCtx, consumerConn, "consumer")
	return broker, nil
}
