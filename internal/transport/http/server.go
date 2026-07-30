package transporthttp

import (
	"context"
	"errors"
	"fmt"
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
	if cfg == nil {
		return nil, fmt.Errorf("http server config is nil")
	}
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(securityHeaders)
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
			Addr:              cfg.App.Listen,
			Handler:           router,
			ReadHeaderTimeout: config.HTTPReadHeaderTimeout,
			ReadTimeout:       config.HTTPReadTimeout,
			WriteTimeout:      config.HTTPWriteTimeout,
			IdleTimeout:       config.HTTPIdleTimeout,
			MaxHeaderBytes:    config.HTTPMaxHeaderBytes,
		},
	}

	if err := s.registerRoutes(); err != nil {
		return nil, err
	}
	return s, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; connect-src 'self'; font-src 'self' data:; form-action 'self'; frame-ancestors 'none'; img-src 'self' data: blob: http: https:; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		h.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
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

	pageHandler, err := Register(s.router, s.cfg, s.deps.Stores.System, s.deps.Stores.Node, s.deps.Theme)
	if err != nil {
		return err
	}
	s.router.NotFound(httpmiddleware.NotFoundHandler(pageHandler))
	return nil
}

func (s *HTTPServer) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("http server context is nil")
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
		if errors.Is(shutdownErr, http.ErrServerClosed) {
			shutdownErr = nil
		}
		var closeErr error
		if shutdownErr != nil {
			closeErr = s.server.Close()
			if errors.Is(closeErr, http.ErrServerClosed) {
				closeErr = nil
			}
		}
		return errors.Join(<-errCh, shutdownErr, closeErr)
	}
}
