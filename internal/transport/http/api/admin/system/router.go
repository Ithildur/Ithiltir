package system

import (
	updater "dash/internal/dashupdate"
	"dash/internal/store"
	themefs "dash/internal/theme"
	"dash/internal/transport/http/api/admin/system/dashupdate"
	"dash/internal/transport/http/api/admin/system/settings"
	"dash/internal/transport/http/api/admin/system/themes"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin/system routes.
func Router(st *store.Stores, themeStore *themefs.Store, dashUpdate *updater.Runner) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/dash-update", dashupdate.Router(dashUpdate))
	r.Include("/settings", settings.Router(st))
	r.Include("/themes", themes.Router(st.System, themeStore))
	return r
}
