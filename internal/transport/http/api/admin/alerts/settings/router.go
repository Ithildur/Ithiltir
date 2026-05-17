package settings

import (
	alertstore "dash/internal/store/alert"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store *alertstore.Store
}

// Router returns admin/alerts/settings routes.
func Router(st *alertstore.Store) *routes.Blueprint {
	h := &handler{store: st}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "alerts"),
	)
	detailRoute(r, h)
	replaceRoute(r, h)
	return r
}
