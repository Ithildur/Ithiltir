package metrics

import (
	"dash/internal/store"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns metrics routes.
func Router(st *store.Stores, auth *authjwt.Manager) *routes.Blueprint {
	h := newHandler(st.Metric, st.Front, auth)
	r := routes.NewBlueprint()
	h.historyRoute(r)
	h.onlineRoute(r)
	return r
}
