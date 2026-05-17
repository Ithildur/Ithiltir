package mtproto

import (
	"errors"
	"net/http"

	"dash/internal/notify"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type codeInput struct {
	ChannelID int64 `json:"channel_id"`
}

type codeView struct {
	LoginID string `json:"login_id"`
	Timeout int    `json:"timeout,omitempty"`
}

func codeRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/code",
		"Send MTProto login code",
		routes.Func(h.codeHandler),
	)
}

func (h *handler) codeHandler(w http.ResponseWriter, r *http.Request) {
	var in codeInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	if in.ChannelID <= 0 {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "channel_id is required")
		return
	}

	cfg, err := loadChannelConfig(r.Context(), h.alert, in.ChannelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		if errors.Is(err, errInvalidChannel) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid mtproto channel")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channel")
		return
	}

	state, timeout, err := notify.StartLogin(r.Context(), cfg.APIID, cfg.APIHash, cfg.Phone)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "notify_error", "failed to send code")
		return
	}
	state.ChannelID = in.ChannelID

	loginID := uuid.NewString()
	if err := saveLoginState(r.Context(), h.login, loginID, state); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "redis_error", "state unavailable")
		return
	}

	response.WriteJSON(w, http.StatusOK, codeView{
		LoginID: loginID,
		Timeout: timeout,
	})
}
