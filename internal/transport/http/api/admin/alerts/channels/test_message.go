package channels

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
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

const testMessageSendTimeout = 10 * time.Second

func testMessageRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/{id}/test",
		"Test alert channel",
		routes.Func(h.testMessageHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) testMessageHandler(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := request.ParseIDInt64(rawID)
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
	sendCtx, cancelSend := context.WithTimeout(r.Context(), testMessageSendTimeout)
	defer cancelSend()
	language := notify.EffectiveLanguage(item.Type, item.Config, h.language)

	var messages []notify.Message
	if item.Type == model.NotifyTypeTelegram {
		session, isMTProto, err := telegram.SessionFromConfig(item.Config)
		if err != nil {
			if recordErr := h.recordTestFailure(r.Context(), item, err); recordErr != nil {
				h.writeTestFailureStoreError(w, recordErr)
				return
			}
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid config")
			return
		}
		if isMTProto && strings.TrimSpace(session) == "" {
			if recordErr := h.recordTestFailure(r.Context(), item, notify.ErrInvalidConfig); recordErr != nil {
				h.writeTestFailureStoreError(w, recordErr)
				return
			}
			httperr.Write(w, http.StatusBadRequest, "not_logged_in", "mtproto not logged in")
			return
		}
		if !isMTProto {
			messages = notify.TelegramBotExampleMessages(language)
		}
	}

	if messages == nil {
		msg := notify.DefaultTestMessage(language)
		if title := strings.TrimSpace(in.Title); title != "" {
			msg.Title = title
		}
		if body := strings.TrimSpace(in.Message); body != "" {
			msg.Body = body
		}
		messages = []notify.Message{msg}
	}

	for _, msg := range messages {
		if err := notify.Send(sendCtx, item, msg); err != nil {
			h.writeTestSendError(w, r, item, err)
			return
		}
	}

	if err := h.recoverTestedChannel(r.Context(), item.ID, item.Revision); err != nil {
		h.logger.Warn("recover tested notification channel failed", err)
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "test message sent but channel recovery failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) writeTestSendError(w http.ResponseWriter, r *http.Request, item *model.NotifyChannel, err error) {
	if recordErr := h.recordTestFailure(r.Context(), item, err); recordErr != nil {
		h.writeTestFailureStoreError(w, recordErr)
		return
	}
	if errors.Is(err, notify.ErrInvalidConfig) {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}
	httperr.Write(w, http.StatusServiceUnavailable, "notify_error", "failed to send test message")
}

func (h *handler) recoverTestedChannel(ctx context.Context, id, revision int64) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.RecoverChannelNotifications(c, id, revision, time.Now().UTC())
	})
	if err != nil {
		return fmt.Errorf("recover tested notification channel: %w", err)
	}
	return nil
}

func (h *handler) recordTestFailure(
	ctx context.Context,
	channel *model.NotifyChannel,
	err error,
) error {
	if channel == nil || err == nil {
		return nil
	}
	delivery := notify.Classify(err)
	_, storeErr := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.RecordChannelFailure(c, alertstore.ChannelFailure{
			ID:        channel.ID,
			Revision:  channel.Revision,
			Code:      delivery.Code,
			LastError: notify.ErrorSummary(err),
			FailedAt:  time.Now().UTC(),
		})
	})
	if storeErr != nil {
		return fmt.Errorf("record tested notification channel failure: %w", storeErr)
	}
	return nil
}

func (h *handler) writeTestFailureStoreError(w http.ResponseWriter, err error) {
	h.logger.Warn("record tested notification channel failure failed", err)
	httperr.Write(w, http.StatusServiceUnavailable, "db_error", "test message failed and channel status update failed")
}
