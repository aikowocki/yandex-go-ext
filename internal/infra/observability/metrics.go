package observability

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const metricsMeterName = "gophprofile/metrics"

var metrics = newMetricSet()

type metricSet struct {
	once sync.Once

	httpRequests metric.Int64Counter
	httpDuration metric.Float64Histogram
	httpActive   metric.Int64UpDownCounter

	uploads                metric.Int64Counter
	uploadTime             metric.Float64Histogram
	uploadSize             metric.Int64Histogram
	processing             metric.Int64Counter
	processTime            metric.Float64Histogram
	publishes              metric.Int64Counter
	publishTime            metric.Float64Histogram
	consumes               metric.Int64Counter
	consumeTime            metric.Float64Histogram
	retries                metric.Int64Counter
	dlq                    metric.Int64Counter
	queueDepth             metric.Int64ObservableGauge
	queueValues            sync.Map
	dependencyAvailability metric.Int64ObservableGauge
	dependencyValues       sync.Map
}

type queueDepthValue struct {
	system      string
	destination string
	value       atomic.Int64
}

type dependencyAvailabilityValue struct {
	dependency string
	value      atomic.Int64
}

func newMetricSet() *metricSet { return &metricSet{} }

func (m *metricSet) init() {
	m.once.Do(func() {
		meter := otel.Meter(metricsMeterName)
		m.httpRequests, _ = meter.Int64Counter("http.server.request.count", metric.WithDescription("Количество HTTP-запросов."))
		m.httpDuration, _ = meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"), metric.WithDescription("Длительность HTTP-запросов."))
		m.httpActive, _ = meter.Int64UpDownCounter("http.server.active_requests", metric.WithDescription("Количество HTTP-запросов в обработке."))
		m.uploads, _ = meter.Int64Counter("avatars.uploads", metric.WithDescription("Количество загрузок аватаров."))
		m.uploadTime, _ = meter.Float64Histogram("avatars.upload.duration", metric.WithUnit("s"), metric.WithDescription("Длительность загрузки аватара."))
		m.uploadSize, _ = meter.Int64Histogram("avatars.upload.size", metric.WithUnit("By"), metric.WithDescription("Размер загружаемого аватара."))
		m.processing, _ = meter.Int64Counter("avatars.processing", metric.WithDescription("Количество попыток обработки аватара."))
		m.processTime, _ = meter.Float64Histogram("avatars.processing.duration", metric.WithUnit("s"), metric.WithDescription("Длительность обработки аватара."))
		m.publishes, _ = meter.Int64Counter("messaging.publish", metric.WithDescription("Количество опубликованных сообщений."))
		m.publishTime, _ = meter.Float64Histogram("messaging.publish.duration", metric.WithUnit("s"), metric.WithDescription("Длительность публикации сообщения."))
		m.consumes, _ = meter.Int64Counter("messaging.consume", metric.WithDescription("Количество обработанных сообщений."))
		m.consumeTime, _ = meter.Float64Histogram("messaging.consume.duration", metric.WithUnit("s"), metric.WithDescription("Длительность обработки сообщения."))
		m.retries, _ = meter.Int64Counter("messaging.retry", metric.WithDescription("Количество повторных попыток обработки сообщений."))
		m.dlq, _ = meter.Int64Counter("messaging.dlq", metric.WithDescription("Количество сообщений, отправленных в очередь недоставленных."))
		m.queueDepth, _ = meter.Int64ObservableGauge("messaging.queue.depth", metric.WithDescription("Количество сообщений, обрабатываемых consumer-компонентами в данный момент."))
		m.dependencyAvailability, _ = meter.Int64ObservableGauge("dependency.availability", metric.WithDescription("Последнее состояние доступности внешней зависимости: 1 — доступна, 0 — недоступна."))
		_, _ = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
			m.queueValues.Range(func(_, raw any) bool {
				value := raw.(*queueDepthValue)
				observer.ObserveInt64(m.queueDepth, value.value.Load(), metric.WithAttributes(
					attribute.String("messaging.system", value.system),
					attribute.String("messaging.destination.name", value.destination),
				))
				return true
			})
			m.dependencyValues.Range(func(_, raw any) bool {
				value := raw.(*dependencyAvailabilityValue)
				observer.ObserveInt64(m.dependencyAvailability, value.value.Load(), metric.WithAttributes(
					attribute.String("dependency", value.dependency),
				))
				return true
			})
			return nil
		}, m.queueDepth, m.dependencyAvailability)
	})
}

