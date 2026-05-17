package mtproto

import (
	"context"
	"encoding/json"
	"errors"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
)

var errInvalidChannel = errors.New("invalid mtproto channel")

type channelConfig struct {
	APIID   int
	APIHash string
	Phone   string
	Session string
}

func loadChannelConfig(ctx context.Context, st *alertstore.Store, channelID int64) (channelConfig, error) {
	item, err := loadChannel(ctx, st, channelID)
	if err != nil {
		return channelConfig{}, err
	}
	if item.Type != model.NotifyTypeTelegram {
		return channelConfig{}, errInvalidChannel
	}
	cfgAny, err := notify.DecodeConfig(item.Type, json.RawMessage(item.Config))
	if err != nil {
		return channelConfig{}, errInvalidChannel
	}
	cfg, ok := cfgAny.(notify.TelegramMTProtoConfig)
	if !ok {
		return channelConfig{}, errInvalidChannel
	}
	return channelConfig{
		APIID:   cfg.APIID,
		APIHash: cfg.APIHash,
		Phone:   cfg.Phone,
		Session: cfg.Session,
	}, nil
}

func updateSession(ctx context.Context, st *alertstore.Store, channelID int64, session string) error {
	item, err := loadChannel(ctx, st, channelID)
	if err != nil {
		return err
	}
	if item.Type != model.NotifyTypeTelegram {
		return errInvalidChannel
	}
	cfgAny, err := notify.DecodeConfig(item.Type, json.RawMessage(item.Config))
	if err != nil {
		return errInvalidChannel
	}
	cfg, ok := cfgAny.(notify.TelegramMTProtoConfig)
	if !ok {
		return errInvalidChannel
	}
	cfg.Session = session
	payload, err := json.Marshal(cfg)
	if err != nil {
		return errInvalidChannel
	}
	_, err = infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.ReplaceChannel(c, channelID, map[string]any{
			"config": payload,
		})
	})
	return err
}

func loadChannel(ctx context.Context, st *alertstore.Store, id int64) (*model.NotifyChannel, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (*model.NotifyChannel, error) {
		return st.GetChannel(c, id)
	})
}
