package channels

import (
	"errors"
	"net/http"
	"strings"

	"dash/internal/model"
	"dash/internal/notify"
	"dash/internal/transport/http/api/admin/alerts/channels/telegram"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

type testMessageInput struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func testMessageRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/{id}/test",
		"Test alert channel",
		routes.Func(h.testMessageHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) testMessageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	var in testMessageInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	item, err := loadChannel(r.Context(), h.store, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channel")
		return
	}

	if item.Type == model.NotifyTypeTelegram {
		session, isMTProto, err := telegram.SessionFromConfig(item.Config)
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid config")
			return
		}
		if isMTProto && strings.TrimSpace(session) == "" {
			httperr.Write(w, http.StatusBadRequest, "not_logged_in", "mtproto not logged in")
			return
		}
		if !isMTProto {
			for _, msg := range notify.TelegramBotExampleMessages() {
				if err := notify.Send(r.Context(), item, msg); err != nil {
					if errors.Is(err, notify.ErrInvalidConfig) {
						httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
						return
					}
					httperr.Write(w, http.StatusServiceUnavailable, "notify_error", "failed to send test message")
					return
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	msg := notify.DefaultTestMessage()
	if title := strings.TrimSpace(in.Title); title != "" {
		msg.Title = title
	}
	if body := strings.TrimSpace(in.Message); body != "" {
		msg.Body = body
	}

	if err := notify.Send(r.Context(), item, msg); err != nil {
		if errors.Is(err, notify.ErrInvalidConfig) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "notify_error", "failed to send test message")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
