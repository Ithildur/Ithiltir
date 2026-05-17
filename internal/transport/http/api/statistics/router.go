package statistics

import (
	"dash/internal/store"
	trafficapi "dash/internal/transport/http/api/statistics/traffic"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store *store.Stores
	auth  *authjwt.Manager
}

func Router(st *store.Stores, auth *authjwt.Manager, timezone string, bearer routes.Middleware) *routes.Blueprint {
	h := &handler{store: st, auth: auth}

	r := routes.NewBlueprint()
	h.accessRoute(r)
	r.Include("/traffic", trafficapi.Router(st, auth, timezone, bearer))
	return r
}
