package rabbitmq

import (
	"strconv"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	amqp "github.com/rabbitmq/amqp091-go"
)

func deliveryMessage(topic string, delivery amqp.Delivery, maxAttempts int) *contracts.Message {
	headers := make(map[string]string, len(delivery.Headers)+2)
	for key, value := range delivery.Headers {
		switch stringValue := value.(type) {
		case []byte:
			headers[key] = string(stringValue)
		case string:
			headers[key] = stringValue
		}
	}
	messageID := delivery.MessageId
	if messageID == "" {
		messageID = headers["message_id"]
	}
	attempt := 1
	if parsed, err := strconv.Atoi(headers[contracts.DeliveryAttemptHeader]); err == nil && parsed > 0 {
		attempt = parsed
	}
	headers[contracts.DeliveryAttemptHeader] = strconv.Itoa(attempt)
	headers[contracts.MaxDeliveryAttemptsHeader] = strconv.Itoa(maxAttempts)
	timestamp := delivery.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return &contracts.Message{ID: messageID, Topic: topic, Payload: delivery.Body, Headers: headers, Timestamp: timestamp}
}

func deliveryAttempt(message *contracts.Message) int {
	if message != nil {
		if attempt, err := strconv.Atoi(message.Headers[contracts.DeliveryAttemptHeader]); err == nil && attempt > 0 {
			return attempt
		}
	}
	return 1
}
