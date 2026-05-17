package node

import (
	"net/netip"

	"dash/internal/config"
	"dash/internal/serverid"
	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func ingestMiddleware(trustedProxies []netip.Prefix) []routes.Middleware {
	return []routes.Middleware{
		middleware.RateLimit(middleware.RateLimitOptions{
			Requests: config.NodeRateLimitRequests,
			Window:   config.NodeRateLimitWindow,
			KeyFunc: middleware.RateLimitKeyByIP(24, 40, middleware.RateLimitKeyOptions{
				TrustedProxies: append([]netip.Prefix(nil), trustedProxies...),
			}),
		}),
		middleware.LimitBody(config.NodeMaxMetricsBodySize),
	}
}

// Router returns node routes.
func Router(st *store.Stores, serverID *serverid.Store, staleAfterSec int, trustedProxies []netip.Prefix) *routes.Blueprint {
	h := newHandler(st.Node, st.Metric, st.Front, st.Alert, serverID, staleAfterSec)
	chain := ingestMiddleware(trustedProxies)
	all := append([]routes.Middleware{middleware.RequireJSONBody}, chain...)
	r := routes.NewBlueprint(routes.DefaultMiddleware(all...))
	h.metricsRoute(r)
	h.staticRoute(r)
	h.identityRoute(r)
	return r
}
