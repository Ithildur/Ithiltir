package vmid

import (
	"dash/internal/http/api/virt/id/vms/id/history"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/history", history.Router(node, sessions))
	return r
}
