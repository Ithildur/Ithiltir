package alerts

import (
	"dash/internal/http/api/admin/alerts/channels"
	"dash/internal/http/api/admin/alerts/events"
	"dash/internal/http/api/admin/alerts/mounts"
	"dash/internal/http/api/admin/alerts/rules"
	"dash/internal/http/api/admin/alerts/settings"
	"dash/internal/store"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin/alerts routes.
func Router(st *store.Stores, language string) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/events", events.Router(st.Alert, st.Node))
	r.Include("/rules", rules.Router(st.Alert))
	r.Include("/mounts", mounts.Router(st.Alert, st.Node))
	r.Include("/settings", settings.Router(st.Alert))
	r.Include("/channels", channels.Router(st.Alert, st.MTLogin, language))
	return r
}
