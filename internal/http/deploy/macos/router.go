package macos

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
	r.Get("/install.sh", "Download macOS install script", script.ServeHTTP)
	r.Handle(http.MethodHead, "/install.sh", "Read macOS install script headers", script.ServeHTTP)
}
