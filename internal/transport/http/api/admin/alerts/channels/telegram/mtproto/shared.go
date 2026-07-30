package mtproto

import (
	"errors"
	"net/http"

	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"

	"gorm.io/gorm"
)

func writeSessionUpdateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
	case errors.Is(err, errInvalidChannel):
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid mtproto channel")
	case errors.Is(err, alertstore.ErrChannelVersionStale):
		httperr.Write(w, http.StatusConflict, "channel_changed", "channel changed; restart login")
	default:
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to save session")
	}
}
