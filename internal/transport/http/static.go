package transporthttp

import (
	"net/http"
	"time"

	"dash/internal/config"
	systemstore "dash/internal/store/system"
	themefs "dash/internal/theme"
	themeroute "dash/internal/transport/http/theme"
	kitstatic "github.com/Ithildur/EiluneKit/http/static"

	"github.com/go-chi/chi/v5"
)

func Register(router chi.Router, cfg *config.Config, st *systemstore.Store, themes *themefs.Store) (http.Handler, error) {
	opts := kitstatic.Options{
		AppDir:      config.DefaultAppDirOptions(),
		Development: !isProductionEnv(cfg.App.Env),
	}

	if err := themeroute.Router(st, themes).MountAt(router, "/theme"); err != nil {
		return nil, err
	}
	registerInstallScriptRoutes(router, cfg)
	if err := mountDeployRoute(router, opts); err != nil {
		return nil, err
	}
	return kitstatic.MountSPA(router, "/", "dist", opts)
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

func mountDeployRoute(router chi.Router, opts kitstatic.Options) error {
	dir, err := kitstatic.ResolveDir("deploy", opts)
	if err != nil {
		return err
	}

	handler := clearWriteDeadline(http.StripPrefix("/deploy", http.FileServer(http.Dir(dir))))
	router.Handle("/deploy/*", handler)
	return nil
}

func clearWriteDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		next.ServeHTTP(w, r)
	})
}
