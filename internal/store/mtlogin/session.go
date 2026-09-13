package mtlogin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"dash/internal/config"
	"dash/internal/notify"
	ctxutil "github.com/Ithildur/EiluneKit/contextutil"
)

const loginTTL = 10 * time.Minute

var ErrNotFound = errors.New("login not found")

type State struct {
	ChannelID       int64                    `json:"channel_id"`
	ChannelRevision int64                    `json:"channel_revision"`
	Auth            notify.MTProtoLoginState `json:"auth"`
}

func (s *Store) Save(ctx context.Context, loginID string, state State) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = ctxutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, s.SetMTProtoLogin(c, loginID, payload, loginTTL)
	})
	return err
}

func (s *Store) Get(ctx context.Context, loginID string) (State, error) {
	var state State
	raw, err := ctxutil.WithTimeout(ctx, config.RedisFetchTimeout, func(c context.Context) ([]byte, error) {
		return s.GetMTProtoLogin(c, loginID)
	})
	if err != nil {
		return state, err
	}
	if raw == nil {
		return state, ErrNotFound
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	return state, nil
}

func (s *Store) Delete(ctx context.Context, loginID string) error {
	_, err := ctxutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, s.DeleteMTProtoLogin(c, loginID)
	})
	return err
}
