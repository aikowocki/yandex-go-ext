package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/transport/rest/dto"
	"github.com/labstack/echo/v4"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/aikowocki/yandex-go-ext/internal/shared/testsupport"
)

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		wantError  string
		wantDetail string
		wantMax    int64
	}{
		{name: "invalid input", err: domain.ErrInvalidInput, status: http.StatusBadRequest, wantError: domain.ErrInvalidInput.Error()},
		{name: "invalid format", err: domain.ErrInvalidFormat, status: http.StatusBadRequest, wantError: "Invalid file format", wantDetail: "Supported formats: jpeg, png, webp"},
		{name: "too large", err: domain.ErrFileTooLarge, status: http.StatusRequestEntityTooLarge, wantError: "File too large", wantMax: 10 * 1024 * 1024},
		{name: "forbidden", err: domain.ErrForbidden, status: http.StatusForbidden, wantError: "Forbidden", wantDetail: "You can only delete your own avatars"},
		{name: "not found", err: domain.ErrNotFound, status: http.StatusNotFound, wantError: "Avatar not found"},
		{name: "already exists", err: domain.ErrAlreadyExists, status: http.StatusConflict, wantError: domain.ErrAlreadyExists.Error()},
		{name: "conflict", err: domain.ErrConflict, status: http.StatusConflict, wantError: domain.ErrConflict.Error()},
		{name: "unknown", err: assertAnError{}, status: http.StatusInternalServerError, wantError: "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			recorder := httptest.NewRecorder()
			ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), recorder)
			if err := respondError(ctx, tt.err); err != nil {
				t.Fatalf("respondError() error = %v", err)
			}
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			var got dto.ErrorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Error != tt.wantError || got.Details != tt.wantDetail || got.MaxSize != tt.wantMax {
				t.Fatalf("response = %+v", got)
			}
		})
	}
}

type assertAnError struct{}

func (assertAnError) Error() string { return "unexpected failure" }

func TestRespondErrorLogsUnknownErrorWithoutExposingDetails(t *testing.T) {
	e := echo.New()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(testsupport.Context(t))
	ctx := e.NewContext(request, recorder)
	internalErr := assertAnError{}

	if err := respondError(ctx, internalErr); err != nil {
		t.Fatalf("respondError() error = %v", err)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var response dto.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "internal server error" || response.Details != "" {
		t.Fatalf("response exposed internal details: %+v", response)
	}
	testsupport.Assert(t).
		Contains("internal server error").
		HasLevel(logging.ErrorLevel).
		HasField("error", internalErr)
}
