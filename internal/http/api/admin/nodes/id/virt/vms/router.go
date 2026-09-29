package vms

import (
	"github.com/Ithildur/EiluneKit/http/routes"

	vmid "dash/internal/http/api/admin/nodes/id/virt/vms/id"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
)

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/{vmid}", vmid.Router(node, sessions))
	return r
}
