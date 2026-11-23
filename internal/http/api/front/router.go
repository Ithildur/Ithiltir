package front

import (
	"time"

	"dash/internal/config"
	frontmetrics "dash/internal/http/api/front/metrics"
	"dash/internal/metrics"
	"dash/internal/store"
	nodestore "dash/internal/store/node"
	systemstore "dash/internal/store/system"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	node           *nodestore.Store
	system         *systemstore.Store
	optionalBearer routes.Middleware
}

// Router returns front routes.
func Router(st *store.Stores, offlineThreshold time.Duration, optionalBearer routes.Middleware) *routes.Blueprint {
	h := &handler{node: st.Node, system: st.System, optionalBearer: optionalBearer}
	if offlineThreshold <= 0 {
		offlineThreshold = config.DefaultNodeOfflineThreshold
	}
	r := routes.NewBlueprint()
	h.brandRoute(r)
	r.Include("/metrics", frontmetrics.Router(st.Front, metrics.DurationSecondsCeil(offlineThreshold)),
		routes.IncludeTags("front"), routes.IncludeAuth(routes.AuthOptional), routes.IncludeMiddleware(optionalBearer))
	h.groupsRoute(r)
	return r
}
