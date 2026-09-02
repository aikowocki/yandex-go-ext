package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	restmiddleware "github.com/aikowocki/yandex-go-ext/internal/transport/rest/middleware"
	avatarusecase "github.com/aikowocki/yandex-go-ext/internal/usecase/avatar"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

// Server обслуживает HTTP-запросы приложения.
type Server struct {
	logger logging.Logger

	echo   *echo.Echo
	http   *http.Server
	health DependencyChecks
}

// DependencyChecks содержит проверки внешних зависимостей.
type DependencyChecks struct {
	Database func(context.Context) error
	Storage  func(context.Context) error
	Broker   func(context.Context) error
}

// NewServer создаёт HTTP-сервер и регистрирует маршруты.
func NewServer(
	cfg config.ServerConfig,
	avatar *avatarusecase.UseCase,
	thumbnails contracts.ThumbnailRepository,
	storage contracts.ObjectStorage,
	logger logging.Logger,
	healthChecks ...DependencyChecks,
) (*Server, error) {
	if logger == nil {
		logger = logging.ComponentLogger(nil, "rest")
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(otelecho.Middleware("gophprofile-server", otelecho.WithSkipper(func(c echo.Context) bool {
		return c.Path() == "/health"
	})))
	e.Use(metricsMiddleware())
	e.Use(restmiddleware.RequestLogger(logger))
	e.Use(restmiddleware.Recovery())
	e.Use(restmiddleware.CORS())
	e.Use(echoMiddleware.BodyLimit(strconv.FormatInt(cfg.MaxUploadSize+1024*1024, 10)))
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			message := httpErr.Message
			if message == nil {
				message = http.StatusText(httpErr.Code)
			}
			_ = c.JSON(httpErr.Code, map[string]any{"error": message})
			return
		}
		_ = respondError(c, err)
	}

	checks := DependencyChecks{}
	if len(healthChecks) > 0 {
		checks = healthChecks[0]
	}

	handler := &avatarHandler{avatar: avatar, thumbnails: thumbnails, storage: storage}
	registerRoutes(e, cfg, handler, checks)

	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return &Server{logger: logger, echo: e, http: &http.Server{Addr: address, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, Handler: e}, health: checks}, nil
}

func healthResponse(c echo.Context, checks DependencyChecks) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	components := map[string]string{}
	healthy := true
	for name, check := range map[string]func(context.Context) error{
		"database": checks.Database,
		"storage":  checks.Storage,
		"broker":   checks.Broker,
	} {
		if check == nil {
			components[name] = "not_configured"
			healthy = false
			continue
		}
		if err := check(ctx); err != nil {
			components[name] = "unhealthy"
			healthy = false
			continue
		}
		components[name] = "ok"
	}

	status := "ok"
	code := http.StatusOK
	if !healthy {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}
	return c.JSON(code, map[string]any{"status": status, "components": components})
}

// Run запускает HTTP-сервер до отмены контекста.
func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.http == nil {
		return fmt.Errorf("server is not initialized")
	}
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
