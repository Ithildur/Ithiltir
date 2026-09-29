package virt

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
	"gorm.io/gorm"

	"dash/internal/http/api/admin/nodes/id/virt/capabilities"
	"dash/internal/http/api/admin/nodes/id/virt/vms"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/infra"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
)

type handler struct{ node *nodestore.Store }

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	h := &handler{node: node}
	r := routes.NewBlueprint()
	virtRoute(r, h)
	r.Include("/capabilities", capabilities.Router(node, sessions))
	r.Include("/vms", vms.Router(node, sessions))
	return r
}

func virtRoute(r *routes.Blueprint, h *handler) {
	r.Get("", "Get node VM snapshot", h.virtHandler)
}

func (h *handler) virtHandler(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid node id")
		return
	}
	view, err := infra.WithPGReadTimeout(r.Context(), func(ctx context.Context) (nodestore.VirtView, error) { return h.node.Virt(ctx, id) })
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httperr.Write(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "virt_unavailable", "VM snapshot storage unavailable")
		return
	}
	response.WriteJSON(w, http.StatusOK, view)
}
