package node

import (
	"net/http"

	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/infra"
)

func (h *handler) identityRoute(r *routes.Blueprint) {
	r.Post("/identity", "Get node server identity", h.identityHandler, routes.Use(ingestMiddleware()...), routes.Tags("node"))
}
func (h *handler) identityHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	logger := infra.WithModule("node")
	if _, _, err := h.authenticate(r.Context(), r); err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	identity, err := h.ingest.Identity(r.Context())
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, identity)
}
