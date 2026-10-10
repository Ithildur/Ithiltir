package api

import (
	"fmt"
	"net/http"
	"net/netip"

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

// Router returns API routes for the parent to mount.
func Router(cfg *config.Config, deps Dependencies) (*routes.Blueprint, error) {
	trustedProxies := append([]netip.Prefix(nil), cfg.HTTP.TrustedProxyPrefixes...)
	authHandler, err := newAuthHandler(cfg.Auth.Password, deps.Auth, trustedProxies)
	if err != nil {
		return nil, err
	}
	bearer, err := authhttp.RequireBearer(deps.Auth)
	if err != nil {
		return nil, fmt.Errorf("api: build bearer middleware: %w", err)
	}
	optionalBearer, err := request.OptionalBearer(deps.Auth)
	if err != nil {
		return nil, fmt.Errorf("api: build optional bearer middleware: %w", err)
	}

	r := routes.NewBlueprint()
	r.Add(authHandler.Routes()...)
	r.Include("/version", versionapi.Router())
	r.Include("/admin", adminapi.Router(deps.Stores, cfg, deps.Theme, deps.TrafficRebuild, deps.DashUpdate, deps.NodeSessions), routes.IncludeAuth(routes.AuthRequired), routes.IncludeMiddleware(bearer))
	// Node handlers authenticate X-Node-Secret independently of bearer sessions.
	r.Include("/node", nodeapi.Router(deps.NodeIngest, trustedProxies))
	r.Include("/front", frontapi.Router(deps.Stores, cfg.App.EffectiveNodeOfflineThreshold(), optionalBearer))
	r.Include("/metrics", metricsapi.Router(deps.Stores, cfg.App.EffectiveLocation(), optionalBearer))
	r.Include("/statistics", statisticsapi.Router(deps.Stores, cfg.App.EffectiveLocation(), bearer, optionalBearer))
	return r, nil
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
