package deploy

import (
	"net/http"

	"dash/internal/http/deploy/linux"
	"dash/internal/http/deploy/macos"
	"dash/internal/http/deploy/windows"
	"github.com/Ithildur/EiluneKit/http/routes"
)

// Router returns the public install script routes.
func Router(linuxScript, macosScript, windowsScript http.Handler) *routes.Blueprint {
	r := routes.NewBlueprint(routes.DefaultTags("deploy"))
	r.Include("/linux", linux.Router(linuxScript))
	r.Include("/macos", macos.Router(macosScript))
	r.Include("/windows", windows.Router(windowsScript))
	return r
}
