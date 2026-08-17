package rabbitmq

import (
	"context"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestDeliveryMessageAddsAttemptMetadata(t *testing.T) {
	message := deliveryMessage("avatar.uploaded", amqp.Delivery{MessageId: "message-1", Body: []byte("payload")}, 5)

	if message.ID != "message-1" {
		t.Fatalf("message id = %q, want message-1", message.ID)
	}
	if got := message.Headers[contracts.DeliveryAttemptHeader]; got != "1" {
		t.Fatalf("delivery attempt = %q, want 1", got)
	}
	if got := message.Headers[contracts.MaxDeliveryAttemptsHeader]; got != "5" {
		t.Fatalf("max delivery attempts = %q, want 5", got)
	}
	if message.Timestamp.IsZero() {
		t.Fatal("timestamp must be populated")
	}
}

func TestDeliveryMessagePreservesAttemptMetadata(t *testing.T) {
	message := deliveryMessage("avatar.uploaded", amqp.Delivery{
		MessageId: "message-2",
		Headers: amqp.Table{
			contracts.DeliveryAttemptHeader: "3",
		},
	}, 5)

	if got := deliveryAttempt(message); got != 3 {
		t.Fatalf("delivery attempt = %d, want 3", got)
	}
}

func TestNextRetryDelayCapsAtMaximum(t *testing.T) {
	if got := nextRetryDelay(time.Second); got != 2*time.Second {
		t.Fatalf("next delay = %s, want 2s", got)
	}
	if got := nextRetryDelay(3 * time.Second); got != rabbitMQStartupMaxDelay {
		t.Fatalf("capped delay = %s, want %s", got, rabbitMQStartupMaxDelay)
	}
}
func TestRabbitMQHealthGuardsAndTopologyHelpers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (&Broker{}).HealthCheck(ctx); err == nil {
		t.Fatal("cancelled RabbitMQ health check succeeded")
	}
	if err := (*Broker)(nil).HealthCheck(context.Background()); err == nil {
		t.Fatal("nil RabbitMQ health check succeeded")
	}
	if err := (&Broker{}).HealthCheck(context.Background()); err == nil {
		t.Fatal("unconfigured RabbitMQ health check succeeded")
	}
	if got := consumerTag("avatar.uploaded/v1"); got != "gophprofile-avatar-uploaded-v1" {
		t.Fatalf("consumer tag = %q", got)
	}
	message := deliveryMessage("topic", amqp.Delivery{Headers: amqp.Table{contracts.DeliveryAttemptHeader: "bad"}}, 3)
	if deliveryAttempt(message) != 1 {
		t.Fatalf("invalid delivery attempt = %d", deliveryAttempt(message))
	}
	if deliveryAttempt(nil) != 1 {
		t.Fatal("nil delivery attempt is not one")
	}
}
