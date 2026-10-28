package virtid

import (
	"errors"
	"net/http"

	"dash/internal/config"
	"dash/internal/http/api/virt/id/vms"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/infra"
	"dash/internal/nodesession"
	"dash/internal/store"
	"dash/internal/store/frontcache"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
	"gorm.io/gorm"
)

type handler struct{ store *store.Stores }

func Router(st *store.Stores, sessions *nodesession.Hub) *routes.Blueprint {
	h := &handler{store: st}
	r := routes.NewBlueprint(routes.DefaultMiddleware(h.requireAccess))
	virtRoute(r, h)
	r.Include("/vms", vms.Router(st.Node, sessions))
	return r
}

func virtRoute(r *routes.Blueprint, h *handler) {
	r.Get("", "Get visible node VM snapshot", h.virtHandler)
}

func (h *handler) virtHandler(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid node id")
		return
	}
	view, err := h.store.Node.Virt(r.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httperr.Write(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "virt_unavailable", "VM snapshot unavailable")
		return
	}
	response.WriteJSON(w, http.StatusOK, view)
}

func (h *handler) requireAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routes.Authenticated(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		settings, err := infra.WithPGReadTimeout(r.Context(), h.store.System.GetSettings)
		if err != nil {
			httperr.Write(w, 503, "virt_unavailable", "VM access settings unavailable")
			return
		}
		if !settings.PVEGuestVisible {
			httperr.Write(w, 403, "forbidden", "VM guest access denied")
			return
		}
		id, err := request.ParseIDInt64(r.PathValue("id"))
		if err != nil {
			httperr.Write(w, 400, "invalid_id", "invalid node id")
			return
		}
		visible, err := h.store.Front.EnsureGuestVisible(r.Context(), id, frontcache.GuestVisibilityOptions{
			CacheTimeout: config.RedisFetchTimeout,
			BuildTimeout: config.PGReadTimeout,
		})
		if err != nil {
			httperr.Write(w, 503, "virt_unavailable", "VM access unavailable")
			return
		}
		if !visible {
			httperr.Write(w, 404, "not_found", "node not found")
			return
		}
		next.ServeHTTP(w, r)
	})
}
