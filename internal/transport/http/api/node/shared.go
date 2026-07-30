package node

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"dash/internal/config"
	"dash/internal/model"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"gorm.io/gorm"
)

// This handler is entered only after authentication fails. Valid node requests
// never consume its subnet-level budget.
func failedAuthHandler(trustedProxies []netip.Prefix) http.Handler {
	unauthorized := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httperr.Write(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
	})
	return middleware.RateLimit(middleware.RateLimitOptions{
		Requests: config.NodeRateLimitRequests,
		Window:   config.NodeRateLimitWindow,
		KeyFunc: middleware.RateLimitKeyByIP(24, 40, middleware.RateLimitKeyOptions{
			TrustedProxies: append([]netip.Prefix(nil), trustedProxies...),
		}),
	})(unauthorized)
}

func (h *handler) authenticate(ctx context.Context, r *http.Request, logger *kitlog.Helper) (string, model.Server, error) {
	secret := r.Header.Get(request.NodeSecretHeader)
	if secret == "" {
		return "", model.Server{}, httperr.Unauthorized(nil)
	}
	server, err := h.serverBySecret(ctx, secret, logger)
	return secret, server, err
}

func (h *handler) serverBySecret(ctx context.Context, secret string, logger *kitlog.Helper) (model.Server, error) {
	server, err := h.node.GetServerBySecret(ctx, secret)
	if err == nil {
		return server, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Server{}, httperr.Unauthorized(err)
	}
	logger.Error("node auth lookup failed", err)
	return model.Server{}, httperr.ServiceUnavailable(err)
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, logger *kitlog.Helper, err error) {
	var httpErr *httperr.Error
	if errors.As(err, &httpErr) && httpErr.Status == http.StatusUnauthorized {
		h.failedAuth.ServeHTTP(w, r)
		return
	}
	httperr.WriteOrInternal(w, logger, err)
}
