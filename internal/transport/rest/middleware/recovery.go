package middleware

import (
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

// Recovery превращает panic обработчика в HTTP-ошибку.
func Recovery() echo.MiddlewareFunc {
	return echoMiddleware.Recover()
}
