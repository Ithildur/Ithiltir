package api

import (
	"fmt"
	"math"
	"net/netip"
	"time"

	"dash/internal/config"
	"dash/internal/serverid"
	"dash/internal/store"
	themefs "dash/internal/theme"
	adminapi "dash/internal/transport/http/api/admin"
	authapi "dash/internal/transport/http/api/auth"
	frontapi "dash/internal/transport/http/api/front"
	metricsapi "dash/internal/transport/http/api/metrics"
	nodeapi "dash/internal/transport/http/api/node"
	statisticsapi "dash/internal/transport/http/api/statistics"
	versionapi "dash/internal/transport/http/api/version"
	authhttp "github.com/Ithildur/EiluneKit/auth/http"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	kitmw "github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"github.com/go-chi/chi/v5"
)

// Dependencies holds shared dependencies for HTTP handlers.
type Dependencies struct {
	Stores *store.Stores
	Auth   *authjwt.Manager
	Theme  *themefs.Store
}

const (
	passwordOnlyAuthUserID = "dash-admin"

	// AuthRequirement is export metadata only. /api/node validates X-Node-Secret in handlers.
	nodeSecretAuth routes.AuthRequirement = "node-secret"
)

type routeSetup struct {
	authHandler      *authhttp.Handler
	bearer           routes.Middleware
	serverID         *serverid.Store
	offlineThreshold time.Duration
	staleAfterSec    int
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

	offlineThreshold, staleAfterSec := nodeThresholds(cfg)
	trustedProxies := append([]netip.Prefix(nil), cfg.HTTP.TrustedProxyPrefixes...)
	authHandler, err := newAuthHandler(cfg.Auth.Password, deps.Auth, trustedProxies)
	if err != nil {
		return routeSetup{}, err
	}
	bearer, err := authhttp.RequireBearer(deps.Auth)
	if err != nil {
		return routeSetup{}, fmt.Errorf("api: build bearer middleware: %w", err)
	}
	installIDPath, err := config.InstallIDPath()
	if err != nil {
		return routeSetup{}, fmt.Errorf("api: resolve install id path: %w", err)
	}

	return routeSetup{
		authHandler:      authHandler,
		bearer:           bearer,
		serverID:         serverid.New(installIDPath),
		offlineThreshold: offlineThreshold,
		staleAfterSec:    staleAfterSec,
		trustedProxies:   trustedProxies,
	}, nil
}

func buildRoutes(cfg *config.Config, deps Dependencies, setup routeSetup) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Add(setup.authHandler.Routes()...)
	r.Include("/version", versionapi.Router())
	r.Include("/admin", adminapi.Router(deps.Stores, cfg, deps.Theme), routes.IncludeAuth(routes.AuthRequired), routes.IncludeMiddleware(setup.bearer))
	r.Include("/node", nodeapi.Router(deps.Stores, setup.serverID, setup.staleAfterSec, setup.trustedProxies), routes.IncludeAuth(nodeSecretAuth))
	r.Include("/front", frontapi.Router(deps.Stores, setup.offlineThreshold, deps.Auth))
	r.Include("/metrics", metricsapi.Router(deps.Stores, deps.Auth))
	r.Include("/statistics", statisticsapi.Router(deps.Stores, deps.Auth, cfg.App.Timezone, setup.bearer))
	return r
}

// Register mounts /api routes onto router.
func Register(router chi.Router, cfg *config.Config, deps Dependencies) error {
	setup, err := prepareRoutes(cfg, deps)
	if err != nil {
		return err
	}

	blueprint := buildRoutes(cfg, deps, setup)

	var mountErr error
	router.Route("/api", func(r chi.Router) {
		r.MethodNotAllowed(kitmw.MethodNotAllowedResponder(r))
		if err := blueprint.Mount(r); err != nil && mountErr == nil {
			mountErr = err
		}
	})
	return mountErr
}

func newAuthHandler(password string, auth authhttp.TokenManager, trustedProxies []netip.Prefix) (*authhttp.Handler, error) {
	authenticator, err := authhttp.NewStaticPassword(passwordOnlyAuthUserID, password)
	if err != nil {
		return nil, fmt.Errorf("api: invalid admin password: %w", err)
	}

	return authapi.NewHandler(auth, authhttp.Options{
		LoginAuthenticator: authenticator,
		BasePath:           "/auth",
		RefreshCookiePath:  "/api/auth",
		TrustedProxies:     trustedProxies,
	})
}

func nodeThresholds(cfg *config.Config) (time.Duration, int) {
	offlineThreshold := cfg.App.EffectiveNodeOfflineThreshold()
	return offlineThreshold, int(math.Ceil(offlineThreshold.Seconds()))
}
