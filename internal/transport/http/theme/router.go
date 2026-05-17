package theme

import (
	"log/slog"
	"net/http"

	"dash/internal/infra"
	systemstore "dash/internal/store/system"
	themefs "dash/internal/theme"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	activeStore themefs.ActiveStore
	themes      *themefs.Store
	logger      *slog.Logger
}

// Router returns public theme routes.
func Router(st *systemstore.Store, themes *themefs.Store) *routes.Blueprint {
	h := &handler{
		activeStore: st,
		themes:      themes,
		logger:      infra.SlogWithModule("theme"),
	}

	r := routes.NewBlueprint(routes.DefaultTags("theme"))
	activeCSSRoute(r, h, http.MethodGet)
	activeCSSRoute(r, h, http.MethodHead)
	activeManifestRoute(r, h, http.MethodGet)
	activeManifestRoute(r, h, http.MethodHead)
	previewRoute(r, h, http.MethodGet)
	previewRoute(r, h, http.MethodHead)
	return r
}
