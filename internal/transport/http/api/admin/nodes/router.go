package nodes

import (
	"dash/internal/config"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store  *nodestore.Store
	config *config.Config
}

// Router returns admin/nodes routes.
func Router(st *nodestore.Store, cfg *config.Config) *routes.Blueprint {
	h := &handler{store: st, config: cfg}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "nodes"),
	)
	listRoute(r, h)
	deployRoute(r, h)
	createRoute(r, h)
	displayOrderRoute(r, h)
	trafficP95Route(r, h)
	upgradeRoute(r, h)
	updateRoute(r, h)
	deleteRoute(r, h)
	return r
}
