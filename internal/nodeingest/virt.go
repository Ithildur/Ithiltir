package nodeingest

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"dash/internal/infra"
	"dash/internal/virt"
)

func (h *Receiver) Virt(ctx context.Context, id int64, secret string, snapshot virt.Snapshot, receivedAt time.Time) error {
	if err := snapshot.Validate(); err != nil || snapshot.CollectedAt.After(receivedAt.Add(5*time.Minute)) {
		return &Error{Code: "invalid_virt", Cause: err}
	}
	err := h.node.SaveVirt(ctx, id, secret, snapshot, receivedAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return unauthorized(err)
	}
	if err != nil {
		infra.WithModule("virt").Error(ctx, "VM snapshot persistence failed", err)
		return &Error{Code: "virt_unavailable", Cause: err}
	}
	return nil
}
