package telegram

import (
	"dash/internal/http/api/admin/alerts/channels/telegram/mtproto"
	alertstore "dash/internal/store/alert"
	"dash/internal/store/mtlogin"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns admin/alerts/channels/telegram routes.
func Router(alert *alertstore.Store, login *mtlogin.Store) *routes.Blueprint {
	r := routes.NewBlueprint()
	r.Include("/mtproto", mtproto.Router(alert, login))
	return r
}
