package transporthttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	httpapi "dash/internal/transport/http/api"
	"github.com/Ithildur/EiluneKit/clientip"
	httpmiddleware "github.com/Ithildur/EiluneKit/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type HTTPServer struct {
	cfg    *config.Config
	deps   httpapi.Dependencies
	router chi.Router
	server *http.Server
}

func NewHTTPServer(cfg *config.Config, deps httpapi.Dependencies) (*HTTPServer, error) {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(httpmiddleware.AccessLog(httpmiddleware.AccessLogOptions{
		Disabled: !infra.DebugEnabled(),
		Logger:   infra.SlogWithModule("http"),
		MinLevel: slog.LevelDebug,
		ClientIP: clientip.Options{
			TrustedProxies: append([]netip.Prefix(nil), cfg.HTTP.TrustedProxyPrefixes...),
		},
		Skip: func(r *http.Request, status int) bool {
			return isProductionEnv(cfg.App.Env) && status == http.StatusNotFound
		},
	}))
	router.Use(middleware.Recoverer)

	s := &HTTPServer{
		cfg:    cfg,
		deps:   deps,
		router: router,
		server: &http.Server{
			Addr:         cfg.App.Listen,
			Handler:      router,
			ReadTimeout:  config.HTTPReadTimeout,
			WriteTimeout: config.HTTPWriteTimeout,
			IdleTimeout:  config.HTTPIdleTimeout,
		},
	}

	if err := s.registerRoutes(); err != nil {
		return nil, err
	}
	return s, nil
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func (s *HTTPServer) registerRoutes() error {
	if err := httpapi.Register(s.router, s.cfg, s.deps); err != nil {
		return err
	}

	pageHandler, err := Register(s.router, s.cfg, s.deps.Stores.System, s.deps.Theme)
	if err != nil {
		return err
	}
	s.router.NotFound(httpmiddleware.NotFoundHandler(pageHandler))
	return nil
}

func (s *HTTPServer) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	errCh := make(chan error, 1)
	go func() {
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := s.server.Shutdown(shutdownCtx)
		serveErr := <-errCh
		if shutdownErr != nil && !errors.Is(shutdownErr, http.ErrServerClosed) {
			if serveErr != nil {
				return errors.Join(serveErr, shutdownErr)
			}
			return shutdownErr
		}
		return serveErr
	}
}
