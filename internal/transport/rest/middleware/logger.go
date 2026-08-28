package middleware

import (
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/labstack/echo/v4"
)

// RequestLogger создаёт middleware для логирования HTTP-запросов.
func RequestLogger(logger logging.Logger) echo.MiddlewareFunc {
	if logger == nil {
		logger = logging.ComponentLogger(nil, "rest")
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			started := time.Now()
			ctx := logging.WithLogger(c.Request().Context(), logger)
			c.SetRequest(c.Request().WithContext(ctx))
			err := next(c)
			logger.Info(ctx, "http request",
				logging.String("method", c.Request().Method),
				logging.String("path", c.Path()),
				logging.Int("status", c.Response().Status),
				logging.Duration("duration", time.Since(started)),
			)
			return err
		}
	}
}
