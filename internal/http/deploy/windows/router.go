package windows

import (
	"net/http"

	"github.com/Ithildur/EiluneKit/http/routes"
)

func Router(script http.Handler) *routes.Blueprint {
	r := routes.NewBlueprint()
	installRoute(r, script)
	return r
}

func installRoute(r *routes.Blueprint, script http.Handler) {
	r.Get("/install.ps1", "Download Windows install script", script.ServeHTTP)
	r.Handle(http.MethodHead, "/install.ps1", "Read Windows install script headers", script.ServeHTTP)
}
