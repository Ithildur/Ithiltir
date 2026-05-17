package mounts

import (
	alertstore "dash/internal/store/alert"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	alert *alertstore.Store
	node  *nodestore.Store
}

// Router returns admin/alerts/mounts routes.
func Router(alert *alertstore.Store, node *nodestore.Store) *routes.Blueprint {
	h := &handler{alert: alert, node: node}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "alerts"),
	)
	listRoute(r, h)
	replaceRoute(r, h)
	return r
}
