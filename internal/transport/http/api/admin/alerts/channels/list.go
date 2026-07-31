package channels

import (
	"context"
	"net/http"

	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List alert channels",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {

	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]alertstore.ChannelDelivery, error) {
		return h.store.ListChannelDeliveries(c)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channels")
		return
	}

	out := make([]channelView, 0, len(items))
	for _, item := range items {
		out = append(out, viewFromDelivery(item))
	}

	response.WriteJSON(w, http.StatusOK, out)
}
