package front

import (
	"net/http"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func (h *handler) brandRoute(r *routes.Blueprint) {
	r.Get(
		"/brand",
		"Get front brand settings",
		h.brandHandler,
		routes.Tags("front"),
	)
}

func (h *handler) brandHandler(w http.ResponseWriter, r *http.Request) {
	brand, err := infra.WithPGReadTimeout(r.Context(), h.system.GetSiteBrand)
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, brand)
}
