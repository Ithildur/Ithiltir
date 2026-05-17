package front

import (
	"context"
	"net/http"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func (h *handler) groupsRoute(r *routes.Blueprint) {
	r.Get(
		"/groups",
		"List front group nodes",
		routes.Func(h.groupsHandler),
		routes.Tags("front"),
		routes.Auth(routes.AuthOptional),
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
	return infra.WithPGReadTimeout(ctx, func(c context.Context) ([]nodestore.GroupNodes, error) {
		return h.node.GroupNodes(c, guestVisibleOnly)
	})
}
