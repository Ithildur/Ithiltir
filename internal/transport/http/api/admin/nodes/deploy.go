package nodes

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type deployView struct {
	Scripts map[string]deployScript `json:"scripts"`
}

type deployScript struct {
	URL           string `json:"url"`
	CommandPrefix string `json:"command_prefix"`
}

func deployRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/deploy",
		"Get deploy script info",
		routes.Func(h.deployHandler),
	)
}

// deploy returns node deploy/install script URLs and command prefixes.
// The returned command prefixes intentionally exclude the per-node secret.
func (h *handler) deployHandler(w http.ResponseWriter, r *http.Request) {
	cfg := h.config
	if cfg == nil {
		httperr.Write(w, http.StatusInternalServerError, "config_unavailable", "config is not configured")
		return
	}

	publicScheme := cfg.App.PublicURLScheme
	publicHost := cfg.App.PublicURLHost
	publicBasePath := cfg.App.PublicURLBasePath

	commandHost, commandPort := splitPublicHost(publicHost)
	commandArgs := joinHostPortArgs(commandHost, commandPort)

	linuxURL := scriptURL(publicScheme, publicHost, publicBasePath, "linux", "install.sh")
	macURL := scriptURL(publicScheme, publicHost, publicBasePath, "macos", "install.sh")
	winURL := scriptURL(publicScheme, publicHost, publicBasePath, "windows", "install.ps1")

	scripts := map[string]deployScript{
		"linux": {
			URL:           linuxURL,
			CommandPrefix: fmt.Sprintf("curl -fsSL %s | sudo bash -s -- %s ", linuxURL, commandArgs),
		},
		"macos": {
			URL:           macURL,
			CommandPrefix: fmt.Sprintf("curl -fsSL %s | sudo bash -s -- %s ", macURL, commandArgs),
		},
		"windows": {
			URL:           winURL,
			CommandPrefix: fmt.Sprintf("iwr -UseBasicParsing %s -OutFile install.ps1; powershell -ExecutionPolicy Bypass -File .\\install.ps1 %s ", winURL, commandArgs),
		},
	}

	resp := deployView{Scripts: scripts}
	response.WriteJSON(w, http.StatusOK, resp)
}

func scriptURL(scheme, host, basePath, platform, file string) string {
	fullPath := path.Join("/", strings.TrimPrefix(basePath, "/"), "deploy", platform, file)
	return (&url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   fullPath,
	}).String()
}

func splitPublicHost(hostport string) (string, string) {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return "", ""
	}
	if strings.HasPrefix(hostport, "[") {
		if host, port, err := net.SplitHostPort(hostport); err == nil {
			return host, port
		}
		if strings.HasSuffix(hostport, "]") {
			return strings.TrimSuffix(strings.TrimPrefix(hostport, "["), "]"), ""
		}
	}
	if host, port, err := net.SplitHostPort(hostport); err == nil {
		return host, port
	}
	return hostport, ""
}

func joinHostPortArgs(host, port string) string {
	if host == "" {
		return ""
	}
	if port == "" {
		return host
	}
	return host + " " + port
}
