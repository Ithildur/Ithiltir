package events

import (
	"context"
	"net/http"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type serversView struct {
	Items []serverView `json:"items"`
}

type serverView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func serversRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/servers",
		"List alert event filter servers",
		routes.Func(h.serversHandler),
	)
}

func (h *handler) serversHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	refs, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]nodestore.ServerRef, error) {
		return h.nodes.ServerRefs(c)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch alert event servers")
		return
	}

	response.WriteJSON(w, http.StatusOK, serversView{Items: serverViews(refs)})
}

func serverViews(refs []nodestore.ServerRef) []serverView {
	if len(refs) == 0 {
		return make([]serverView, 0)
	}
	out := make([]serverView, 0, len(refs))
	for _, ref := range refs {
		out = append(out, serverView{
			ID:   ref.ID,
			Name: ref.Name,
		})
	}
	return out
}
