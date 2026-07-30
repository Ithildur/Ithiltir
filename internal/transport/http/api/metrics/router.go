package metrics

import (
	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns metrics routes.
func Router(st *store.Stores, optionalBearer routes.Middleware) *routes.Blueprint {
	h := newHandler(st.Metric, st.Front, st.Node, optionalBearer)
	r := routes.NewBlueprint()
	h.historyRoute(r)
	h.onlineRoute(r)
	return r
}
