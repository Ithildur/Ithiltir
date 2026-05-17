package alerts

import (
	"dash/internal/store"
	"dash/internal/transport/http/api/admin/alerts/channels"
	"dash/internal/transport/http/api/admin/alerts/mounts"
	"dash/internal/transport/http/api/admin/alerts/rules"
	"dash/internal/transport/http/api/admin/alerts/settings"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin/alerts routes.
func Router(st *store.Stores) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/rules", rules.Router(st.Alert))
	r.Include("/mounts", mounts.Router(st.Alert, st.Node))
	r.Include("/settings", settings.Router(st.Alert))
	r.Include("/channels", channels.Router(st.Alert, st.MTLogin))
	return r
}
