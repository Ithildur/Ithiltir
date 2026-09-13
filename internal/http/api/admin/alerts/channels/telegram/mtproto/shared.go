package mtproto

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"dash/internal/http/httperr"
	alertstore "dash/internal/store/alert"

	"gorm.io/gorm"
)

func writeSessionUpdateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
	case errors.Is(err, alertstore.ErrInvalidMTProtoChannel):
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid mtproto channel")
	case errors.Is(err, alertstore.ErrChannelVersionStale):
		httperr.Write(w, http.StatusConflict, "channel_changed", "channel changed; restart login")
	default:
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to save session")
	}
}

func (h *handler) clearLoginState(ctx context.Context, loginID string) {
	if err := h.login.Delete(ctx, loginID); err != nil {
		h.logger.Warn(
			"failed to clear completed MTProto login state",
			err,
			slog.String("login_id", loginID),
		)
	}
}
