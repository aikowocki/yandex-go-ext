package kafka

import (
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
)

func parseMessage(message *sarama.ConsumerMessage) *contracts.Message {
	result := &contracts.Message{Topic: message.Topic, Payload: message.Value, Headers: make(map[string]string)}
	for _, header := range message.Headers {
		switch string(header.Key) {
		case "message_id":
			result.ID = string(header.Value)
		case "timestamp":
			if timestamp, err := time.Parse(time.RFC3339Nano, string(header.Value)); err == nil {
				result.Timestamp = timestamp
			}
		case "headers":
			_ = json.Unmarshal(header.Value, &result.Headers)
		default:
			result.Headers[string(header.Key)] = string(header.Value)
		}
	}
	if result.ID == "" {
		result.ID = string(message.Key)
	}
	if result.Timestamp.IsZero() {
		result.Timestamp = time.Now().UTC()
	}
	if _, ok := result.Headers[contracts.ContentTypeHeader]; !ok {
		result.Headers[contracts.ContentTypeHeader] = "application/json"
	}
	if _, ok := result.Headers[contracts.SchemaVersionHeader]; !ok {
		result.Headers[contracts.SchemaVersionHeader] = "v1"
	}
	return result
}
