package nodes

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"dash/internal/infra"
	"dash/internal/nodeupdate"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	appversion "dash/internal/version"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

func upgradeRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/{id}/upgrade",
		"Upgrade node agent",
		routes.Func(h.upgradeHandler),
	)
}

func (h *handler) upgradeHandler(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	target := strings.TrimSpace(appversion.BundledNodeString())
	if target == "" || target == "unknown" {
		httperr.Write(w, http.StatusConflict, "node_version_unavailable", "bundled node version is unavailable")
		return
	}
	if err := appversion.ValidateNodeVersion(target); err != nil {
		httperr.Write(w, http.StatusConflict, "invalid_node_version", "bundled node version is invalid")
		return
	}

	platform, err := h.loadAgentPlatform(r.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch node")
		return
	}
	asset, err := nodeupdate.BundledAsset(h.config, platform.OS, platform.Arch)
	if err != nil {
		writeAssetError(w, err)
		return
	}
	update := nodestore.AgentUpdateTarget{
		Version: target,
		URL:     asset.URL,
		SHA256:  asset.SHA256,
		Size:    asset.Size,
	}

	h.store.RequestAgentUpdate(id, update)

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) loadAgentPlatform(ctx context.Context, id int64) (nodestore.AgentPlatform, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (nodestore.AgentPlatform, error) {
		return h.store.AgentPlatform(c, id)
	})
}

func writeAssetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, nodeupdate.ErrMissingPlatform):
		httperr.Write(w, http.StatusConflict, "node_platform_unknown", "node platform is unknown")
	case errors.Is(err, nodeupdate.ErrUnsupportedPlatform):
		httperr.Write(w, http.StatusConflict, "node_platform_unsupported", "node platform is unsupported")
	case errors.Is(err, nodeupdate.ErrAssetNotFound):
		httperr.Write(w, http.StatusConflict, "node_asset_missing", "node update asset is missing")
	default:
		httperr.Write(w, http.StatusServiceUnavailable, "node_asset_error", "failed to prepare node update asset")
	}
}
