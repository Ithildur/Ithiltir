package traffic

import (
	"net/http"

	trafficjob "dash/internal/traffic"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	runner *trafficjob.RebuildRunner
}

// Router returns admin/nodes/traffic routes.
func Router(runner *trafficjob.RebuildRunner) *routes.Blueprint {
	h := &handler{runner: runner}
	r := routes.NewBlueprint(routes.DefaultTags("admin", "nodes"))
	rebuildRoute(r, h)
	return r
}

func rebuildRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/rebuild",
		"Get traffic rebuild status",
		routes.Func(h.rebuildStatusHandler),
	)
}

func (h *handler) rebuildStatusHandler(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, h.runner.Current())
}
