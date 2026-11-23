package metricid

import (
	"context"
	"net/http"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/store/frontcache"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	front         *frontcache.Store
	staleAfterSec int
}

func Router(front *frontcache.Store, staleAfterSec int) *routes.Blueprint {
	r := routes.NewBlueprint()
	nodeRoute(r, &handler{front: front, staleAfterSec: staleAfterSec})
	return r
}

func nodeRoute(r *routes.Blueprint, h *handler) {
	r.Get("", "Get front node metrics", h.nodeHandler)
}

func (h *handler) nodeHandler(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid node id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), config.PGReadTimeout)
	defer cancel()
	node, err := h.front.FetchFrontNode(ctx, id, h.staleAfterSec, routes.Authenticated(ctx))
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	if node == nil {
		httperr.TryWrite(w, httperr.NotFound(nil))
		return
	}
	response.WriteJSON(w, http.StatusOK, node)
}
