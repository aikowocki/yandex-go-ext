package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os/signal"
	"syscall"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"golang.org/x/sync/errgroup"
)

const shutdownTimeout = 10 * time.Second

// Run запускает сервер и фоновые компоненты приложения.
func (c *Container) Run() error {
	if c == nil || c.Server == nil || c.Config == nil {
		return errors.New("application is not initialized")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		logging.Info(groupCtx, "starting HTTP server", logging.String("addr", fmt.Sprintf("%s:%d", c.Config.Server.Host, c.Config.Server.Port)))
		return c.Server.Run(groupCtx)
	})

	pprofServer := newPprofServer(c.Config.Server.PprofAddress)
	if pprofServer != nil {
		group.Go(func() error {
			logging.Info(groupCtx, "starting pprof server", logging.String("addr", c.Config.Server.PprofAddress))
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logging.Warn(groupCtx, "pprof server stopped", logging.Err(err))
			}
			return nil
		})
	}

	<-groupCtx.Done()
	logging.Info(groupCtx, "shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	c.shutdown(shutdownCtx, pprofServer)

	return group.Wait()
}

func (c *Container) shutdown(ctx context.Context, pprofServer *http.Server) {
	if pprofServer != nil {
		_ = pprofServer.Shutdown(ctx)
	}
	_ = c.Close()
	logging.Info(ctx, "shutdown complete")
}

// Close корректно останавливает компоненты приложения.
func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		if c.Broker != nil {
			if err := c.Broker.Close(); err != nil {
				c.closeErr = errors.Join(c.closeErr, err)
			}
		}
		if c.DB != nil {
			c.DB.Close()
		}
		if c.logger != nil {
			if err := c.logger.Sync(); err != nil {
				c.closeErr = errors.Join(c.closeErr, err)
			}
		}
		if c.restoreLogger != nil {
			c.restoreLogger()
		}
	})
	return c.closeErr
}

func newPprofServer(addr string) *http.Server {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
