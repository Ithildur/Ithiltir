package admin

import (
	"dash/internal/config"
	updater "dash/internal/dashupdate"
	adminalerts "dash/internal/http/api/admin/alerts"
	admingroups "dash/internal/http/api/admin/groups"
	adminnodes "dash/internal/http/api/admin/nodes"
	adminsystem "dash/internal/http/api/admin/system"
	"dash/internal/store"
	themefs "dash/internal/theme"
	trafficjob "dash/internal/traffic"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin routes.
func Router(st *store.Stores, cfg *config.Config, themes *themefs.Store, rebuild *trafficjob.RebuildRunner, dashUpdate *updater.Runner) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/groups", admingroups.Router(st.Node))
	r.Include("/nodes", adminnodes.Router(st.Node, rebuild, cfg))
	r.Include("/alerts", adminalerts.Router(st, cfg.App.EffectiveLanguage()))
	r.Include("/system", adminsystem.Router(st, themes, dashUpdate))
	return r
}
