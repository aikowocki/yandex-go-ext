package observability

import (
	"context"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestProviderDisabledLifecycle(t *testing.T) {
	provider, err := New(context.Background(), config.ObservabilityConfig{}, "")
	if err != nil {
		t.Fatalf("New disabled provider: %v", err)
	}
	if provider.Logger("test") != nil {
		t.Fatal("disabled provider returned a logger")
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
	if err := (*Provider)(nil).Shutdown(context.Background()); err != nil {
		t.Fatalf("nil Shutdown: %v", err)
	}
}

func TestProviderRejectsInvalidEnabledConfiguration(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.ObservabilityConfig
		want string
	}{
		{name: "empty service", cfg: config.ObservabilityConfig{Enabled: true}, want: "service name"},
		{name: "negative ratio", cfg: config.ObservabilityConfig{Enabled: true, TraceSampleRatio: -0.1}, want: "sample ratio"},
		{name: "ratio above one", cfg: config.ObservabilityConfig{Enabled: true, TraceSampleRatio: 1.1}, want: "sample ratio"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := "service"
			if tc.name == "empty service" {
				serviceName = ""
			}
			_, err := New(context.Background(), tc.cfg, serviceName)
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
