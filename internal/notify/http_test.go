package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestControlledRedirect(t *testing.T) {
	tests := []struct {
		name     string
		original string
		previous string
		target   string
		wantCode string
	}{
		{name: "same origin", original: "https://example.com/hook", previous: "https://example.com/hook", target: "https://example.com/next"},
		{name: "HTTP upgrade", original: "http://example.com/hook", previous: "http://example.com/hook", target: "https://example.com/next"},
		{name: "different host", original: "https://example.com/hook", previous: "https://example.com/hook", target: "https://other.example/next", wantCode: "redirect_host_changed"},
		{name: "same scheme port change", original: "https://example.com/hook", previous: "https://example.com/hook", target: "https://example.com:8443/next", wantCode: "redirect_port_changed"},
		{name: "HTTPS downgrade", original: "http://example.com/hook", previous: "https://example.com/next", target: "http://example.com/final", wantCode: "redirect_scheme_changed"},
		{name: "user information", original: "https://example.com/hook", previous: "https://example.com/hook", target: "https://user@example.com/next", wantCode: "redirect_userinfo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := mustURL(t, tt.original)
			previous := mustURL(t, tt.previous)
			via := []*http.Request{{Method: http.MethodPost, URL: original}}
			if tt.previous != tt.original {
				via = append(via, &http.Request{Method: http.MethodPost, URL: previous})
			}
			err := controlledRedirect(&http.Request{Method: http.MethodPost, URL: mustURL(t, tt.target)}, via)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("controlledRedirect() error = %v", err)
				}
				return
			}
			failure := Classify(err)
			if failure.Class != DeliveryBlocked || failure.Code != tt.wantCode {
				t.Fatalf("controlledRedirect() failure = %+v, want blocked/%s", failure, tt.wantCode)
			}
		})
	}
}

func TestControlledRedirectLimitsHops(t *testing.T) {
	u := mustURL(t, "https://example.com/hook")
	via := make([]*http.Request, maxHTTPRedirects+1)
	for i := range via {
		via[i] = &http.Request{Method: http.MethodPost, URL: u}
	}
	err := controlledRedirect(&http.Request{Method: http.MethodPost, URL: u}, via)
	if got := Classify(err); got.Class != DeliveryBlocked || got.Code != "redirect_limit" {
		t.Fatalf("controlledRedirect() failure = %+v, want blocked/redirect_limit", got)
	}
}

func TestControlledRedirectRejectsPOSTMethodDowngrade(t *testing.T) {
	u := mustURL(t, "https://example.com/hook")
	err := controlledRedirect(
		&http.Request{Method: http.MethodGet, URL: u},
		[]*http.Request{{Method: http.MethodPost, URL: u}},
	)
	if got := Classify(err); got.Class != DeliveryBlocked || got.Code != "redirect_method_changed" {
		t.Fatalf("controlledRedirect() failure = %+v, want blocked/redirect_method_changed", got)
	}
}

func TestHTTPClientOnlyFollowsBodyPreservingRedirects(t *testing.T) {
	for _, tt := range []struct {
		name       string
		status     int
		wantCode   string
		wantMethod string
		wantBody   string
	}{
		{name: "302 changes POST to GET", status: http.StatusFound, wantCode: "redirect_method_changed"},
		{name: "307 preserves POST body", status: http.StatusTemporaryRedirect, wantMethod: http.MethodPost, wantBody: "payload"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var finalMethod, finalBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/final", tt.status)
					return
				}
				finalMethod = r.Method
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read redirected body: %v", err)
				}
				finalBody = string(body)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			request, err := http.NewRequest(http.MethodPost, server.URL+"/start", strings.NewReader("payload"))
			if err != nil {
				t.Fatalf("http.NewRequest() error = %v", err)
			}
			response, err := (&http.Client{CheckRedirect: controlledRedirect}).Do(request)
			if response != nil {
				response.Body.Close()
			}
			if tt.wantCode != "" {
				failure := Classify(requestError(err))
				if failure.Class != DeliveryBlocked || failure.Code != tt.wantCode {
					t.Fatalf("redirect failure = %+v, want blocked/%s", failure, tt.wantCode)
				}
				if finalMethod != "" {
					t.Fatalf("redirected request reached final endpoint with method %s", finalMethod)
				}
				return
			}
			if err != nil {
				t.Fatalf("client.Do() error = %v", err)
			}
			if finalMethod != tt.wantMethod || finalBody != tt.wantBody {
				t.Fatalf("redirected request = %s %q, want %s %q", finalMethod, finalBody, tt.wantMethod, tt.wantBody)
			}
		})
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", raw, err)
	}
	return u
}
