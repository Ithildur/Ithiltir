package groups

import (
	"context"
	"net/http"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List groups",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	groups, err := loadGroups(r.Context(), h.store)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch groups")
		return
	}

	if groups == nil {
		groups = make([]nodestore.GroupItem, 0)
	}

	response.WriteJSON(w, http.StatusOK, groups)
}

func loadGroups(ctx context.Context, st *nodestore.Store) ([]nodestore.GroupItem, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) ([]nodestore.GroupItem, error) {
		return st.GroupsWithCounts(c)
	})
}
