package notify

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	msgpeer "github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
)

type MTProtoLoginState struct {
	ChannelID     int64  `json:"channel_id"`
	APIID         int    `json:"api_id"`
	APIHash       string `json:"api_hash"`
	Phone         string `json:"phone"`
	PhoneCodeHash string `json:"phone_code_hash"`
	Session       string `json:"session"`
}

func StartLogin(ctx context.Context, apiID int, apiHash, phone string) (MTProtoLoginState, int, error) {
	storage := &session.StorageMemory{}
	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: storage,
		NoUpdates:      true,
	})
	var codeHash string
	var timeout int

	if err := client.Run(ctx, func(ctx context.Context) error {
		sentCode, err := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		if err != nil {
			return err
		}
		hash, to, err := extractSentCode(sentCode)
		if err != nil {
			return err
		}
		codeHash = hash
		timeout = to
		return nil
	}); err != nil {
		return MTProtoLoginState{}, 0, err
	}

	sessionText, err := dumpSession(storage)
	if err != nil {
		return MTProtoLoginState{}, 0, err
	}

	state := MTProtoLoginState{
		APIID:         apiID,
		APIHash:       apiHash,
		Phone:         phone,
		PhoneCodeHash: codeHash,
		Session:       sessionText,
	}
	return state, timeout, nil
}

func VerifyCode(ctx context.Context, state MTProtoLoginState, code string) (string, bool, error) {
	storage, err := loadSession(state.Session)
	if err != nil {
		return "", false, err
	}
	client := telegram.NewClient(state.APIID, state.APIHash, telegram.Options{
		SessionStorage: storage,
		NoUpdates:      true,
	})
	passwordRequired := false

	err = client.Run(ctx, func(ctx context.Context) error {
		_, err := client.Auth().SignIn(ctx, state.Phone, code, state.PhoneCodeHash)
		if err != nil {
			if errors.Is(err, auth.ErrPasswordAuthNeeded) {
				passwordRequired = true
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}

	sessionText, err := dumpSession(storage)
	if err != nil {
		return "", passwordRequired, err
	}
	return sessionText, passwordRequired, nil
}

func SubmitPassword(ctx context.Context, state MTProtoLoginState, password string) (string, error) {
	storage, err := loadSession(state.Session)
	if err != nil {
		return "", err
	}
	client := telegram.NewClient(state.APIID, state.APIHash, telegram.Options{
		SessionStorage: storage,
		NoUpdates:      true,
	})
	if err := client.Run(ctx, func(ctx context.Context) error {
		_, err := client.Auth().Password(ctx, password)
		return err
	}); err != nil {
		return "", err
	}

	return dumpSession(storage)
}

func PingSession(ctx context.Context, apiID int, apiHash, sessionText string) error {
	if strings.TrimSpace(sessionText) == "" {
		return errors.New("session is empty")
	}
	storage, err := loadSession(sessionText)
	if err != nil {
		return err
	}
	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: storage,
		NoUpdates:      true,
	})
	return client.Run(ctx, func(ctx context.Context) error {
		_, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      1,
			OffsetPeer: &tg.InputPeerEmpty{},
		})
		return err
	})
}

func sendMTProtoMessage(ctx context.Context, cfg TelegramMTProtoConfig, peer tg.InputPeerClass, text string) error {
	storage, err := loadSession(cfg.Session)
	if err != nil {
		return err
	}
	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{
		SessionStorage: storage,
		NoUpdates:      true,
	})
	return client.Run(ctx, func(ctx context.Context) error {
		sender := message.NewSender(client.API())
		if peer != nil {
			_, err := sender.To(peer).Text(ctx, text)
			return err
		}

		target, err := resolveTarget(cfg.ChatID, cfg.Username)
		if err != nil {
			return err
		}
		if target.self {
			_, err := sender.Self().Text(ctx, text)
			return err
		}
		if target.resolve != "" {
			_, err := sender.Resolve(target.resolve).Text(ctx, text)
			return err
		}
		if target.id == 0 {
			return fmt.Errorf("chat_id is required")
		}

		p, err := resolvePeerByID(ctx, client.API(), target.id)
		if err != nil {
			return err
		}
		_, err = sender.To(p).Text(ctx, text)
		return err
	})
}

type mtprotoTarget struct {
	resolve string
	id      int64
	self    bool
}

func resolveTarget(chatID, username string) (mtprotoTarget, error) {
	username = strings.TrimSpace(username)
	if username != "" {
		return mtprotoTarget{resolve: username}, nil
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return mtprotoTarget{}, fmt.Errorf("chat_id is required")
	}
	if strings.EqualFold(chatID, "self") {
		return mtprotoTarget{self: true}, nil
	}
	if strings.HasPrefix(chatID, "@") {
		return mtprotoTarget{resolve: chatID}, nil
	}
	if id, ok := parseID(chatID); ok {
		return mtprotoTarget{id: id}, nil
	}
	return mtprotoTarget{resolve: chatID}, nil
}

func parseID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if strings.HasPrefix(raw, "-100") && len(raw) > 4 {
		raw = raw[4:]
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	if id < 0 {
		id = -id
	}
	return id, true
}

func resolvePeerByID(ctx context.Context, api *tg.Client, id int64) (tg.InputPeerClass, error) {
	resp, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		Limit:      200,
		OffsetPeer: &tg.InputPeerEmpty{},
	})
	if err != nil {
		return nil, err
	}

	entitiesResult, ok := resp.(msgpeer.EntitySearchResult)
	if !ok {
		return nil, fmt.Errorf("unsupported dialogs response")
	}
	entities := msgpeer.EntitiesFromResult(entitiesResult)
	if user, ok := entities.User(id); ok {
		return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
	}
	if chat, ok := entities.Chat(id); ok {
		return &tg.InputPeerChat{ChatID: chat.ID}, nil
	}
	if channel, ok := entities.Channel(id); ok {
		return &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash}, nil
	}

	return nil, fmt.Errorf("chat_id not found")
}

func extractSentCode(sent tg.AuthSentCodeClass) (string, int, error) {
	switch v := sent.(type) {
	case *tg.AuthSentCode:
		return v.PhoneCodeHash, v.Timeout, nil
	case *tg.AuthSentCodePaymentRequired:
		return v.PhoneCodeHash, 0, errors.New("payment required")
	case *tg.AuthSentCodeSuccess:
		return "", 0, errors.New("already authorized")
	default:
		return "", 0, errors.New("unsupported code response")
	}
}

func loadSession(encoded string) (*session.StorageMemory, error) {
	storage := &session.StorageMemory{}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return storage, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}
	if err := storage.StoreSession(context.Background(), data); err != nil {
		return nil, err
	}
	return storage, nil
}

func dumpSession(storage *session.StorageMemory) (string, error) {
	data, err := storage.Bytes(nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
