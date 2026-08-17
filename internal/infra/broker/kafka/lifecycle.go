package kafka

import "github.com/aikowocki/yandex-go-ext/internal/shared/logging"

// Close закрывает соединения брокера.
func (b *Broker) Close() error {
	var closeErr error
	b.closeOnce.Do(func() {
		if b.cancel != nil {
			b.cancel()
		}
		b.mu.Lock()
		if b.consumeCancel != nil {
			b.consumeCancel()
		}
		b.mu.Unlock()
		b.wg.Wait()
		if b.consumerGroup != nil {
			closeErr = b.consumerGroup.Close()
		}
		if b.healthClient != nil {
			if err := b.healthClient.Close(); closeErr == nil {
				closeErr = err
			}
		}
		if b.producer != nil {
			if err := b.producer.Close(); closeErr == nil {
				closeErr = err
			}
		}
		logging.Info(b.ctx, "kafka broker closed")
	})
	return closeErr
}
