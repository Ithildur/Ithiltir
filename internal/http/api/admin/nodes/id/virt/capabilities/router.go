package capabilities

import (
	"net/http"

	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
)

type handler struct {
	node     *nodestore.Store
	sessions *nodesession.Hub
}

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	h := &handler{node: node, sessions: sessions}
	r := routes.NewBlueprint()
	capabilitiesRoute(r, h)
	return r
}
func capabilitiesRoute(r *routes.Blueprint, h *handler) {
	r.Get("", "Get node VM query capabilities", h.capabilitiesHandler)
}
func (h *handler) capabilitiesHandler(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, 400, "invalid_id", "invalid node id")
		return
	}
	exists, err := h.node.NodeExists(r.Context(), id)
	if err != nil {
		httperr.Write(w, 503, "virt_unavailable", "VM query unavailable")
		return
	}
	if !exists {
		httperr.Write(w, 404, "not_found", "node not found")
		return
	}
	caps := h.sessions.Capabilities(r.Context(), id)
	response.WriteJSON(w, http.StatusOK, caps)
}
