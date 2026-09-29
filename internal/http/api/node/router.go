package node

import (
	"net/netip"

	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/config"
	nodevirt "dash/internal/http/api/node/virt"
	"dash/internal/nodeingest"
)

func ingestMiddleware() []routes.Middleware {
	return []routes.Middleware{
		middleware.LimitBody(config.NodeMaxMetricsBodySize),
	}
}

// Router returns node routes.
func Router(ingest *nodeingest.Receiver, trustedProxies []netip.Prefix) *routes.Blueprint {
	h := &handler{ingest: ingest, failedAuth: failedAuthHandler(trustedProxies)}
	r := routes.NewBlueprint(routes.DefaultMiddleware(middleware.RequireJSONBody))
	h.metricsRoute(r)
	h.staticRoute(r)
	h.identityRoute(r)
	r.Include("/virt", nodevirt.Router(ingest, h.failedAuth))
	return r
}
