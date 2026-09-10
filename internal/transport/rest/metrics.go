package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/labstack/echo/v4"
)

func metricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			started := time.Now()
			finish := observability.BeginHTTP(c.Request().Context(), c.Request().Method, c.Path())
			err := next(c)
			status := c.Response().Status
			if status == 0 {
				status = errorStatus(err)
			}
			finish(status, time.Since(started))
			return err
		}
	}
}

func errorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr.Code > 0 {
		return httpErr.Code
	}
	return http.StatusInternalServerError
}
