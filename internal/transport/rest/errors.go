package rest

import (
	"errors"
	"net/http"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/transport/rest/dto"
	"github.com/labstack/echo/v4"
)

func respondError(c echo.Context, err error) error {
	status := http.StatusInternalServerError
	response := dto.ErrorResponse{Error: "internal server error"}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
		response.Error = err.Error()
	case errors.Is(err, domain.ErrInvalidFormat):
		status = http.StatusBadRequest
		response.Error = "Invalid file format"
		response.Details = "Supported formats: jpeg, png, webp"
	case errors.Is(err, domain.ErrFileTooLarge):
		status = http.StatusRequestEntityTooLarge
		response.Error = "File too large"
		response.MaxSize = 10 * 1024 * 1024
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
		response.Error = "Forbidden"
		response.Details = "You can only delete your own avatars"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		response.Error = "Avatar not found"
	case errors.Is(err, domain.ErrAlreadyExists), errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		response.Error = err.Error()
	}
	return c.JSON(status, response)
}
