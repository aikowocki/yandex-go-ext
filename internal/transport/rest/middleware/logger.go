package middleware

import (
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/labstack/echo/v4"
)

// RequestLogger записывает сведения об HTTP-запросе.
func RequestLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		started := time.Now()
		err := next(c)
		logging.Info(c.Request().Context(), "http request",
			logging.String("method", c.Request().Method),
			logging.String("path", c.Path()),
			logging.Int("status", c.Response().Status),
			logging.Duration("duration", time.Since(started)),
		)
		return err
	}
}
