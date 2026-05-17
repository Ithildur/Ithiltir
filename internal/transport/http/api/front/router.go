package front

import (
	"time"

	"dash/internal/store"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns front routes.
func Router(st *store.Stores, offlineThreshold time.Duration, auth *authjwt.Manager) *routes.Blueprint {
	h := newHandler(st.Front, st.Node, st.System, offlineThreshold, auth)
	r := routes.NewBlueprint()
	h.brandRoute(r)
	h.metricsRoute(r)
	h.groupsRoute(r)
	return r
}
