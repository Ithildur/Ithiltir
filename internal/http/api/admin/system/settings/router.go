package settings

import (
	"dash/internal/store"
	"dash/internal/store/metricdata"
	"dash/internal/store/system"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	tx     settingsTx
	metric *metricdata.Store
	system *system.Store
}

// Router returns admin/system/settings routes.
func Router(st *store.Stores) *routes.Blueprint {
	h := &handler{tx: st, metric: st.Metric, system: st.System}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "system"),
	)
	detailRoute(r, h)
	replaceRoute(r, h)
	patchRoute(r, h)
	return r
}
