package nodeid

import (
	"dash/internal/config"
	nodetraffic "dash/internal/http/api/admin/nodes/id/traffic"
	nodevirt "dash/internal/http/api/admin/nodes/id/virt"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
	trafficjob "dash/internal/traffic"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store  *nodestore.Store
	config *config.Config
}

// Router returns admin/nodes/{id} routes.
func Router(node *nodestore.Store, runner *trafficjob.RebuildRunner, cfg *config.Config, sessions *nodesession.Hub) *routes.Blueprint {
	h := &handler{store: node, config: cfg}
	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "nodes"),
	)
	updateRoute(r, h)
	deleteRoute(r, h)
	upgradeRoute(r, h)
	r.Include("/traffic", nodetraffic.Router(node, runner))
	r.Include("/virt", nodevirt.Router(node, sessions))
	return r
}
