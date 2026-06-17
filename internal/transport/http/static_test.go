package transporthttp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dash/internal/config"
	"dash/internal/model"
	"dash/internal/store/frontcache"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/appdir"
	kitstatic "github.com/Ithildur/EiluneKit/http/static"
	"github.com/go-chi/chi/v5"
)

func TestDeployAssetRequiresNodeSecret(t *testing.T) {
	root := t.TempDir()
	deployDir := filepath.Join(root, "deploy", "linux")
	if err := os.MkdirAll(deployDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	asset := filepath.Join(deployDir, "node_linux_amd64")
	if err := os.WriteFile(asset, []byte("node asset"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("DASH_HOME", root)

	cfg := &config.Config{
		App: config.AppConfig{
			PublicURLScheme: "https",
			PublicURLHost:   "dash.example.com",
		},
	}
	st := nodestore.New(nil, nil, frontcache.New(nil, nil))
	if err := st.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: "node-secret"}); err != nil {
		t.Fatalf("SyncServerCache() error = %v", err)
	}

	router := chi.NewRouter()
	registerInstallScriptRoutes(router, cfg)
	opts := kitstatic.Options{
		AppDir: appdir.Options{
			EnvVar:            "DASH_HOME",
			Markers:           []string{"deploy"},
			RequireDirMarkers: true,
			Sources:           appdir.SourceEnvVar,
		},
	}
	if err := mountDeployRoute(router, st, opts); err != nil {
		t.Fatalf("mountDeployRoute() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/deploy/linux/install.sh", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("install script status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(request.NodeSecretHeader)) {
		t.Fatalf("install script does not send %s", request.NodeSecretHeader)
	}

	req = httptest.NewRequest(http.MethodGet, "/deploy/linux/node_linux_amd64", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing secret status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/deploy/linux/node_linux_amd64", nil)
	req.Header.Set(request.NodeSecretHeader, "node-secret")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("header secret status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "node asset" {
		t.Fatalf("header secret body = %q", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/deploy/linux/node_linux_amd64?key=node-secret", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("query key status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	token, err := st.GrantDeployAccess("/deploy/linux/node_linux_amd64")
	if err != nil {
		t.Fatalf("GrantDeployAccess() error = %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/deploy/linux/node_linux_amd64?"+request.DeployGrantQuery+"="+token, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("grant token status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "node asset" {
		t.Fatalf("grant token body = %q", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/deploy/linux/node_linux_arm64?"+request.DeployGrantQuery+"="+token, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("grant token wrong path status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestInstallScriptsSendNodeSecretHeader(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			PublicURLScheme: "https",
			PublicURLHost:   "dash.example.com",
		},
	}
	for _, platform := range []string{"linux", "macos", "windows"} {
		t.Run(platform, func(t *testing.T) {
			script, err := renderInstallScript(cfg, platform)
			if err != nil {
				t.Fatalf("renderInstallScript() error = %v", err)
			}
			if !bytes.Contains(script, []byte(request.NodeSecretHeader)) {
				t.Fatalf("rendered script does not send %s", request.NodeSecretHeader)
			}
		})
	}
}
