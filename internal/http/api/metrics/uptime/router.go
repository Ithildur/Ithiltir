package uptime

import (
	"time"

	dayapi "dash/internal/http/api/metrics/uptime/day"
	"dash/internal/store"
	"dash/internal/store/metricdata"
	"dash/internal/store/system"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	metric *metricdata.Store
	system *system.Store
	loc    *time.Location
}

func Router(st *store.Stores, loc *time.Location) *routes.Blueprint {
	h := &handler{metric: st.Metric, system: st.System, loc: loc}
	r := routes.NewBlueprint(routes.DefaultTags("metrics"))
	dailyRoute(r, h)
	r.Include("/day", dayapi.Router(st.Metric, st.System, loc))
	return r
}
