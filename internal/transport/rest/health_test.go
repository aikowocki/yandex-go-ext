package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHealthResponse(t *testing.T) {
	tests := []struct {
		name   string
		checks DependencyChecks
		status int
		want   string
	}{
		{name: "not configured", status: http.StatusServiceUnavailable, want: "unhealthy"},
		{
			name: "healthy",
			checks: DependencyChecks{
				Database: func(context.Context) error { return nil },
				Storage:  func(context.Context) error { return nil },
				Broker:   func(context.Context) error { return nil },
			},
			status: http.StatusOK,
			want:   "ok",
		},
		{
			name: "dependency failure",
			checks: DependencyChecks{
				Database: func(context.Context) error { return errors.New("db down") },
				Storage:  func(context.Context) error { return nil },
				Broker:   func(context.Context) error { return nil },
			},
			status: http.StatusServiceUnavailable,
			want:   "unhealthy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			recorder := httptest.NewRecorder()
			ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/health", nil), recorder)
			if err := healthResponse(ctx, tt.checks); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			var body struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Status != tt.want {
				t.Fatalf("status body = %q, want %q", body.Status, tt.want)
			}
		})
	}
}
