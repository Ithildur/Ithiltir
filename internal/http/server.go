package httpserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/nodeingest"
	"dash/internal/noderpc"
	"dash/internal/nodesession"
	"dash/internal/serverid"
)

type HTTPServer struct {
	server *http.Server
	rpc    *noderpc.Server
}

func NewHTTPServer(cfg *config.Config, deps Dependencies) (*HTTPServer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("http server config is nil")
	}
	if deps.Stores == nil {
		return nil, fmt.Errorf("http server store is nil")
	}
	if deps.NodeIngest == nil {
		path, err := config.InstallIDPath()
		if err != nil {
			return nil, err
		}
		st := deps.Stores
		deps.NodeIngest = nodeingest.New(st.Node, st.Metric, st.Front, st.Alert, serverid.New(path), int(math.Ceil(cfg.App.EffectiveNodeOfflineThreshold().Seconds())))
	}
	if deps.NodeSessions == nil {
		deps.NodeSessions = nodesession.New(deps.NodeIngest.Authenticate)
	}
	handler, err := newHandler(cfg, deps)
	if err != nil {
		return nil, err
	}
	rpc := noderpc.New(deps.NodeIngest, deps.NodeSessions, cfg.HTTP.TrustedProxyPrefixes)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	s := &HTTPServer{
		rpc: rpc,
		server: &http.Server{
			Addr: cfg.App.Listen,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if noderpc.IsRequest(r) {
					rpc.ServeHTTP(w, r)
					return
				}
				handler.ServeHTTP(w, r)
			}),
			Protocols:         protocols,
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
	serveDone := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = s.server.ListenAndServe()
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		close(serveDone)
	}()

	select {
	case <-serveDone:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := s.shutdown(shutdownCtx)
	<-serveDone
	return errors.Join(serveErr, shutdownErr)
}

func (s *HTTPServer) shutdown(ctx context.Context) error {
	// Drain HTTP and stop query sessions together, under the same exit budget.
	rpcDone := make(chan struct{})
	go func() {
		s.rpc.Close()
		close(rpcDone)
	}()
	shutdownErr := s.server.Shutdown(ctx)
	var closeErr error
	if shutdownErr != nil {
		closeErr = s.server.Close()
	}
	<-rpcDone
	return errors.Join(shutdownErr, closeErr)
}
