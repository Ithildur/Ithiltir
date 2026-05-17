package settings

import (
	"dash/internal/store/metricdata"
	"dash/internal/store/system"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	metric *metricdata.Store
	system *system.Store
}

// Router returns admin/system/settings routes.
func Router(metric *metricdata.Store, system *system.Store) *routes.Blueprint {
	h := &handler{metric: metric, system: system}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "system"),
	)
	detailRoute(r, h)
	replaceRoute(r, h)
	patchRoute(r, h)
	return r
}
