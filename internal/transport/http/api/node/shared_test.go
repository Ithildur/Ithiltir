package node

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dash/internal/config"
	"dash/internal/transport/http/httperr"
)

func TestFailedAuthLimitCountsOnlyUnauthorized(t *testing.T) {
	h := handler{failedAuth: failedAuthHandler(nil)}
	serveError := func(err error, remoteAddr string) int {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/api/node/metrics", nil)
		r.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		h.writeError(w, r, nil, err)
		return w.Code
	}

	for range config.NodeRateLimitRequests {
		if status := serveError(httperr.Unauthorized(nil), "192.0.2.1:1234"); status != http.StatusUnauthorized {
			t.Fatalf("auth failure status = %d, want %d", status, http.StatusUnauthorized)
		}
	}
	if status := serveError(httperr.InvalidRequest(nil), "192.0.2.1:1234"); status != http.StatusBadRequest {
		t.Fatalf("authenticated request error status = %d, want %d", status, http.StatusBadRequest)
	}
	if status := serveError(httperr.Unauthorized(nil), "192.0.2.1:1234"); status != http.StatusTooManyRequests {
		t.Fatalf("limited auth failure status = %d, want %d", status, http.StatusTooManyRequests)
	}
	if status := serveError(httperr.Unauthorized(nil), "192.0.3.1:1234"); status != http.StatusUnauthorized {
		t.Fatalf("other subnet auth failure status = %d, want %d", status, http.StatusUnauthorized)
	}
}
