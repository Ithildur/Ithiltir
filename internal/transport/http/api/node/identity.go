package node

import (
	"net/http"

	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type identityView struct {
	InstallID string `json:"install_id"`
	Created   bool   `json:"created"`
}

func (h *handler) identityRoute(r *routes.Blueprint) {
	r.Post(
		"/identity",
		"Get node server identity",
		routes.Func(h.identityHandler),
		routes.Tags("node"),
	)
}

func (h *handler) identityHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()
	logger := infra.WithModule("node")

	if _, _, err := h.authenticate(ctx, r, logger); err != nil {
		h.writeError(w, r, logger, err)
		return
	}

	identity, err := h.loadIdentity()
	if err != nil {
		logger.Error("load server identity failed", err)
		h.writeError(w, r, logger, httperr.ServiceUnavailable(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, identity)
}

func (h *handler) loadIdentity() (identityView, error) {
	id, created, err := h.serverID.GetOrCreate()
	if err != nil {
		return identityView{}, err
	}
	return identityView{InstallID: id, Created: created}, nil
}
