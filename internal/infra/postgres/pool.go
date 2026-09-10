package postgres

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
)

// DB оборачивает пул соединений и выбирает querier для текущего context.
type DB struct {
	*pgxpool.Pool
	metricRegistration metric.Registration
	outboxPending      atomic.Int64
	storageUsage       atomic.Int64
	outboxMetricReady  atomic.Bool
	storageMetricReady atomic.Bool
	metricsCancel      context.CancelFunc
	metricsWG          sync.WaitGroup
}

// NewPool создаёт и проверяет пул PostgreSQL.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.Tracer = observability.PGXQueryTracer{}
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	db := &DB{Pool: pool}
	db.registerMetrics()
	db.startMetricRefresh(ctx)
	return db, nil
}

func (db *DB) registerMetrics() {
	if db == nil || db.Pool == nil {
		return
	}
	meter := otel.Meter("gophprofile/postgres")
	connections, err := meter.Int64ObservableGauge("db.client.connections", metric.WithDescription("Максимальное количество соединений в пуле PostgreSQL."))
	if err != nil {
		return
	}
	acquired, err := meter.Int64ObservableGauge("db.client.connections.usage", metric.WithDescription("Количество занятых соединений пула PostgreSQL."))
	if err != nil {
		return
	}
	idle, err := meter.Int64ObservableGauge("db.client.connections.idle", metric.WithDescription("Количество свободных соединений PostgreSQL в пуле."))
	if err != nil {
		return
	}
	outboxPending, err := meter.Int64ObservableGauge("outbox.pending", metric.WithDescription("Количество ожидающих событий transactional outbox."))
	if err != nil {
		return
	}
	storageUsage, err := meter.Int64ObservableGauge("storage.usage", metric.WithDescription("Объём байтов blob-объектов, сохранённых в PostgreSQL."))
	if err != nil {
		return
	}
	registration, err := meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		stat := db.Stat()
		attrs := metric.WithAttributes(attribute.String("db.system", "postgresql"), attribute.String("pool", "primary"))
		observer.ObserveInt64(connections, int64(stat.MaxConns()), attrs)
		observer.ObserveInt64(acquired, int64(stat.AcquiredConns()), attrs)
		observer.ObserveInt64(idle, int64(stat.IdleConns()), attrs)
		if db.outboxMetricReady.Load() {
			observer.ObserveInt64(outboxPending, db.outboxPending.Load(), attrs)
		}
		if db.storageMetricReady.Load() {
			observer.ObserveInt64(storageUsage, db.storageUsage.Load(), metric.WithAttributes(attribute.String("object.type", "blob")))
		}
		return nil
	}, connections, acquired, idle, outboxPending, storageUsage)
	if err == nil {
		db.metricRegistration = registration
	}
}

func (db *DB) startMetricRefresh(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	db.metricsCancel = cancel
	db.metricsWG.Go(func() {
		db.refreshMetrics(ctx)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				db.refreshMetrics(ctx)
			}
		}
	})
}

func (db *DB) refreshMetrics(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 250*time.Millisecond)
	defer cancel()
	var pending int64
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND next_attempt_at <= now()`).Scan(&pending); err == nil {
		db.outboxPending.Store(pending)
		db.outboxMetricReady.Store(true)
	}
	var storageBytes int64
	if err := db.Pool.QueryRow(ctx, `SELECT COALESCE(sum(size_bytes), 0) FROM blobs WHERE storage_status = 'ready'`).Scan(&storageBytes); err == nil {
		db.storageUsage.Store(storageBytes)
		db.storageMetricReady.Store(true)
	}
}

// Close снимает регистрацию метрик базы данных и закрывает пул PostgreSQL.
func (db *DB) Close() {
	if db == nil {
		return
	}
	if db.metricsCancel != nil {
		db.metricsCancel()
		db.metricsWG.Wait()
	}
	if db.metricRegistration != nil {
		_ = db.metricRegistration.Unregister()
	}
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// querier возвращает активную транзакцию из context или retrying pool querier.
func (db *DB) querier(ctx context.Context) gen.DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return retryingDB{pool: db.Pool}
}
