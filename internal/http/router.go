package httpserver

import (
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/Ithildur/EiluneKit/clientip"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/config"
	"dash/internal/http/api"
	"dash/internal/http/deploy"
	themeroute "dash/internal/http/theme"
	"dash/internal/infra"
)

// Dependencies holds the application dependencies used by the HTTP server.
type Dependencies = api.Dependencies

func newHandler(cfg *config.Config, deps Dependencies) (http.Handler, error) {
	apiRoutes, err := api.Router(cfg, deps)
	if err != nil {
		return nil, err
	}
	files, err := staticFallback(cfg, deps.Stores.Node)
	if err != nil {
		return nil, err
	}
	root := routes.NewBlueprint()
	root.Include("/api", apiRoutes)
	root.Include("/theme", themeroute.Router(deps.Stores.System, deps.Theme))
	root.Include("/deploy", deploy.Router(
		installScriptHandler(cfg, "linux", "text/x-shellscript; charset=utf-8"),
		installScriptHandler(cfg, "macos", "text/x-shellscript; charset=utf-8"),
		installScriptHandler(cfg, "windows", "text/plain; charset=utf-8"),
	))
	routeList := root.Routes()
	return routes.NewHandler(routeList, handlerOptions(cfg, files, routeList))
}

func handlerOptions(cfg *config.Config, files fallback, routeList []routes.Route) routes.HandlerOptions {
	return routes.HandlerOptions{
		Middleware: []routes.Middleware{
			middleware.RequestID,
			securityHeaders,
			middleware.AccessLog(middleware.AccessLogOptions{
				Disabled: !infra.DebugEnabled(),
				Logger:   infra.SlogWithModule("http"),
				MinLevel: slog.LevelDebug,
				ClientIP: clientip.Options{
					TrustedProxies: append([]netip.Prefix(nil), cfg.HTTP.TrustedProxyPrefixes...),
				},
				Skip: func(_ *http.Request, status int) bool {
					return isProductionEnv(cfg.App.Env) && status == http.StatusNotFound
				},
			}),
			middleware.Recover(middleware.RecoverOptions{
				Logger: infra.SlogWithModule("http"),
				OnPanic: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}),
			}),
			apiBoundary,
			methodBoundary(routeList),
		},
		NotFound:         files,
		MethodNotAllowed: http.HandlerFunc(methodNotAllowed),
	}
}
