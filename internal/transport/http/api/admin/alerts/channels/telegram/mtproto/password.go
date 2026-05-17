package mtproto

import (
	"errors"
	"net/http"
	"strings"

	"dash/internal/notify"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/routes"

	"github.com/gotd/td/telegram/auth"
	"gorm.io/gorm"
)

type passwordInput struct {
	LoginID  string `json:"login_id"`
	Password string `json:"password"`
}

func passwordRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/password",
		"Submit MTProto password",
		routes.Func(h.passwordHandler),
	)
}

func (h *handler) passwordHandler(w http.ResponseWriter, r *http.Request) {
	var in passwordInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	in.LoginID = strings.TrimSpace(in.LoginID)
	in.Password = strings.TrimSpace(in.Password)
	if in.LoginID == "" || in.Password == "" {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "login_id and password are required")
		return
	}

	state, err := loadLoginState(r.Context(), h.login, in.LoginID)
	if err != nil {
		if errors.Is(err, errLoginNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "login_id not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "redis_error", "state unavailable")
		return
	}

	sessionText, err := notify.SubmitPassword(r.Context(), state, in.Password)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordInvalid) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "password is invalid")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "notify_error", "failed to verify password")
		return
	}

	if err := updateSession(r.Context(), h.alert, state.ChannelID, sessionText); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		if errors.Is(err, errInvalidChannel) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid mtproto channel")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to save session")
		return
	}
	deleteLoginState(r.Context(), h.login, in.LoginID)
	w.WriteHeader(http.StatusNoContent)
}
