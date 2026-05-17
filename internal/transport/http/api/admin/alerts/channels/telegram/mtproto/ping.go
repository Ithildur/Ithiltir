package mtproto

import (
	"errors"
	"net/http"

	"dash/internal/notify"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

type pingInput struct {
	ChannelID int64 `json:"channel_id"`
}

type pingView struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

func pingRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/ping",
		"Ping MTProto session",
		routes.Func(h.pingHandler),
	)
}

func (h *handler) pingHandler(w http.ResponseWriter, r *http.Request) {
	var in pingInput
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

	if cfg.Session == "" {
		response.WriteJSON(w, http.StatusOK, pingView{
			Valid:  false,
			Reason: "not_logged_in",
		})
		return
	}

	if err := notify.PingSession(r.Context(), cfg.APIID, cfg.APIHash, cfg.Session); err != nil {
		response.WriteJSON(w, http.StatusOK, pingView{
			Valid:  false,
			Reason: "invalid_session",
		})
		return
	}

	response.WriteJSON(w, http.StatusOK, pingView{
		Valid: true,
	})
}
