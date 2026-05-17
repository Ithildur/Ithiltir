package traffic

import (
	"context"
	"net/http"

	"dash/internal/infra"
	trafficstore "dash/internal/store/traffic"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type ifaceView struct {
	Name string `json:"name"`
}

func (h *handler) ifacesRoute(r *routes.Blueprint) {
	r.Get(
		"/ifaces",
		"List traffic interfaces",
		routes.Func(h.ifacesHandler),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) ifacesHandler(w http.ResponseWriter, r *http.Request) {
	serverID, err := parseServerID(r.URL.Query())
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}
	allowed, err := h.canReadTraffic(r.Context(), r, serverID)
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}
	if !allowed {
		httperr.TryWrite(w, httperr.Forbidden(errTrafficGuestForbidden))
		return
	}

	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]trafficstore.TrafficIface, error) {
		return h.traffic.ListTrafficIfaces(c, serverID)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch traffic interfaces")
		return
	}
	out := make([]ifaceView, 0, len(items))
	for _, item := range items {
		out = append(out, ifaceView{Name: item.Name})
	}
	response.WriteJSON(w, http.StatusOK, out)
}
