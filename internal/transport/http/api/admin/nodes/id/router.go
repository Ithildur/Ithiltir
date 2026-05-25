package nodeid

import (
	"dash/internal/config"
	nodestore "dash/internal/store/node"
	trafficjob "dash/internal/traffic"
	nodetraffic "dash/internal/transport/http/api/admin/nodes/id/traffic"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store  *nodestore.Store
	config *config.Config
}

// Router returns admin/nodes/{id} routes.
func Router(node *nodestore.Store, runner *trafficjob.RebuildRunner, cfg *config.Config) *routes.Blueprint {
	h := &handler{store: node, config: cfg}
	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "nodes"),
	)
	updateRoute(r, h)
	deleteRoute(r, h)
	upgradeRoute(r, h)
	r.Include("/traffic", nodetraffic.Router(node, runner))
	return r
}
