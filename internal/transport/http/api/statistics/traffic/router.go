package traffic

import (
	"time"

	"dash/internal/store"
	"dash/internal/store/frontcache"
	trafficstore "dash/internal/store/traffic"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	traffic  *trafficstore.Store
	front    *frontcache.Store
	auth     *authjwt.Manager
	location *time.Location
	bearer   routes.Middleware
}

func Router(st *store.Stores, auth *authjwt.Manager, loc *time.Location, bearer routes.Middleware) *routes.Blueprint {
	if loc == nil {
		loc = time.Local
	}
	h := &handler{traffic: st.Traffic, front: st.Front, auth: auth, location: loc, bearer: bearer}

	r := routes.NewBlueprint(routes.DefaultTags("statistics", "traffic"))
	h.settingsRoute(r)
	h.ifacesRoute(r)
	h.summaryRoute(r)
	h.dailyRoute(r)
	h.monthlyRoute(r)
	return r
}
