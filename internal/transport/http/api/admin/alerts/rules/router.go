package rules

import (
	alertstore "dash/internal/store/alert"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store *alertstore.Store
}

// Router returns admin/alerts/rules routes.
func Router(st *alertstore.Store) *routes.Blueprint {
	h := &handler{store: st}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "alerts"),
	)
	listRoute(r, h)
	createRoute(r, h)
	updateRoute(r, h)
	deleteRoute(r, h)
	return r
}
