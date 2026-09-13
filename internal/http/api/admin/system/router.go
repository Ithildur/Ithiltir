package system

import (
	updater "dash/internal/dashupdate"
	"dash/internal/http/api/admin/system/dashupdate"
	"dash/internal/http/api/admin/system/settings"
	"dash/internal/http/api/admin/system/themes"
	"dash/internal/store"
	themefs "dash/internal/theme"
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