// BeginHTTP Начинает сбор HTTP RED-метрик и возвращает функцию фиксации завершения запроса.
func BeginHTTP(ctx context.Context, method, route string) func(status int, duration time.Duration) {
	metrics.init()
	baseAttrs := metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", safeRoute(route)),
	)
	metrics.httpActive.Add(ctx, 1, baseAttrs)
	return func(status int, duration time.Duration) {
		metrics.httpActive.Add(ctx, -1, baseAttrs)
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", method),
			attribute.String("http.route", safeRoute(route)),
			attribute.Int("http.response.status_code", normalizedStatus(status)),
		}
		metrics.httpRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
		metrics.httpDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}

func safeRoute(route string) string {
	if route == "" {
		return "unknown"
	}
	return route
}

func normalizedStatus(status int) int {
	if status < 100 {
		return 500
	}
	return status
}

// RecordAvatarUpload записывает результат и размер загрузки avatar.
func RecordAvatarUpload(ctx context.Context, status string, duration time.Duration, size int64) {
	metrics.init()
	attrs := metric.WithAttributes(attribute.String("status", safeStatus(status)))
	metrics.uploads.Add(ctx, 1, attrs)
	metrics.uploadTime.Record(ctx, duration.Seconds(), attrs)
	if size >= 0 {
		metrics.uploadSize.Record(ctx, size, attrs)
	}
}

// RecordAvatarProcessing записывает результат и длительность одной попытки обработки worker.
func RecordAvatarProcessing(ctx context.Context, status string, duration time.Duration) {
	metrics.init()
	attrs := metric.WithAttributes(attribute.String("status", safeStatus(status)))
	metrics.processing.Add(ctx, 1, attrs)
	metrics.processTime.Record(ctx, duration.Seconds(), attrs)
}

func safeStatus(status string) string {
	switch status {
	case "success", "error", "skipped":
		return status
	default:
		return "unknown"
	}
}

// RecordMessagingPublish записывает попытку публикации сообщения в broker.
func RecordMessagingPublish(ctx context.Context, system, destination, status string, duration time.Duration) {
	metrics.init()
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", safeMessagingValue(system)),
		attribute.String("messaging.destination.name", safeMessagingValue(destination)),
		attribute.String("status", safeStatus(status)),
	)
	metrics.publishes.Add(ctx, 1, attrs)
	metrics.publishTime.Record(ctx, duration.Seconds(), attrs)
}

// RecordMessagingConsume записывает попытку обработки сообщения broker.
func RecordMessagingConsume(ctx context.Context, system, destination, status string, duration time.Duration) {
	metrics.init()
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", safeMessagingValue(system)),
		attribute.String("messaging.destination.name", safeMessagingValue(destination)),
		attribute.String("status", safeStatus(status)),
	)
	metrics.consumes.Add(ctx, 1, attrs)
	metrics.consumeTime.Record(ctx, duration.Seconds(), attrs)
}

// RecordMessagingRetry записывает решение о повторной попытке или о переадресации без уведомления.
func RecordMessagingRetry(ctx context.Context, system, destination string, deadLetter bool) {
	metrics.init()
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", safeMessagingValue(system)),
		attribute.String("messaging.destination.name", safeMessagingValue(destination)),
	)
	if deadLetter {
		metrics.dlq.Add(ctx, 1, attrs)
		return
	}
	metrics.retries.Add(ctx, 1, attrs)
}

func safeMessagingValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

// ChangeQueueDepth изменяет число сообщений, которые сейчас обрабатываются consumer.
func ChangeQueueDepth(system, destination string, delta int64) {
	metrics.init()
	key := safeMessagingValue(system) + "\x00" + safeMessagingValue(destination)
	value, loaded := metrics.queueValues.LoadOrStore(key, &queueDepthValue{
		system:      safeMessagingValue(system),
		destination: safeMessagingValue(destination),
	})
	if !loaded {
		value = value.(*queueDepthValue)
	}
	value.(*queueDepthValue).value.Add(delta)
}

// SetDependencyAvailability сохраняет результат последней проверки зависимости.
// Метка dependency ограничена именами database, storage и broker.
func SetDependencyAvailability(dependency string, available bool) {
	if dependency != "database" && dependency != "storage" && dependency != "broker" {
		return
	}
	metrics.init()
	value, loaded := metrics.dependencyValues.LoadOrStore(dependency, &dependencyAvailabilityValue{dependency: dependency})
	if !loaded {
		value = value.(*dependencyAvailabilityValue)
	}
	state := int64(0)
	if available {
		state = 1
	}
	value.(*dependencyAvailabilityValue).value.Store(state)
}
