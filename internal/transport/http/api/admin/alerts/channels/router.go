package channels

import (
	alertstore "dash/internal/store/alert"
	"dash/internal/store/mtlogin"
	"dash/internal/transport/http/api/admin/alerts/channels/telegram"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	store    *alertstore.Store
	language string
}

// Router returns admin/alerts/channels routes.
func Router(st *alertstore.Store, login *mtlogin.Store, language string) *routes.Blueprint {
	h := &handler{store: st, language: language}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "alerts"),
	)
	listRoute(r, h)
	detailRoute(r, h)
	createRoute(r, h)
	replaceRoute(r, h)
	enabledRoute(r, h)
	deleteRoute(r, h)
	testMessageRoute(r, h)
	r.Include("/telegram", telegram.Router(st, login))
	return r
}
