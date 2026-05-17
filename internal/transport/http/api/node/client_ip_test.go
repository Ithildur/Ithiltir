package node

import (
	"net/http/httptest"
	"testing"
)

func TestNodeClientIPPrefersXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/node/metrics", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 203.0.113.10")

	ip, ok := nodeClientIP(req)
	if !ok {
		t.Fatal("nodeClientIP() did not resolve an IP")
	}
	if got := ip.String(); got != "198.51.100.7" {
		t.Fatalf("nodeClientIP() = %q, want %q", got, "198.51.100.7")
	}
}

func TestNodeClientIPWithoutXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/node/metrics", nil)
	req.RemoteAddr = "203.0.113.10:1234"

	ip, ok := nodeClientIP(req)
	if !ok {
		t.Fatal("nodeClientIP() did not resolve an IP")
	}
	if got := ip.String(); got != "203.0.113.10" {
		t.Fatalf("nodeClientIP() = %q, want %q", got, "203.0.113.10")
	}
}

func TestNodeClientIPWithoutXForwardedForAcceptsBareRemoteIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/node/metrics", nil)
	req.RemoteAddr = "203.0.113.10"

	ip, ok := nodeClientIP(req)
	if !ok {
		t.Fatal("nodeClientIP() did not resolve an IP")
	}
	if got := ip.String(); got != "203.0.113.10" {
		t.Fatalf("nodeClientIP() = %q, want %q", got, "203.0.113.10")
	}
}
