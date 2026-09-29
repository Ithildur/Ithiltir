package api

import (
	"fmt"
	"net/http"
	"net/netip"
	"time"

	authhttp "github.com/Ithildur/EiluneKit/auth/http"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/config"
	updater "dash/internal/dashupdate"
	adminapi "dash/internal/http/api/admin"
	authapi "dash/internal/http/api/auth"
	frontapi "dash/internal/http/api/front"
	metricsapi "dash/internal/http/api/metrics"
	nodeapi "dash/internal/http/api/node"
	statisticsapi "dash/internal/http/api/statistics"
	versionapi "dash/internal/http/api/version"
	"dash/internal/http/request"
	"dash/internal/nodeingest"
	"dash/internal/nodesession"
	"dash/internal/store"
	themefs "dash/internal/theme"
	trafficjob "dash/internal/traffic"
)

// Dependencies holds shared dependencies for HTTP handlers.
type Dependencies struct {
	Stores         *store.Stores
	Auth           *authjwt.Manager
	Theme          *themefs.Store
	TrafficRebuild *trafficjob.RebuildRunner
	DashUpdate     *updater.Runner
	NodeIngest     *nodeingest.Receiver
	NodeSessions   *nodesession.Hub
}

const passwordOnlyAuthUserID = "dash-admin"

type routeSetup struct {
	authHandler      *authhttp.Handler
	bearer           routes.Middleware
	optionalBearer   routes.Middleware
	offlineThreshold time.Duration
	trustedProxies   []netip.Prefix
}

func prepareRoutes(cfg *config.Config, deps Dependencies) (routeSetup, error) {
	if cfg == nil {
		return routeSetup{}, fmt.Errorf("api: config is nil")
	}

	if deps.Stores == nil {
		return routeSetup{}, fmt.Errorf("api: store is nil")
	}

	if deps.Auth == nil {
		return routeSetup{}, fmt.Errorf("api: auth manager is nil")
	}
	if deps.Theme == nil {
		return routeSetup{}, fmt.Errorf("api: theme store is nil")
	}
	if deps.TrafficRebuild == nil {
		return routeSetup{}, fmt.Errorf("api: traffic rebuild runner is nil")
	}
	if deps.DashUpdate == nil {
		return routeSetup{}, fmt.Errorf("api: dash update runner is nil")
	}
	if deps.NodeIngest == nil {
		return routeSetup{}, fmt.Errorf("api: node receiver is nil")
	}
	if deps.NodeSessions == nil {
		return routeSetup{}, fmt.Errorf("api: node sessions are nil")
	}

	trustedProxies := append([]netip.Prefix(nil), cfg.HTTP.TrustedProxyPrefixes...)
	authHandler, err := newAuthHandler(cfg.Auth.Password, deps.Auth, trustedProxies)
	if err != nil {
		return routeSetup{}, err
	}
	bearer, err := authhttp.RequireBearer(deps.Auth)
	if err != nil {
		return routeSetup{}, fmt.Errorf("api: build bearer middleware: %w", err)
	}
	optionalBearer, err := request.OptionalBearer(deps.Auth)
	if err != nil {
		return routeSetup{}, fmt.Errorf("api: build optional bearer middleware: %w", err)
	}

	return routeSetup{
		authHandler:      authHandler,
		bearer:           bearer,
		optionalBearer:   optionalBearer,
		offlineThreshold: cfg.App.EffectiveNodeOfflineThreshold(),
		trustedProxies:   trustedProxies,
	}, nil
}

func buildRoutes(cfg *config.Config, deps Dependencies, setup routeSetup) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Add(setup.authHandler.Routes()...)
	r.Include("/version", versionapi.Router())
	r.Include("/admin", adminapi.Router(deps.Stores, cfg, deps.Theme, deps.TrafficRebuild, deps.DashUpdate, deps.NodeSessions), routes.IncludeAuth(routes.AuthRequired), routes.IncludeMiddleware(setup.bearer))
	// Node handlers authenticate X-Node-Secret independently of bearer sessions.
	r.Include("/node", nodeapi.Router(deps.NodeIngest, setup.trustedProxies))
	r.Include("/front", frontapi.Router(deps.Stores, setup.offlineThreshold, setup.optionalBearer))
	r.Include("/metrics", metricsapi.Router(deps.Stores, cfg.App.EffectiveLocation(), setup.optionalBearer))
	r.Include("/statistics", statisticsapi.Router(deps.Stores, cfg.App.EffectiveLocation(), setup.bearer, setup.optionalBearer))
	return r
}

// Router returns API routes for the parent to mount.
func Router(cfg *config.Config, deps Dependencies) (*routes.Blueprint, error) {
	setup, err := prepareRoutes(cfg, deps)
	if err != nil {
		return nil, err
	}
	return buildRoutes(cfg, deps, setup), nil
}

func newAuthHandler(password string, auth authhttp.TokenManager, trustedProxies []netip.Prefix) (*authhttp.Handler, error) {
	if err := config.ValidateAdminPassword(password); err != nil {
		return nil, fmt.Errorf("api: invalid admin password: %w", err)
	}
	authenticator, err := authhttp.NewStaticPassword(passwordOnlyAuthUserID, password)
	if err != nil {
		return nil, fmt.Errorf("api: invalid admin password: %w", err)
	}

	return authapi.NewHandler(auth, authhttp.Options{
		LoginAuthenticator: authenticator,
		BasePath:           new("/auth"),
		RefreshCookiePath:  "/api/auth",
		CookieSameSite:     http.SameSiteStrictMode,
		TrustedProxies:     trustedProxies,
	})
}
