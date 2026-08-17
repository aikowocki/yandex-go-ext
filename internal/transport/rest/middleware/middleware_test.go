package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequestLoggerPreservesHandlerError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/avatars", nil)
	recorder := httptest.NewRecorder()
	ctx := e.NewContext(req, recorder)
	wantErr := errors.New("handler failed")
	gotErr := RequestLogger(func(echo.Context) error { return wantErr })(ctx)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("RequestLogger() error = %v", gotErr)
	}
}

func TestRecoveryConvertsPanicToHTTPError(t *testing.T) {
	e := echo.New()
	e.Use(Recovery())
	e.GET("/panic", func(echo.Context) error { panic("boom") })
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	e.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic status = %d", recorder.Code)
	}
}
