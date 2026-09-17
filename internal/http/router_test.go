package httpserver

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"dash/internal/config"
	"dash/internal/dashupdate"
	"dash/internal/model"
	"dash/internal/store"
	pgtest "dash/internal/testutil/postgres"
	"dash/internal/theme"
	"dash/internal/traffic"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	authstore "github.com/Ithildur/EiluneKit/auth/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func newTestHandler(t *testing.T, st *store.Stores) http.Handler {
	t.Helper()
	home := t.TempDir()
	for _, dir := range []string{"dist", "dist/assets", "deploy"} {
		if err := os.Mkdir(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"index.html":         "<!doctype html><title>dashboard</title>",
		"theme-bootstrap.js": "window.theme = {};",
		"assets/a.js":        "asset",
	} {
		if err := os.WriteFile(filepath.Join(home, "dist", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DASH_HOME", home)
	themes, err := theme.NewStore(filepath.Join(home, "themes"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := authjwt.New(strings.Repeat("k", 32), authstore.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		st = store.New(nil, nil, time.UTC, nil)
	}
	if err := st.Node.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: "node-secret"}); err != nil {
		t.Fatal(err)
	}
	srv, err := NewHTTPServer(&config.Config{
		App: config.AppConfig{
			Env: "production", PublicURLScheme: "https", PublicURLHost: "dash.example.com",
		},
		Auth: config.AuthConfig{Password: "test-password"},
	}, Dependencies{
		Stores: st, Auth: manager, Theme: themes,
		TrafficRebuild: &traffic.RebuildRunner{}, DashUpdate: &dashupdate.Runner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv.server.Handler
}

func TestHTTPRoutes(t *testing.T) {
	handler := newTestHandler(t, nil)
	for _, endpoint := range []struct {
		method string
		path   string
		status int
		code   string
		allow  []string
	}{
		{"GET", "/api/version", 200, "", nil},
		{"GET", "/api/version/", 200, "", nil},
		{"GET", "/api", 404, "not_found", nil},
		{"GET", "/api/missing", 404, "not_found", nil},
		{"POST", "/api/missing", 404, "not_found", nil},
		{"BREW", "/api/version", 501, "not_implemented", nil},
		{"BREW", "/api/missing", 501, "not_implemented", nil},
		{"get", "/api/version", 501, "not_implemented", nil},
		{"TRACE", "/api/version", 501, "not_implemented", nil},
		{"CONNECT", "example.com:443", 501, "", nil},
		{"POST", "/api/version", 405, "method_not_allowed", []string{"GET"}},
		{"HEAD", "/api/version", 405, "method_not_allowed", []string{"GET"}},
		{"OPTIONS", "/api/admin/groups", 501, "not_implemented", nil},
		{"GET", "/api/admin/groups", 401, "unauthorized", nil},
		{"GET", "/api/admin/groups/", 401, "unauthorized", nil},
		{"GET", "/api/auth/sessions", 401, "unauthorized", nil},
		{"PATCH", "/api/admin/nodes/1", 401, "unauthorized", nil},
		{"PATCH", "/api/admin/nodes/1/", 401, "unauthorized", nil},
		{"DELETE", "/api/admin/nodes/1", 401, "unauthorized", nil},
		{"DELETE", "/api/admin/nodes/1/", 401, "unauthorized", nil},
		{"POST", "/api/admin/nodes/1/upgrade", 401, "unauthorized", nil},
		{"GET", "/api/admin/nodes/traffic/rebuild", 401, "unauthorized", nil},
		{"POST", "/api/admin/nodes/1/traffic/rebuild", 401, "unauthorized", nil},
		{"GET", "/theme/preview/missing.png", 404, "", nil},
		{"POST", "/theme/active.css", 405, "", []string{"GET", "HEAD"}},
		{"GET", "/deploy/linux/install.sh", 200, "", nil},
		{"HEAD", "/deploy/linux/install.sh", 200, "", nil},
		{"GET", "/deploy/macos/install.sh", 200, "", nil},
		{"GET", "/deploy/windows/install.ps1", 200, "", nil},
		{"POST", "/deploy/linux/install.sh", 405, "", []string{"GET", "HEAD"}},
		{"POST", "/deploy/macos/install.sh", 405, "", []string{"GET", "HEAD"}},
		{"POST", "/deploy/windows/install.ps1", 405, "", []string{"GET", "HEAD"}},
		{"GET", "/deploy/linux/node_linux_amd64", 401, "", nil},
		{"BREW", "/deploy/linux/install.sh", 501, "", nil},
		{"BREW", "/deploy/linux/node_linux_amd64", 501, "", nil},
		{"GET", "/assets/a.js", 200, "", nil},
		{"OPTIONS", "/assets/a.js", 501, "", nil},
		{"BREW", "/page", 501, "", nil},
		{"GET", "/apix", 200, "", nil},
		{"GET", "/page", 200, "", nil},
		{"GET", "/theme/missing", 200, "", nil},
	} {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(endpoint.method, endpoint.path, nil))
			if w.Code != endpoint.status {
				t.Fatalf("status %d, want %d; body %s", w.Code, endpoint.status, w.Body)
			}
			if !slices.Equal(w.Header().Values("Allow"), endpoint.allow) {
				t.Errorf("Allow = %v, want %v", w.Header().Values("Allow"), endpoint.allow)
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing security headers")
			}
			if strings.HasPrefix(endpoint.path, "/api/") || endpoint.path == "/api" {
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Error("missing API cache policy")
				}
			}
			if endpoint.code != "" {
				var failure struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil || failure.Code != endpoint.code {
					t.Fatalf("error body %s, want code %s; error %v", w.Body, endpoint.code, err)
				}
			}
			if endpoint.status == http.StatusOK && (endpoint.path == "/apix" || endpoint.path == "/page" || endpoint.path == "/theme/missing") {
				if !strings.Contains(w.Body.String(), "<title>dashboard</title>") || w.Header().Get("Cache-Control") != "" {
					t.Fatalf("page fallback = %v %s", w.Header(), w.Body)
				}
			}
		})
	}

	server := httptest.NewServer(handler)
	defer server.Close()
	for _, method := range []string{"BREW", "get"} {
		r, err := http.NewRequestWithContext(t.Context(), method, server.URL+"/assets/a.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil || closeErr != nil || resp.StatusCode != http.StatusNotImplemented || string(body) != "Not Implemented\n" || resp.Header.Get("Allow") != "" {
			t.Fatalf("%s file request: status %d, headers %v, body %q, read %v, close %v", method, resp.StatusCode, resp.Header, body, readErr, closeErr)
		}
	}
}

func TestHTTPAPIRequests(t *testing.T) {
	handler := newTestHandler(t, nil)
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"test-password","persistence":"session"}`))
	login.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, login)
	var session struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil || w.Code != 200 || session.AccessToken == "" {
		t.Fatalf("login = %d %s; error %v", w.Code, w.Body, err)
	}
	cookies := w.Result().Cookies()
	refresh := slices.IndexFunc(cookies, func(cookie *http.Cookie) bool { return cookie.HttpOnly })
	if refresh < 0 || cookies[refresh].Path != "/api/auth" || cookies[refresh].SameSite != http.SameSiteStrictMode {
		t.Fatal("refresh cookie scope or SameSite changed")
	}
	r := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	r.Header.Set("Authorization", "Bearer "+session.AccessToken)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("authenticated sessions = %d %s", w.Code, w.Body)
	}
	for _, path := range []string{"/api/admin/nodes/bad", "/api/admin/nodes/bad/"} {
		for _, method := range []string{http.MethodPatch, http.MethodDelete} {
			r := httptest.NewRequest(method, path, nil)
			r.Header.Set("Authorization", "Bearer "+session.AccessToken)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"invalid_id"`) {
				t.Fatalf("dynamic path %s %s = %d %s", method, path, w.Code, w.Body)
			}
		}
	}
	for _, path := range []string{"/api/node/identity", "/api/node/metrics", "/api/node/static"} {
		for _, secret := range []string{"", "invalid-secret"} {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Node-Secret", secret)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s with secret %q = %d %s", path, secret, w.Code, w.Body)
			}
		}
	}
	for _, contentType := range []struct {
		value  string
		status int
	}{
		{"application/json; charset=", 415},
		{"application/json ; charset=utf-8", 200},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/node/identity", strings.NewReader(`{}`))
		r.Header.Set("Content-Type", contentType.value)
		r.Header.Set("X-Node-Secret", "node-secret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != contentType.status {
			t.Fatalf("Content-Type %q = %d %s, want %d", contentType.value, w.Code, w.Body, contentType.status)
		}
	}
	for _, endpoint := range []struct {
		path   string
		status int
	}{
		{"/api/admin/groups", 413},
		{"/api/missing", 404},
		{"/api/version", 405},
	} {
		r := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(strings.Repeat(" ", int(maxAPIRequestBytes)+1)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != endpoint.status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("oversized POST %s = %d %v %s, want %d", endpoint.path, w.Code, w.Header(), w.Body, endpoint.status)
		}
	}
	t.Run("metrics persistence deadline", func(t *testing.T) {
		db := pgtest.NewDB(t)
		if err := db.Exec("INSERT INTO servers(id,name,hostname,secret) VALUES (1,'node','node','node-secret')").Error; err != nil {
			t.Fatal(err)
		}
		st := store.New(db, nil, time.UTC, pgtest.ConfigCipher(t))
		handler := newTestHandler(t, st)
		payload := `{"version":"0.2.4","hostname":"node","timestamp":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","metrics":{"disk":{"physical":[],"logical":[],"filesystems":[],"base_io":[]},"network":[],"raid":{"arrays":[]},"system":{"uptime":"1s","uptime_seconds":1}}}`
		for _, delayed := range []bool{true, false} {
			var body io.Reader = strings.NewReader(payload)
			if delayed {
				reader, writer := io.Pipe()
				t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
				body = reader
				go func() {
					// The first byte is consumed only after the handler has begun.
					_, err := io.WriteString(writer, payload[:1])
					if err == nil {
						timer := time.NewTimer(config.PGWriteTimeout + 100*time.Millisecond)
						defer timer.Stop()
						select {
						case <-timer.C:
							_, err = io.WriteString(writer, payload[1:])
						case <-t.Context().Done():
							err = t.Context().Err()
						}
					}
					_ = writer.CloseWithError(err)
				}()
			}
			r := httptest.NewRequest(http.MethodPost, "/api/node/metrics", body).WithContext(t.Context())
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Node-Secret", "node-secret")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			status, rows := http.StatusOK, int64(1)
			if delayed {
				status, rows = http.StatusServiceUnavailable, 0
			}
			if w.Code != status {
				t.Fatalf("metrics delayed=%v: %d %s, want %d", delayed, w.Code, w.Body, status)
			}
			for _, table := range []string{"server_metrics", "server_current_metrics"} {
				var count int64
				if err := db.Table(table).Count(&count).Error; err != nil || count != rows {
					t.Fatalf("%s delayed=%v: count %d, error %v, want %d", table, delayed, count, err, rows)
				}
			}
		}
	})
}

func TestHTTPRecovery(t *testing.T) {
	for _, started := range []bool{false, true} {
		name := "before headers"
		if started {
			name = "after headers"
		}
		t.Run(name, func(t *testing.T) {
			root := routes.NewBlueprint()
			root.Get("/api/panic", "", func(w http.ResponseWriter, _ *http.Request) {
				if started {
					_, _ = w.Write([]byte("partial"))
				}
				panic("request failed")
			})
			routeList := root.Routes()
			handler, err := routes.NewHandler(routeList, handlerOptions(&config.Config{}, fallback{
				page: http.NotFoundHandler(), deploy: http.NotFoundHandler(),
			}, routeList))
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			var aborted any
			func() {
				defer func() { aborted = recover() }()
				handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/panic", nil))
			}()
			if started {
				if aborted != http.ErrAbortHandler || w.Code != 200 || w.Body.String() != "partial" {
					t.Fatalf("started response: panic %v, status %d, body %s", aborted, w.Code, w.Body)
				}
			} else if aborted != nil || w.Code != 500 || w.Body.Len() != 0 {
				t.Fatalf("unstarted response: panic %v, status %d, body %s", aborted, w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Error("panic response lost API cache policy")
			}
		})
	}
}
