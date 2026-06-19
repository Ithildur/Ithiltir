package dashupdate

import (
	"net/http"
	"time"

	updater "dash/internal/dashupdate"
	"github.com/Ithildur/EiluneKit/http/routes"
)

const (
	releaseNotesMaxBytes = 512 * 1024
	releaseNotesTimeout  = 8 * time.Second
)

type handler struct {
	client *http.Client
	runner *updater.Runner
}

// Router returns admin/system/dash-update routes.
func Router() *routes.Blueprint {
	h := &handler{
		client: &http.Client{Timeout: releaseNotesTimeout},
		runner: updater.NewRunner(),
	}

	r := routes.NewBlueprint(
		routes.DefaultTags("admin", "system"),
	)
	statusRoute(r, h)
	checkRoute(r, h)
	runRoute(r, h)
	releaseNotesRoute(r, h)
	return r
}
