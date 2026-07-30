package channels

import (
	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/mtlogin"
	"dash/internal/transport/http/api/admin/alerts/channels/telegram"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

type handler struct {
	store    *alertstore.Store
	language string
	logger   *kitlog.Helper
}

// Router returns admin/alerts/channels routes.
func Router(st *alertstore.Store, login *mtlogin.Store, language string) *routes.Blueprint {
	h := &handler{
		store:    st,
		language: language,
		logger:   infra.WithModule("alert_channels"),
	}

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
