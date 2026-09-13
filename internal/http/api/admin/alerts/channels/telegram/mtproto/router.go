package mtproto

import (
	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/mtlogin"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"
)

type handler struct {
	alert  *alertstore.Store
	login  *mtlogin.Store
	logger *kitlog.Helper
}

// Router returns admin/alerts/channels/telegram/mtproto routes.
func Router(alert *alertstore.Store, login *mtlogin.Store) *routes.Blueprint {
	h := &handler{
		alert:  alert,
		login:  login,
		logger: infra.WithModule("admin.alerts.telegram.mtproto"),
	}

	r := routes.NewBlueprint(
		routes.DefaultMiddleware(middleware.RequireJSONBody),
	)
	codeRoute(r, h)
	verifyRoute(r, h)
	passwordRoute(r, h)
	pingRoute(r, h)
	return r
}
