package kafka

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"testing"
	"time"
)

func TestParseMessageUsesNativeHeadersAndKeyFallback(t *testing.T) {
	stamp := time.Date(2026, time.August, 16, 17, 0, 0, 123000000, time.UTC)
	message := &sarama.ConsumerMessage{
		Topic: "avatar.uploaded",
		Key:   []byte("message-key"),
		Value: []byte(`{"avatar_id":"avatar-1"}`),
		Headers: []*sarama.RecordHeader{
			{Key: []byte("timestamp"), Value: []byte(stamp.Format(time.RFC3339Nano))},
			{Key: []byte(contracts.ContentTypeHeader), Value: []byte("application/json")},
			{Key: []byte(contracts.SchemaVersionHeader), Value: []byte("v2")},
			{Key: []byte(contracts.DeliveryAttemptHeader), Value: []byte("2")},
		},
	}

	got := parseMessage(message)
	if got.ID != "message-key" {
		t.Fatalf("expected key fallback as message ID, got %q", got.ID)
	}
	if !got.Timestamp.Equal(stamp) {
		t.Fatalf("expected timestamp %s, got %s", stamp, got.Timestamp)
	}
	if got.Headers[contracts.SchemaVersionHeader] != "v2" {
		t.Fatalf("expected schema version v2, got %q", got.Headers[contracts.SchemaVersionHeader])
	}
	if got.Headers[contracts.DeliveryAttemptHeader] != "2" {
		t.Fatalf("expected delivery attempt 2, got %q", got.Headers[contracts.DeliveryAttemptHeader])
	}
}

func TestParseMessageSupportsLegacyJSONHeaders(t *testing.T) {
	legacyHeaders, err := json.Marshal(map[string]string{
		contracts.ContentTypeHeader: "application/json",
		"custom":                    "value",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := parseMessage(&sarama.ConsumerMessage{
		Topic: "topic",
		Key:   []byte("id"),
		Value: []byte("payload"),
		Headers: []*sarama.RecordHeader{
			{Key: []byte("message_id"), Value: []byte("header-id")},
			{Key: []byte("headers"), Value: legacyHeaders},
		},
	})
	if got.ID != "header-id" {
		t.Fatalf("expected message_id header, got %q", got.ID)
	}
	if got.Headers["custom"] != "value" {
		t.Fatalf("expected legacy custom header, got %q", got.Headers["custom"])
	}
	if got.Headers[contracts.SchemaVersionHeader] != "v1" {
		t.Fatalf("expected default schema version v1, got %q", got.Headers[contracts.SchemaVersionHeader])
	}
}
func TestParseMessageDefaultsAndInvalidHeaders(t *testing.T) {
	got := parseMessage(&sarama.ConsumerMessage{Topic: "topic", Value: []byte("payload"), Headers: []*sarama.RecordHeader{
		{Key: []byte("timestamp"), Value: []byte("bad")},
		{Key: []byte("headers"), Value: []byte("bad json")},
	}})
	if got.ID != "" || got.Timestamp.IsZero() || got.Headers[contracts.ContentTypeHeader] != "application/json" || got.Headers[contracts.SchemaVersionHeader] != "v1" {
		t.Fatalf("unexpected parsed message: %+v", got)
	}
}

func TestKafkaHealthGuards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (&Broker{}).HealthCheck(ctx); err == nil {
		t.Fatal("cancelled Kafka health check succeeded")
	}
	if err := (&Broker{}).HealthCheck(context.Background()); err == nil {
		t.Fatal("unconfigured Kafka health check succeeded")
	}
}
