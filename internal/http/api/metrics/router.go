package metrics

import (
	"time"

	"dash/internal/http/api/metrics/uptime"
	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns metrics routes.
func Router(st *store.Stores, loc *time.Location, optionalBearer routes.Middleware) *routes.Blueprint {
	h := newHandler(st.Metric, st.Front, st.Node, optionalBearer)
	r := routes.NewBlueprint()
	h.historyRoute(r)
	h.onlineRoute(r)
	r.Include("/uptime", uptime.Router(st, loc), routes.IncludeAuth(routes.AuthOptional), routes.IncludeMiddleware(optionalBearer))
	return r
}
