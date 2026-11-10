package vms

import (
	"net/http"

	vmid "dash/internal/http/api/virt/id/vms/id"
	"dash/internal/http/httperr"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	r := routes.NewBlueprint(routes.DefaultMiddleware(requireAdministrator))
	r.Include("/{vmid}", vmid.Router(node, sessions))
	return r
}

func requireAdministrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !routes.Authenticated(r.Context()) {
			httperr.Write(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
