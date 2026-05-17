package themes

import (
	"dash/internal/infra"
	systemstore "dash/internal/store/system"
	themefs "dash/internal/theme"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

type handler struct {
	store  *systemstore.Store
	themes *themefs.Store
	logger *kitlog.Helper
}

// Router returns admin/system/themes routes.
func Router(st *systemstore.Store, themeStore *themefs.Store) *routes.Blueprint {
	h := &handler{
		store:  st,
		themes: themeStore,
		logger: infra.WithModule("theme"),
	}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "system"),
	)
	listRoute(r, h)
	uploadRoute(r, h)
	applyRoute(r, h)
	deleteRoute(r, h)
	return r
}
