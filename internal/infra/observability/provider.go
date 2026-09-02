package observability

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	shutdownOnce   sync.Once
	shutdownErr    error
}

func New(ctx context.Context, cfg config.ObservabilityConfig, serviceName string) (*Provider, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	provider := &Provider{}
	if !cfg.Enabled {
		return provider, nil
	}
	if serviceName == "" {
		return nil, errors.New("observability service name is empty")
	}
	if cfg.TraceSampleRatio < 0 || cfg.TraceSampleRatio > 1 {
		return nil, fmt.Errorf("trace sample ratio must be between 0 and 1: %v", cfg.TraceSampleRatio)
	}

	exporterOptions := []otlptracegrpc.Option{otlptracegrpc.WithEndpointURL(cfg.OTLPEndpoint)}
	if cfg.OTLPInsecure {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, metricExporterOptions(cfg)...)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment.name", cfg.Environment),
		),
	)
	if err != nil {
		_ = metricExporter.Shutdown(ctx)
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	provider.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
	)
	provider.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	otel.SetTracerProvider(provider.tracerProvider)
	otel.SetMeterProvider(provider.meterProvider)
	return provider, nil
}

func metricExporterOptions(cfg config.ObservabilityConfig) []otlpmetricgrpc.Option {
	options := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpointURL(cfg.OTLPEndpoint)}
	if cfg.OTLPInsecure {
		options = append(options, otlpmetricgrpc.WithInsecure())
	}
	return options
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.shutdownOnce.Do(func() {
		if p.meterProvider != nil {
			p.shutdownErr = p.meterProvider.Shutdown(ctx)
		}
		if p.tracerProvider != nil {
			p.shutdownErr = errors.Join(p.shutdownErr, p.tracerProvider.Shutdown(ctx))
		}
	})
	return p.shutdownErr
}
