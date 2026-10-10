package groups

import (
	"net/http"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List groups",
		h.listHandler,
	)
	r.Get(
		"",
		"List groups",
		h.listHandler,
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	groups, err := infra.WithPGReadTimeout(r.Context(), h.store.GroupsWithCounts)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch groups")
		return
	}

	if groups == nil {
		groups = make([]nodestore.GroupItem, 0)
	}

	response.WriteJSON(w, http.StatusOK, groups)
}
