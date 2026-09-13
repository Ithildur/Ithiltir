package traffic

import (
	"time"

	"dash/internal/store"
	"dash/internal/store/frontcache"
	nodestore "dash/internal/store/node"
	trafficstore "dash/internal/store/traffic"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	traffic        *trafficstore.Store
	front          *frontcache.Store
	node           *nodestore.Store
	location       *time.Location
	bearer         routes.Middleware
	optionalBearer routes.Middleware
}

func Router(st *store.Stores, loc *time.Location, bearer, optionalBearer routes.Middleware) *routes.Blueprint {
	h := &handler{traffic: st.Traffic, front: st.Front, node: st.Node, location: loc, bearer: bearer, optionalBearer: optionalBearer}

	r := routes.NewBlueprint(routes.DefaultTags("statistics", "traffic"))
	h.settingsRoute(r)
	h.ifacesRoute(r)
	h.summaryRoute(r)
	h.dailyRoute(r)
	h.monthlyRoute(r)
	return r
}
