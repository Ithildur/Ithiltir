package alert

import (
	"context"
	"encoding/json"
	"errors"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
)

var ErrInvalidMTProtoChannel = errors.New("invalid mtproto channel")

type MTProtoConfig struct {
	Revision int64
	APIID    int
	APIHash  string
	Phone    string
	Session  string
}

func (s *Store) MTProtoConfig(ctx context.Context, channelID int64) (MTProtoConfig, error) {
	item, err := s.loadMTProtoChannel(ctx, channelID)
	if err != nil {
		return MTProtoConfig{}, err
	}
	if item.Type != model.NotifyTypeTelegram {
		return MTProtoConfig{}, ErrInvalidMTProtoChannel
	}
	cfgAny, err := notify.DecodeConfig(item.Type, json.RawMessage(item.Config))
	if err != nil {
		return MTProtoConfig{}, ErrInvalidMTProtoChannel
	}
	cfg, ok := cfgAny.(notify.TelegramMTProtoConfig)
	if !ok {
		return MTProtoConfig{}, ErrInvalidMTProtoChannel
	}
	return MTProtoConfig{
		Revision: item.Revision,
		APIID:    cfg.APIID,
		APIHash:  cfg.APIHash,
		Phone:    cfg.Phone,
		Session:  cfg.Session,
	}, nil
}

func (s *Store) UpdateMTProtoSession(ctx context.Context, channelID, revision int64, session string) error {
	item, err := s.loadMTProtoChannel(ctx, channelID)
	if err != nil {
		return err
	}
	if item.Revision != revision {
		return ErrChannelVersionStale
	}
	if item.Type != model.NotifyTypeTelegram {
		return ErrInvalidMTProtoChannel
	}
	cfgAny, err := notify.DecodeConfig(item.Type, json.RawMessage(item.Config))
	if err != nil {
		return ErrInvalidMTProtoChannel
	}
	cfg, ok := cfgAny.(notify.TelegramMTProtoConfig)
	if !ok {
		return ErrInvalidMTProtoChannel
	}
	cfg.Session = session
	payload, err := json.Marshal(cfg)
	if err != nil {
		return ErrInvalidMTProtoChannel
	}
	_, err = infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, s.UpdateChannelConfig(c, channelID, revision, payload)
	})
	return err
}

func (s *Store) loadMTProtoChannel(ctx context.Context, id int64) (*model.NotifyChannel, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (*model.NotifyChannel, error) {
		return s.GetChannel(c, id)
	})
}
