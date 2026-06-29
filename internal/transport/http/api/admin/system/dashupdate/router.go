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
func Router(runner *updater.Runner) *routes.Blueprint {
	if runner == nil {
		runner = updater.NewRunner()
	}
	h := &handler{
		client: releaseNotesClient(),
		runner: runner,
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

func releaseNotesClient() *http.Client {
	return &http.Client{
		Timeout:       releaseNotesTimeout,
		CheckRedirect: releaseNotesRedirect,
	}
}

func releaseNotesRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return http.ErrUseLastResponse
	}
	if req.URL.Scheme != "https" || req.URL.Hostname() != "www.ithiltir.dev" {
		return http.ErrUseLastResponse
	}
	return nil
}
