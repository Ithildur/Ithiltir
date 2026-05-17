package mtproto

import (
	alertstore "dash/internal/store/alert"
	"dash/internal/store/mtlogin"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	alert *alertstore.Store
	login *mtlogin.Store
}

// Router returns admin/alerts/channels/telegram/mtproto routes.
func Router(alert *alertstore.Store, login *mtlogin.Store) *routes.Blueprint {
	h := &handler{alert: alert, login: login}

	r := routes.NewBlueprint(
		routes.DefaultMiddleware(middleware.RequireJSONBody),
	)
	codeRoute(r, h)
	verifyRoute(r, h)
	passwordRoute(r, h)
	pingRoute(r, h)
	return r
}
