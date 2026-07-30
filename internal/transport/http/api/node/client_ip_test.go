package node

import (
	"net/http/httptest"
	"testing"
)

func TestNodeClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{
			name:       "forwarded observation",
			remoteAddr: "203.0.113.10:1234",
			forwarded:  "198.51.100.7, 203.0.113.10",
			want:       "198.51.100.7",
		},
		{name: "remote socket", remoteAddr: "203.0.113.10:1234", want: "203.0.113.10"},
		{name: "bare remote IP", remoteAddr: "203.0.113.10", want: "203.0.113.10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/node/metrics", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}

			ip, ok := nodeClientIP(req)
			if !ok {
				t.Fatal("nodeClientIP() did not resolve an IP")
			}
			if got := ip.String(); got != tt.want {
				t.Fatalf("nodeClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
