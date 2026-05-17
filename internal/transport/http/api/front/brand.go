package front

import (
	"context"
	"net/http"

	"dash/internal/infra"
	systemstore "dash/internal/store/system"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func (h *handler) brandRoute(r *routes.Blueprint) {
	r.Get(
		"/brand",
		"Get front brand settings",
		routes.Func(h.brandHandler),
		routes.Tags("front"),
	)
}

func (h *handler) brandHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	brand, err := h.loadBrand(r.Context())
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, brand)
}

func (h *handler) loadBrand(ctx context.Context) (systemstore.SiteBrand, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (systemstore.SiteBrand, error) {
		return h.system.GetSiteBrand(c)
	})
}
