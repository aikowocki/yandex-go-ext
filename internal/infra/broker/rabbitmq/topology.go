package rabbitmq

import (
	"fmt"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

func declareExchanges(channel *amqp.Channel, exchange string) error {
	if err := channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq exchange %q: %w", exchange, err)
	}
	if err := channel.ExchangeDeclare(exchange+".retry", "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq retry exchange: %w", err)
	}
	if err := channel.ExchangeDeclare(exchange+".dlx", "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq dead-letter exchange: %w", err)
	}
	return nil
}

func consumerTag(topic string) string {
	return "gophprofile-" + strings.NewReplacer(".", "-", "/", "-").Replace(topic)
}

func (b *Broker) declareSubscriptionTopology(topic string) (string, error) {
	queueName := "gophprofile." + topic
	mainArgs := amqp.Table{}
	if b.queueType == "quorum" {
		mainArgs["x-queue-type"] = amqp.QueueTypeQuorum
	}
	if _, err := b.consumerChannel.QueueDeclare(queueName, true, false, false, false, mainArgs); err != nil {
		return "", fmt.Errorf("declare rabbitmq queue %q: %w", queueName, err)
	}
	if err := b.consumerChannel.QueueBind(queueName, topic, b.exchange, false, nil); err != nil {
		return "", fmt.Errorf("bind rabbitmq queue %q: %w", queueName, err)
	}

	retryQueue := queueName + ".retry"
	retryArgs := amqp.Table{
		"x-message-ttl":             int32(b.retryDelay.Milliseconds()),
		"x-dead-letter-exchange":    b.exchange,
		"x-dead-letter-routing-key": topic,
	}
	if _, err := b.consumerChannel.QueueDeclare(retryQueue, true, false, false, false, retryArgs); err != nil {
		return "", fmt.Errorf("declare rabbitmq retry queue %q: %w", retryQueue, err)
	}
	if err := b.consumerChannel.QueueBind(retryQueue, topic, b.retryExchange, false, nil); err != nil {
		return "", fmt.Errorf("bind rabbitmq retry queue %q: %w", retryQueue, err)
	}

	deadQueue := queueName + ".dlq"
	if _, err := b.consumerChannel.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
		return "", fmt.Errorf("declare rabbitmq dead-letter queue %q: %w", deadQueue, err)
	}
	if err := b.consumerChannel.QueueBind(deadQueue, topic, b.deadExchange, false, nil); err != nil {
		return "", fmt.Errorf("bind rabbitmq dead-letter queue %q: %w", deadQueue, err)
	}
	return queueName, nil
}
