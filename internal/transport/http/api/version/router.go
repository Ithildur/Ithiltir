package version

import (
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns version routes.
func Router() *routes.Blueprint {
	r := routes.NewBlueprint()
	detailRoute(r)
	return r
}
