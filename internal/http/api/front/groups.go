package front

import (
	"context"
	"net/http"

	"dash/internal/config"
	"dash/internal/http/httperr"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func (h *handler) groupsRoute(r *routes.Blueprint) {
	r.Get(
		"/groups",
		"List front group nodes",
		h.groupsHandler,
		routes.Tags("front"),
		routes.Auth(routes.AuthOptional),
		routes.Use(h.optionalBearer),
	)
}

func (h *handler) groupsHandler(w http.ResponseWriter, r *http.Request) {
	authorized := h.isAuthorized(r)
	groups, err := h.loadGroups(r.Context(), !authorized)
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}

	if groups == nil {
		groups = make([]nodestore.GroupNodes, 0)
	}

	response.WriteJSON(w, http.StatusOK, groups)
}

func (h *handler) loadGroups(ctx context.Context, guestVisibleOnly bool) ([]nodestore.GroupNodes, error) {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGReadTimeout)
	defer cancel()
	return h.node.GroupNodes(dbCtx, guestVisibleOnly)
}
