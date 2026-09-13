package httpserver

import (
	"dash/internal/dashupdate"
	"dash/internal/http/api"
	themeroute "dash/internal/http/theme"
	"dash/internal/store"
	"dash/internal/theme"
	"dash/internal/traffic"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/middleware"
)

// Dependencies holds the application dependencies used by the HTTP server.
type Dependencies struct {
	Stores         *store.Stores
	Auth           *authjwt.Manager
	Theme          *theme.Store
	TrafficRebuild *traffic.RebuildRunner
	DashUpdate     *dashupdate.Runner
}

func (s *HTTPServer) registerRoutes() error {
	if err := api.Register(s.router, s.cfg, api.Dependencies{
		Stores:         s.deps.Stores,
		Auth:           s.deps.Auth,
		Theme:          s.deps.Theme,
		TrafficRebuild: s.deps.TrafficRebuild,
		DashUpdate:     s.deps.DashUpdate,
	}); err != nil {
		return err
	}
	if err := themeroute.Router(s.deps.Stores.System, s.deps.Theme).MountAt(s.router, "/theme"); err != nil {
		return err
	}
	pageHandler, err := registerStaticRoutes(s.router, s.cfg, s.deps.Stores.Node)
	if err != nil {
		return err
	}
	s.router.NotFound(middleware.NotFoundHandler(pageHandler))
	return nil
}
