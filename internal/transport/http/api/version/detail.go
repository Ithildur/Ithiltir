package version

import (
	"net/http"

	appversion "dash/internal/version"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type view struct {
	Version     string `json:"version"`
	NodeVersion string `json:"node_version"`
}

func detailRoute(r *routes.Blueprint) {
	r.Get(
		"/",
		"Get bundled versions",
		routes.Func(detailHandler),
		routes.Tags("version"),
	)
}

func detailHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	response.WriteJSON(w, http.StatusOK, view{
		Version:     appversion.CurrentString(),
		NodeVersion: appversion.BundledNodeString(),
	})
}
