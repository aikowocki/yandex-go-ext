package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/shared/testsupport"
	"github.com/labstack/echo/v4"
)

func TestRequestLoggerPreservesHandlerError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/avatars", nil)
	ctx := testsupport.Context(t)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()
	requestContext := e.NewContext(req, recorder)
	wantErr := errors.New("handler failed")
	next := func(echo.Context) error { return wantErr }
	handler := RequestLogger(nil)(next)
	gotErr := handler(requestContext)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("RequestLogger() error = %v", gotErr)
	}
	testsupport.Assert(t).
		Contains("http request").
		HasField("component", "rest")
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
