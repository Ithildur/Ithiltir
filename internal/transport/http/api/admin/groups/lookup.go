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

func lookupRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/map",
		"Get group lookup",
		routes.Func(h.lookupHandler),
	)
}

func (h *handler) lookupHandler(w http.ResponseWriter, r *http.Request) {
	groupLookup, err := loadLookup(r.Context(), h.store)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch group map")
		return
	}

	if groupLookup == nil {
		groupLookup = make(map[int64]string)
	}

	response.WriteJSON(w, http.StatusOK, groupLookup)
}

func loadLookup(ctx context.Context, st *nodestore.Store) (map[int64]string, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (map[int64]string, error) {
		return st.GroupLookup(c)
	})
}
