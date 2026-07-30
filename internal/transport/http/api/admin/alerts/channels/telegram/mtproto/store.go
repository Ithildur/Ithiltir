package mtproto

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"dash/internal/config"
	"dash/internal/notify"
	"dash/internal/store/mtlogin"
	ctxutil "github.com/Ithildur/EiluneKit/contextutil"
)

const loginTTL = 10 * time.Minute

var errLoginNotFound = errors.New("login not found")

type loginState struct {
	ChannelID       int64                    `json:"channel_id"`
	ChannelRevision int64                    `json:"channel_revision"`
	Auth            notify.MTProtoLoginState `json:"auth"`
}

func saveLoginState(ctx context.Context, st *mtlogin.Store, loginID string, state loginState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = ctxutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.SetMTProtoLogin(c, loginID, payload, loginTTL)
	})
	return err
}

func loadLoginState(ctx context.Context, st *mtlogin.Store, loginID string) (loginState, error) {
	var state loginState
	raw, err := ctxutil.WithTimeout(ctx, config.RedisFetchTimeout, func(c context.Context) ([]byte, error) {
		return st.GetMTProtoLogin(c, loginID)
	})
	if err != nil {
		return state, err
	}
	if raw == nil {
		return state, errLoginNotFound
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	return state, nil
}

func deleteLoginState(ctx context.Context, st *mtlogin.Store, loginID string) error {
	_, err := ctxutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.DeleteMTProtoLogin(c, loginID)
	})
	return err
}

func (h *handler) clearLoginState(ctx context.Context, loginID string) {
	if err := deleteLoginState(ctx, h.login, loginID); err != nil {
		h.logger.Warn(
			"failed to clear completed MTProto login state",
			err,
			slog.String("login_id", loginID),
		)
	}
}
