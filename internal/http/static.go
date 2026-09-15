package httpserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/http/request"
	nodestore "dash/internal/store/node"
	kitstatic "github.com/Ithildur/EiluneKit/http/static"

	"gorm.io/gorm"
)

func staticFallback(cfg *config.Config, node *nodestore.Store) (fallback, error) {
	opts := kitstatic.Options{
		AppDir:      config.DefaultAppDirOptions(),
		Development: !isProductionEnv(cfg.App.Env),
	}

	download, err := deployHandler(node, opts)
	if err != nil {
		return fallback{}, err
	}
	spa, err := kitstatic.SPAHandler("dist", opts)
	if err != nil {
		return fallback{}, err
	}
	return fallback{page: noStoreThemeBootstrap(spa), deploy: download}, nil
}

func noStoreThemeBootstrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/theme-bootstrap.js" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func installScriptHandler(cfg *config.Config, platform, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		script, err := renderInstallScript(cfg, platform)
		if err != nil {
			http.Error(w, "render install script failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(script)
	})
}

func deployHandler(node *nodestore.Store, opts kitstatic.Options) (http.Handler, error) {
	dir, err := kitstatic.ResolveDir("deploy", opts)
	if err != nil {
		return nil, err
	}

	return requireDeployAccess(node, http.StripPrefix("/deploy", http.FileServer(http.Dir(dir)))), nil
}

func requireDeployAccess(node *nodestore.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Vary", request.NodeSecretHeader)
		ok, err := validDeployAccess(r, node)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(config.DeployWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validDeployAccess(r *http.Request, node *nodestore.Store) (bool, error) {
	secret := strings.TrimSpace(r.Header.Get(request.NodeSecretHeader))
	if secret != "" {
		if _, err := node.GetServerBySecret(r.Context(), secret); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	token := strings.TrimSpace(r.URL.Query().Get(request.DeployGrantQuery))
	return node.ValidDeployGrant(token, r.URL.Path), nil
}
