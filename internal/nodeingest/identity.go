package nodeingest

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"dash/internal/infra"
	"dash/internal/model"
)

type Identity struct {
	InstallID string `json:"install_id"`
	Created   bool   `json:"created"`
}

func (h *Receiver) Authenticate(ctx context.Context, secret string) (model.Server, error) {
	if secret == "" {
		return model.Server{}, unauthorized(nil)
	}
	server, err := h.node.GetServerBySecret(ctx, secret)
	if err == nil {
		return server, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Server{}, unauthorized(err)
	}
	infra.WithModule("node").Error(ctx, "node auth lookup failed", err)
	return model.Server{}, unavailable(err)
}

func (h *Receiver) Identity(ctx context.Context) (Identity, error) {
	id, created, err := h.serverID.GetOrCreate()
	if err != nil {
		infra.WithModule("node").Error(ctx, "load server identity failed", err)
		return Identity{}, unavailable(err)
	}
	return Identity{InstallID: id, Created: created}, nil
}
