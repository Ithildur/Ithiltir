package system

import (
	"dash/internal/store"
	themefs "dash/internal/theme"
	"dash/internal/transport/http/api/admin/system/settings"
	"dash/internal/transport/http/api/admin/system/themes"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin/system routes.
func Router(st *store.Stores, themeStore *themefs.Store) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/settings", settings.Router(st.Metric, st.System))
	r.Include("/themes", themes.Router(st.System, themeStore))
	return r
}
