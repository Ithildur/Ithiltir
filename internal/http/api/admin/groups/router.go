package groups

import (
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store *nodestore.Store
}

// Router returns admin/groups routes.
func Router(st *nodestore.Store) *routes.Blueprint {
	h := &handler{store: st}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "groups"),
	)
	listRoute(r, h)
	lookupRoute(r, h)
	createRoute(r, h)
	updateRoute(r, h)
	deleteRoute(r, h)
	return r
}
