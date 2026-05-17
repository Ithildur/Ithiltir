package mtproto

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"dash/internal/config"
	"dash/internal/notify"
	"dash/internal/store/mtlogin"
	ctxutil "github.com/Ithildur/EiluneKit/contextutil"
)

const loginTTL = 10 * time.Minute

var errLoginNotFound = errors.New("login not found")

func saveLoginState(ctx context.Context, st *mtlogin.Store, loginID string, state notify.MTProtoLoginState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = ctxutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.SetMTProtoLogin(c, loginID, payload, loginTTL)
	})
	return err
}

func loadLoginState(ctx context.Context, st *mtlogin.Store, loginID string) (notify.MTProtoLoginState, error) {
	var state notify.MTProtoLoginState
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

func deleteLoginState(ctx context.Context, st *mtlogin.Store, loginID string) {
	_, _ = ctxutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.DeleteMTProtoLogin(c, loginID)
	})
}
