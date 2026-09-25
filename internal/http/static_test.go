package httpserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"dash/internal/config"
	"dash/internal/http/deploy"
	"dash/internal/http/request"
	"dash/internal/model"
	"dash/internal/store/frontcache"
	"dash/internal/store/frontprojection"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/appdir"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitstatic "github.com/Ithildur/EiluneKit/http/static"
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
	projection := frontprojection.New()
	st := nodestore.New(nil, frontcache.New(nil, nil, projection), projection, time.Local)
	if err := st.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: "node-secret"}); err != nil {
		t.Fatalf("SyncServerCache() error = %v", err)
	}

	rootRoutes := routes.NewBlueprint()
	rootRoutes.Include("/deploy", deploy.Router(
		installScriptHandler(cfg, "linux", "text/x-shellscript; charset=utf-8"),
		installScriptHandler(cfg, "macos", "text/x-shellscript; charset=utf-8"),
		installScriptHandler(cfg, "windows", "text/plain; charset=utf-8"),
	))
	opts := kitstatic.Options{
		AppDir: appdir.Options{
			EnvVar:            "DASH_HOME",
			Markers:           []string{"deploy"},
			RequireDirMarkers: true,
			Sources:           appdir.SourceEnvVar,
		},
	}
	download, err := deployHandler(st, opts)
	if err != nil {
		t.Fatal(err)
	}
	routeList := rootRoutes.Routes()
	router, err := routes.NewHandler(routeList, handlerOptions(cfg, fallback{
		page: http.NotFoundHandler(), deploy: download,
	}, routeList))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/deploy/linux/install.sh", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("install script status = %d, want %d", rr.Code, http.StatusOK)
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

	server := httptest.NewServer(router)
	defer server.Close()
	for _, transfer := range []struct {
		method      string
		rangeHeader string
		status      int
		body        string
	}{
		{http.MethodGet, "", 200, "node asset"},
		{http.MethodHead, "", 200, ""},
		{http.MethodGet, "bytes=0-3", 206, "node"},
	} {
		req, err := http.NewRequestWithContext(t.Context(), transfer.method, server.URL+"/deploy/linux/node_linux_amd64", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(request.NodeSecretHeader, "node-secret")
		if transfer.rangeHeader != "" {
			req.Header.Set("Range", transfer.rangeHeader)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil || closeErr != nil || resp.StatusCode != transfer.status || string(body) != transfer.body {
			t.Fatalf("%s %s: status %d, body %q, read %v, close %v", transfer.method, transfer.rangeHeader, resp.StatusCode, body, readErr, closeErr)
		}
		if resp.Header.Get("Cache-Control") != "private, no-store" || resp.Header.Get("Vary") != request.NodeSecretHeader {
			t.Fatalf("download cache policy = %v", resp.Header)
		}
		if transfer.rangeHeader != "" && resp.Header.Get("Content-Range") != "bytes 0-3/10" {
			t.Fatalf("Content-Range = %q", resp.Header.Get("Content-Range"))
		}
	}
}

func TestThemeBootstrapIsNotCached(t *testing.T) {
	handler := noStoreThemeBootstrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tt := range []struct {
		path string
		want string
	}{
		{path: "/theme-bootstrap.js", want: "no-store"},
		{path: "/assets/index-abc123.js", want: ""},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if got := rr.Header().Get("Cache-Control"); got != tt.want {
			t.Fatalf("%s Cache-Control = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestLinuxInstallDownloadsSendNodeSecret(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("installer download test requires bash")
	}
	script, err := renderInstallScript(&config.Config{}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	// Load the real download functions without running the system installer.
	functions, ok := bytes.CutSuffix(bytes.TrimSpace(script), []byte(`main "$@"`))
	if !ok {
		t.Fatal("installer entry point changed")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(request.NodeSecretHeader) != "download-secret" {
			http.Error(w, "invalid node secret", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "node asset")
	}))
	defer server.Close()

	for _, client := range []string{"curl", "wget"} {
		t.Run(client, func(t *testing.T) {
			if _, err := exec.LookPath(client); err != nil {
				t.Skipf("installer download test requires %s", client)
			}
			asset := filepath.Join(t.TempDir(), "node")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bash, "-c", string(functions)+"\n\"$1\" \"$2\" \"$3\" \"$4\"\n",
				"install-download", "download_with_"+client, server.URL, asset, "download-secret")
			cmd.WaitDelay = time.Second
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("installer download: %v\n%s", err, output)
			}
			body, err := os.ReadFile(asset)
			if err != nil || string(body) != "node asset" {
				t.Fatalf("downloaded asset = %q, error %v", body, err)
			}
		})
	}
}

func TestInstallScriptsSendNodeSecretHeader(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			PublicURLScheme: "https",
			PublicURLHost:   "dash.example.com",
		},
	}
	for _, platform := range []string{"macos", "windows"} {
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

func TestInstallScriptsUseConfiguredLanguage(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		language string
		want     string
	}{
		{
			name:     "linux chinese",
			platform: "linux",
			language: config.LanguageChinese,
			want:     "连接数缓存 helper 已安装",
		},
		{
			name:     "linux english",
			platform: "linux",
			language: config.LanguageEnglish,
			want:     "Connections cache helper installed",
		},
		{
			name:     "macos chinese",
			platform: "macos",
			language: config.LanguageChinese,
			want:     "网络时间同步已启用",
		},
		{
			name:     "macos english",
			platform: "macos",
			language: config.LanguageEnglish,
			want:     "network time sync is enabled",
		},
		{
			name:     "windows chinese",
			platform: "windows",
			language: config.LanguageChinese,
			want:     "Windows 时间服务已启用",
		},
		{
			name:     "windows english",
			platform: "windows",
			language: config.LanguageEnglish,
			want:     "Windows time service is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				App: config.AppConfig{
					PublicURLScheme: "https",
					PublicURLHost:   "dash.example.com",
					Language:        tt.language,
				},
			}
			script, err := renderInstallScript(cfg, tt.platform)
			if err != nil {
				t.Fatalf("renderInstallScript() error = %v", err)
			}
			if bytes.Contains(script, []byte(appLanguageToken)) {
				t.Fatalf("rendered script contains language token")
			}
			wantLanguage := `APP_LANGUAGE="${APP_LANGUAGE:-` + tt.language + `}"`
			if tt.platform == "windows" {
				wantLanguage = `$APP_LANGUAGE = if ($env:APP_LANGUAGE) { $env:APP_LANGUAGE } else { "` + tt.language + `" }`
			}
			if !bytes.Contains(script, []byte(wantLanguage)) {
				t.Fatalf("rendered script missing language default %q", wantLanguage)
			}
			if !bytes.Contains(script, []byte(tt.want)) {
				t.Fatalf("rendered script missing %q", tt.want)
			}
		})
	}
}

func TestShellInstallScriptsUseBash32Syntax(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			PublicURLScheme: "https",
			PublicURLHost:   "dash.example.com",
			Language:        config.LanguageEnglish,
		},
	}
	bash4CaseExpansion := regexp.MustCompile(`\$\{[^}\n]*(,,|\^\^)[^}\n]*\}`)

	for _, platform := range []string{"linux", "macos"} {
		t.Run(platform, func(t *testing.T) {
			script, err := renderInstallScript(cfg, platform)
			if err != nil {
				t.Fatalf("renderInstallScript() error = %v", err)
			}
			if match := bash4CaseExpansion.Find(script); match != nil {
				t.Fatalf("%s install script uses Bash 4 case expansion %q", platform, match)
			}
		})
	}
}
