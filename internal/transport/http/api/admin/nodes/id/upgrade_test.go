package nodeid

import (
	"net/url"
	"testing"
	"time"

	"dash/internal/store/frontcache"
	"dash/internal/store/frontprojection"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/request"
)

func TestLegacyUpdateURLAddsDeployGrant(t *testing.T) {
	projection := frontprojection.New()
	h := &handler{store: nodestore.New(nil, frontcache.New(nil, nil, projection), projection, time.Local)}
	got, err := h.legacyUpdateURL("https://dash.example.com/deploy/linux/node_linux_amd64?mirror=local")
	if err != nil {
		t.Fatalf("legacyUpdateURL() error = %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", got, err)
	}
	if parsed.Scheme != "https" || parsed.Host != "dash.example.com" || parsed.Path != "/deploy/linux/node_linux_amd64" {
		t.Fatalf("legacyUpdateURL() changed base URL: %s", got)
	}
	if parsed.Query().Get("mirror") != "local" {
		t.Fatalf("legacyUpdateURL() dropped existing query: %s", got)
	}
	token := parsed.Query().Get(request.DeployGrantQuery)
	if token == "" {
		t.Fatalf("legacyUpdateURL() missing %s", request.DeployGrantQuery)
	}
	if !h.store.ValidDeployGrant(token, parsed.Path) {
		t.Fatalf("legacyUpdateURL() token is not valid for %s", parsed.Path)
	}
}
