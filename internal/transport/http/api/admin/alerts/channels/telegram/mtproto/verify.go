package mtproto

import (
	"errors"
	"net/http"
	"strings"

	"dash/internal/notify"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type verifyInput struct {
	LoginID string `json:"login_id"`
	Code    string `json:"code"`
}

type verifyView struct {
	PasswordRequired bool `json:"password_required"`
}

func verifyRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/verify",
		"Verify MTProto code",
		routes.Func(h.verifyHandler),
	)
}

func (h *handler) verifyHandler(w http.ResponseWriter, r *http.Request) {
	var in verifyInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	in.LoginID = strings.TrimSpace(in.LoginID)
	in.Code = strings.TrimSpace(in.Code)
	if in.LoginID == "" || in.Code == "" {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "login_id and code are required")
		return
	}

	state, err := loadLoginState(r.Context(), h.login, in.LoginID)
	if err != nil {
		if errors.Is(err, errLoginNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "login_id not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "login_state_error", "login state unavailable")
		return
	}

	sessionText, passwordRequired, err := notify.VerifyCode(r.Context(), state.Auth, in.Code)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "notify_error", "failed to verify code")
		return
	}

	if passwordRequired {
		state.Auth.Session = sessionText
		if err := saveLoginState(r.Context(), h.login, in.LoginID, state); err != nil {
			httperr.Write(w, http.StatusServiceUnavailable, "login_state_error", "login state unavailable")
			return
		}
		response.WriteJSON(w, http.StatusOK, verifyView{
			PasswordRequired: true,
		})
		return
	}

	if err := updateSession(r.Context(), h.alert, state.ChannelID, state.ChannelRevision, sessionText); err != nil {
		writeSessionUpdateError(w, err)
		return
	}
	h.clearLoginState(r.Context(), in.LoginID)
	w.WriteHeader(http.StatusNoContent)
}
