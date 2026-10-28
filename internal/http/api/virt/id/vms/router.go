package vms

import (
	vmid "dash/internal/http/api/virt/id/vms/id"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/{vmid}", vmid.Router(node, sessions))
	return r
}
