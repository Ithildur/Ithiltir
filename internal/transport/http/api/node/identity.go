package node

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
	kitlog "github.com/Ithildur/EiluneKit/logging"
	"gorm.io/gorm"
)

type identityView struct {
	InstallID string `json:"install_id"`
	Created   bool   `json:"created"`
}

func (h *handler) identityRoute(r *routes.Blueprint) {
	r.Post(
		"/identity",
		"Get node server identity",
		routes.Func(h.identityHandler),
		routes.Tags("node"),
	)
}

func (h *handler) identityHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer r.Body.Close()
	logger := infra.WithModule("node")

	if err := h.validateIdentity(ctx, r, logger); err != nil {
		httperr.WriteOrInternal(w, logger, err)
		return
	}

	identity, err := h.loadIdentity()
	if err != nil {
		logger.Error("load server identity failed", err)
		httperr.WriteOrInternal(w, logger, httperr.ServiceUnavailable(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, identity)
}

func (h *handler) validateIdentity(ctx context.Context, r *http.Request, logger *kitlog.Helper) error {
	secret, ok := readSecret(r)
	if !ok {
		return httperr.Unauthorized(nil)
	}
	if _, err := h.loadServer(ctx, secret); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httperr.Unauthorized(err)
		}
		logger.Error("redis load server failed", err)
		return httperr.ServiceUnavailable(err)
	}
	return nil
}

func (h *handler) loadIdentity() (identityView, error) {
	id, created, err := h.serverID.GetOrCreate()
	if err != nil {
		return identityView{}, err
	}
	return identityView{InstallID: id, Created: created}, nil
}
