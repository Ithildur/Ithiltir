package nodes

import (
	"dash/internal/config"
	"dash/internal/http/api/admin/nodes/id"
	nodestraffic "dash/internal/http/api/admin/nodes/traffic"
	nodestore "dash/internal/store/node"
	trafficjob "dash/internal/traffic"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store  *nodestore.Store
	config *config.Config
}

// Router returns admin/nodes routes.
func Router(st *nodestore.Store, runner *trafficjob.RebuildRunner, cfg *config.Config) *routes.Blueprint {
	h := &handler{store: st, config: cfg}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "nodes"),
	)
	listRoute(r, h)
	deployRoute(r, h)
	createRoute(r, h)
	displayOrderRoute(r, h)
	trafficP95Route(r, h)
	r.Include("/traffic", nodestraffic.Router(runner))
	r.Include("/{id}", nodeid.Router(st, runner, cfg))
	return r
}
