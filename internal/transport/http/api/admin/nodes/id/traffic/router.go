package traffic

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	trafficjob "dash/internal/traffic"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	node   *nodestore.Store
	runner *trafficjob.RebuildRunner
}

// Router returns admin/nodes/{id}/traffic routes.
func Router(node *nodestore.Store, runner *trafficjob.RebuildRunner) *routes.Blueprint {
	h := &handler{
		node:   node,
		runner: runner,
	}
	r := routes.NewBlueprint(routes.DefaultTags("admin", "nodes"))
	rebuildRoute(r, h)
	return r
}

func rebuildRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/rebuild",
		"Rebuild node traffic",
		routes.Func(h.rebuildHandler),
	)
}

func (h *handler) rebuildHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := h.existingNodeID(w, r)
	if !ok {
		return
	}

	view, err := h.runner.Start(id)
	if errors.Is(err, trafficjob.ErrRebuildRunning) {
		httperr.Write(w, http.StatusConflict, "traffic_rebuild_running", "traffic rebuild is already running")
		return
	}
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "traffic_rebuild_unavailable", "traffic rebuild is unavailable")
		return
	}
	response.WriteJSON(w, http.StatusAccepted, view)
}

func (h *handler) existingNodeID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return 0, false
	}

	exists, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) (bool, error) {
		return h.node.NodeExists(c, id)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch node")
		return 0, false
	}
	if !exists {
		httperr.Write(w, http.StatusNotFound, "not_found", "node not found")
		return 0, false
	}
	return id, true
}
