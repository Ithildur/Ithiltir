package vmid

import (
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/http/api/admin/nodes/id/virt/vms/id/history"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
)

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/history", history.Router(node, sessions))
	return r
}
