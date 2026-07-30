package transporthttp

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"dash/internal/config"
	nodestore "dash/internal/store/node"
	systemstore "dash/internal/store/system"
	themefs "dash/internal/theme"
	"dash/internal/transport/http/request"
	themeroute "dash/internal/transport/http/theme"
	kitstatic "github.com/Ithildur/EiluneKit/http/static"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func Register(router chi.Router, cfg *config.Config, system *systemstore.Store, node *nodestore.Store, themes *themefs.Store) (http.Handler, error) {
	opts := kitstatic.Options{
		AppDir:      config.DefaultAppDirOptions(),
		Development: !isProductionEnv(cfg.App.Env),
	}

	if err := themeroute.Router(system, themes).MountAt(router, "/theme"); err != nil {
		return nil, err
	}
	registerInstallScriptRoutes(router, cfg)
	if err := mountDeployRoute(router, node, opts); err != nil {
		return nil, err
	}
	spa, err := kitstatic.SPAHandler("dist", opts)
	if err != nil {
		return nil, err
	}
	spa = noStoreThemeBootstrap(spa)
	router.Handle("/*", spa)
	return spa, nil
}

func noStoreThemeBootstrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/theme-bootstrap.js" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func registerInstallScriptRoutes(router chi.Router, cfg *config.Config) {
	mountInstallScriptRoute(router, cfg, "/deploy/linux/install.sh", "linux", "text/x-shellscript; charset=utf-8")
	mountInstallScriptRoute(router, cfg, "/deploy/macos/install.sh", "macos", "text/x-shellscript; charset=utf-8")
	mountInstallScriptRoute(router, cfg, "/deploy/windows/install.ps1", "windows", "text/plain; charset=utf-8")
}

func mountInstallScriptRoute(router chi.Router, cfg *config.Config, routePath, platform, contentType string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		script, err := renderInstallScript(cfg, platform)
		if err != nil {
			http.Error(w, "render install script failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(script)
	}

	router.Get(routePath, handler)
	router.Head(routePath, handler)
}

func mountDeployRoute(router chi.Router, node *nodestore.Store, opts kitstatic.Options) error {
	dir, err := kitstatic.ResolveDir("deploy", opts)
	if err != nil {
		return err
	}

	handler := requireDeployAccess(node, http.StripPrefix("/deploy", http.FileServer(http.Dir(dir))))
	router.Handle("/deploy/*", handler)
	return nil
}

func requireDeployAccess(node *nodestore.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := validDeployAccess(r, node)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(config.DeployWriteTimeout))
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
