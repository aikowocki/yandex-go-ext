package kafka

import (
	"context"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// Broker отправляет и принимает сообщения через Kafka.
type Broker struct {
	producer      sarama.SyncProducer
	consumerGroup sarama.ConsumerGroup
	healthClient  sarama.Client
	brokers       []string
	groupID       string
	maxAttempts   int
	dlqSuffix     string

	ctx    context.Context
	cancel context.CancelFunc
	logger logging.Logger

	mu            sync.RWMutex
	handlers      map[string]contracts.MessageHandler
	consumeCancel context.CancelFunc
	started       bool
	wg            sync.WaitGroup
	closeOnce     sync.Once
}

// NewBroker создаёт подключение к Kafka.
func NewBroker(ctx context.Context, cfg *config.KafkaConfig, logger logging.Logger) (*Broker, error) {
	if logger == nil {
		logger = logging.ComponentLogger(nil, "broker")
	}
	if cfg == nil || len(cfg.Brokers) == 0 || cfg.GroupID == "" {
		return nil, fmt.Errorf("kafka configuration is incomplete")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = logging.WithLogger(ctx, logger)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	maxAttempts := cfg.MaxAttempts
	dlqSuffix := cfg.DLQSuffix
	sessionTimeout := cfg.SessionTimeout
	heartbeatInterval := cfg.HeartbeatInterval
	fetchMinBytes := cfg.FetchMinBytes
	fetchMaxWait := cfg.FetchMaxWait

	producerConfig := sarama.NewConfig()
	producerConfig.Version = sarama.V3_0_0_0
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 3
	producerConfig.Producer.Idempotent = true
	producerConfig.Net.MaxOpenRequests = 1
	producer, err := sarama.NewSyncProducer(cfg.Brokers, producerConfig)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}

	consumerConfig := sarama.NewConfig()
	consumerConfig.Version = sarama.V3_0_0_0
	consumerConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	consumerConfig.Consumer.Group.Session.Timeout = sessionTimeout
	consumerConfig.Consumer.Group.Heartbeat.Interval = heartbeatInterval
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerConfig.Consumer.Offsets.AutoCommit.Enable = true
	consumerConfig.Consumer.Fetch.Min = fetchMinBytes
	consumerConfig.Consumer.MaxWaitTime = fetchMaxWait
	consumerConfig.Consumer.Return.Errors = true
	consumerGroup, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, consumerConfig)
	if err != nil {
		_ = producer.Close()
		return nil, fmt.Errorf("create Kafka consumer group: %w", err)
	}
	healthClient, err := sarama.NewClient(cfg.Brokers, producerConfig)
	if err != nil {
		_ = consumerGroup.Close()
		_ = producer.Close()
		return nil, fmt.Errorf("create Kafka health client: %w", err)
	}

	brokerCtx, cancel := context.WithCancel(ctx)
	broker := &Broker{
		logger:        logger,
		producer:      producer,
		consumerGroup: consumerGroup,
		healthClient:  healthClient,
		brokers:       append([]string(nil), cfg.Brokers...),
		groupID:       cfg.GroupID,
		maxAttempts:   maxAttempts,
		dlqSuffix:     dlqSuffix,
		ctx:           brokerCtx,
		cancel:        cancel,
		handlers:      make(map[string]contracts.MessageHandler),
	}
	broker.wg.Add(1)
	go broker.consumeErrors()
	return broker, nil
}
