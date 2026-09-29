package node

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/Ithildur/EiluneKit/http/middleware"
	kitlog "github.com/Ithildur/EiluneKit/logging"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/model"
	"dash/internal/nodeingest"
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

func (h *handler) authenticate(ctx context.Context, r *http.Request) (string, model.Server, error) {
	secret := r.Header.Get(request.NodeSecretHeader)
	server, err := h.ingest.Authenticate(ctx, secret)
	return secret, server, err
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, logger *kitlog.Helper, err error) {
	if failure, ok := errors.AsType[*nodeingest.Error](err); ok {
		switch failure.Code {
		case "unauthorized":
			err = httperr.Unauthorized(failure.Cause)
		case "invalid_metrics":
			err = httperr.InvalidMetrics(failure.Cause)
		case "invalid_static_payload":
			err = httperr.InvalidStaticPayload(failure.Cause)
		case "service_unavailable":
			err = httperr.ServiceUnavailable(failure.Cause)
		}
	}
	var httpErr *httperr.Error
	if errors.As(err, &httpErr) && httpErr.Status == http.StatusUnauthorized {
		h.failedAuth.ServeHTTP(w, r)
		return
	}
	httperr.WriteOrInternal(r.Context(), w, logger, err)
}
