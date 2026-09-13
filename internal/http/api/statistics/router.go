package statistics

import (
	"time"

	trafficapi "dash/internal/http/api/statistics/traffic"
	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store          *store.Stores
	optionalBearer routes.Middleware
}

func Router(st *store.Stores, loc *time.Location, bearer, optionalBearer routes.Middleware) *routes.Blueprint {
	h := &handler{store: st, optionalBearer: optionalBearer}

	r := routes.NewBlueprint()
	h.accessRoute(r)
	r.Include("/traffic", trafficapi.Router(st, loc, bearer, optionalBearer))
	return r
}
