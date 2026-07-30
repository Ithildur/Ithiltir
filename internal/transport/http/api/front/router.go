package front

import (
	"time"

	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns front routes.
func Router(st *store.Stores, offlineThreshold time.Duration, optionalBearer routes.Middleware) *routes.Blueprint {
	h := newHandler(st.Front, st.Node, st.System, offlineThreshold, optionalBearer)
	r := routes.NewBlueprint()
	h.brandRoute(r)
	h.metricsRoute(r)
	h.groupsRoute(r)
	return r
}
