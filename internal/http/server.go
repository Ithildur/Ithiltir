package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dash/internal/config"
)

type HTTPServer struct {
	server *http.Server
}

func NewHTTPServer(cfg *config.Config, deps Dependencies) (*HTTPServer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("http server config is nil")
	}
	handler, err := newHandler(cfg, deps)
	if err != nil {
		return nil, err
	}

	s := &HTTPServer{
		server: &http.Server{
			Addr:              cfg.App.Listen,
			Handler:           handler,
			ReadHeaderTimeout: config.HTTPReadHeaderTimeout,
			ReadTimeout:       config.HTTPReadTimeout,
			WriteTimeout:      config.HTTPWriteTimeout,
			IdleTimeout:       config.HTTPIdleTimeout,
			MaxHeaderBytes:    config.HTTPMaxHeaderBytes,
		},
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
