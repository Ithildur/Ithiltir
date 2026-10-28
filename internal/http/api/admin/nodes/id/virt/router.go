package virt

import (
	"dash/internal/http/api/admin/nodes/id/virt/capabilities"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/capabilities", capabilities.Router(node, sessions))
	return r
}
