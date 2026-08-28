package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const userIDKey = "gophprofile.user_id"

// RequireUserID проверяет наличие идентификатора пользователя в запросе.
func RequireUserID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := strings.TrimSpace(c.Request().Header.Get("X-User-ID"))
		if userID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "X-User-ID header required")
		}
		c.Set(userIDKey, userID)
		return next(c)
	}
}

// UserID возвращает идентификатор пользователя из контекста.
func UserID(c echo.Context) string {
	value, _ := c.Get(userIDKey).(string)
	return value
}
