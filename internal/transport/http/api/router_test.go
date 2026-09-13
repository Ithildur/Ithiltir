package api

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dash/internal/config"
	"dash/internal/model"
	"dash/internal/serverid"
	"dash/internal/store"
	"dash/internal/transport/http/request"
	authhttp "github.com/Ithildur/EiluneKit/auth/http"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	authstore "github.com/Ithildur/EiluneKit/auth/store"
	"github.com/go-chi/chi/v5"
)

func TestMountedAuthentication(t *testing.T) {
	st := store.New(nil, nil, time.UTC, nil)
	const secret = "node-test-secret"
	if err := st.Node.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: secret}); err != nil {
		t.Fatal(err)
	}
	manager, err := authjwt.New(strings.Repeat("k", 32), authstore.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	authHandler, err := newAuthHandler("test-password", manager, nil)
	if err != nil {
		t.Fatal(err)
	}
	bearer, err := authhttp.RequireBearer(manager)
	if err != nil {
		t.Fatal(err)
	}
	optionalBearer, err := request.OptionalBearer(manager)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	blueprint := buildRoutes(&config.Config{}, Dependencies{Stores: st}, routeSetup{
		authHandler: authHandler, bearer: bearer, optionalBearer: optionalBearer,
		serverID: serverid.New(filepath.Join(t.TempDir(), "install-id")),
	})
	if err := blueprint.Mount(router); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/node/identity", "/node/metrics", "/node/static"} {
		for _, key := range []string{"", "invalid-secret"} {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set(request.NodeSecretHeader, key)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s with secret %q: status %d, body %s", path, key, w.Code, w.Body)
			}
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/node/identity", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(request.NodeSecretHeader, secret)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("node identity: status %d, body %s", w.Code, w.Body)
	}
	r = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"password":"test-password","persistence":"session"}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil || w.Code != http.StatusOK || login.AccessToken == "" {
		t.Fatalf("login: status %d, body %s, error %v", w.Code, w.Body, err)
	}
	for _, token := range []string{"", login.AccessToken} {
		r = httptest.NewRequest(http.MethodGet, "/auth/sessions", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w = httptest.NewRecorder()
		router.ServeHTTP(w, r)
		want := http.StatusUnauthorized
		if token != "" {
			want = http.StatusOK
		}
		if w.Code != want {
			t.Fatalf("sessions: status %d, want %d, body %s", w.Code, want, w.Body)
		}
	}
}
