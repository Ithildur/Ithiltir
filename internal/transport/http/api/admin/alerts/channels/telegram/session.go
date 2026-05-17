package telegram

import "dash/internal/notify"

func SessionFromConfig(raw []byte) (string, bool, error) {
	return notify.SessionFromConfig(raw)
}
