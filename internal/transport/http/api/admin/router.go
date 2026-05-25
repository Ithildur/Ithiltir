package admin

import (
	"dash/internal/config"
	"dash/internal/store"
	themefs "dash/internal/theme"
	trafficjob "dash/internal/traffic"
	adminalerts "dash/internal/transport/http/api/admin/alerts"
	admingroups "dash/internal/transport/http/api/admin/groups"
	adminnodes "dash/internal/transport/http/api/admin/nodes"
	adminsystem "dash/internal/transport/http/api/admin/system"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin routes.
func Router(st *store.Stores, cfg *config.Config, themes *themefs.Store, rebuild *trafficjob.RebuildRunner) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/groups", admingroups.Router(st.Node))
	r.Include("/nodes", adminnodes.Router(st.Node, rebuild, cfg))
	r.Include("/alerts", adminalerts.Router(st, cfg.App.EffectiveLanguage()))
	r.Include("/system", adminsystem.Router(st, themes))
	return r
}
