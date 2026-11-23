package frontmetrics

import (
	metricid "dash/internal/http/api/front/metrics/id"
	"dash/internal/store/frontcache"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	front         *frontcache.Store
	staleAfterSec int
}

func Router(front *frontcache.Store, staleAfterSec int) *routes.Blueprint {
	r := routes.NewBlueprint()
	listRoute(r, &handler{front: front, staleAfterSec: staleAfterSec})
	r.Include("/{id}", metricid.Router(front, staleAfterSec))
	return r
}
