package broker

import (
	"context"
	"errors"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/broker/kafka"
	"github.com/aikowocki/yandex-go-ext/internal/infra/broker/rabbitmq"
)

func TestNewBrokerRejectsNilAndUnsupportedConfig(t *testing.T) {
	if _, err := NewBroker(context.Background(), nil, nil); err == nil {
		t.Fatal("nil broker config accepted")
	}
	if _, err := NewBroker(context.Background(), &config.BrokerConfig{Type: "unknown"}, nil); err == nil {
		t.Fatal("unsupported broker accepted")
	}
	if _, err := kafka.NewBroker(context.Background(), nil, nil); err == nil {
		t.Fatal("nil Kafka config accepted")
	}
	if _, err := kafka.NewBroker(context.Background(), &config.KafkaConfig{Brokers: nil, GroupID: "workers"}, nil); err == nil {
		t.Fatal("incomplete Kafka config accepted")
	}
	if _, err := rabbitmq.NewBroker(context.Background(), nil, nil); err == nil {
		t.Fatal("nil RabbitMQ config accepted")
	}
	if _, err := rabbitmq.NewBroker(context.Background(), &config.RabbitMQConfig{URL: "", Exchange: "avatars"}, nil); err == nil {
		t.Fatal("incomplete RabbitMQ config accepted")
	}
}

func TestBrokerConstructorsRespectCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	kafkaCfg := &config.KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "workers"}
	if _, err := kafka.NewBroker(ctx, kafkaCfg, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Kafka cancelled context error = %v", err)
	}
	rabbitCfg := &config.RabbitMQConfig{URL: "amqp://localhost", Exchange: "avatars"}
	if _, err := rabbitmq.NewBroker(ctx, rabbitCfg, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("RabbitMQ cancelled context error = %v", err)
	}
}

func TestNewBrokerFactoryDelegatesToBackends(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		cfg  config.BrokerConfig
	}{
		{
			name: "kafka",
			cfg: config.BrokerConfig{
				Type: "kafka",
				Kafka: config.KafkaConfig{
					Brokers: []string{"localhost:9092"},
					GroupID: "workers",
				},
			},
		},
		{
			name: "rabbitmq",
			cfg: config.BrokerConfig{
				Type: "rabbitmq",
				RabbitMQ: config.RabbitMQConfig{
					URL:      "amqp://localhost",
					Exchange: "avatars",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewBroker(ctx, &test.cfg, nil); !errors.Is(err, context.Canceled) {
				t.Fatalf("factory error = %v", err)
			}
		})
	}
}
