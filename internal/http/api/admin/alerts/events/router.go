package events

import (
	alertstore "dash/internal/store/alert"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	alerts *alertstore.Store
	nodes  *nodestore.Store
}

// Router returns admin/alerts/events routes.
func Router(alerts *alertstore.Store, nodes *nodestore.Store) *routes.Blueprint {
	h := &handler{alerts: alerts, nodes: nodes}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "alerts"),
	)
	listRoute(r, h)
	summaryRoute(r, h)
	serversRoute(r, h)
	return r
}
