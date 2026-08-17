package rest

import (
	"github.com/aikowocki/yandex-go-ext/internal/config"
	restmiddleware "github.com/aikowocki/yandex-go-ext/internal/transport/rest/middleware"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

func registerRoutes(e *echo.Echo, cfg config.ServerConfig, handler *avatarHandler, checks DependencyChecks) {
	ratePerSecond := cfg.RateLimitPerSecond
	if ratePerSecond < 1 {
		ratePerSecond = 10
	}
	burst := cfg.RateLimitBurst
	if burst < 1 {
		burst = 20
	}

	e.GET("/health", func(c echo.Context) error { return healthResponse(c, checks) })
	e.File("/web/upload", "web/index.html")
	e.POST("/web/upload", restmiddleware.RequireUserID(handler.createAvatar))
	e.File("/web/gallery/:user_id", "web/gallery.html")

	api := e.Group("/api/v1")
	api.Use(echoMiddleware.RateLimiterWithConfig(echoMiddleware.RateLimiterConfig{
		Store: echoMiddleware.NewRateLimiterMemoryStoreWithConfig(echoMiddleware.RateLimiterMemoryStoreConfig{
			Rate:  rate.Limit(ratePerSecond),
			Burst: burst,
		}),
	}))
	api.POST("/avatars", restmiddleware.RequireUserID(handler.createAvatar))
	api.PATCH("/avatars/:id/crop", restmiddleware.RequireUserID(handler.updateAvatarCrop))
	api.GET("/avatars/:id/metadata", handler.metadata)
	api.GET("/avatars/:id", handler.downloadAvatar)
	api.DELETE("/avatars/:id", restmiddleware.RequireUserID(handler.deleteAvatar))
	api.GET("/users/:user_id/avatar", handler.downloadUserAvatar)
	api.DELETE("/users/:user_id/avatar", restmiddleware.RequireUserID(handler.deleteUserAvatar))
	api.GET("/users/:user_id/avatars", handler.listUserAvatars)
}
