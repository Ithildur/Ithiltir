package groups

import (
	"net/http"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func lookupRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/map",
		"Get group lookup",
		h.lookupHandler,
	)
}

func (h *handler) lookupHandler(w http.ResponseWriter, r *http.Request) {
	groupLookup, err := infra.WithPGReadTimeout(r.Context(), h.store.GroupLookup)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch group map")
		return
	}

	if groupLookup == nil {
		groupLookup = make(map[int64]string)
	}

	response.WriteJSON(w, http.StatusOK, groupLookup)
}
