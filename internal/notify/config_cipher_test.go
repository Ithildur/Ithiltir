package notify

import (
	"bytes"
	"errors"
	"testing"

	"dash/internal/model"
)

func TestConfigCipherBindsChannelIdentity(t *testing.T) {
	configCipher, err := NewConfigCipher(bytes.Repeat([]byte{0x42}, ConfigKeySize))
	if err != nil {
		t.Fatalf("NewConfigCipher() error = %v", err)
	}
	plain := []byte(`{"token":"secret"}`)
	sealed, err := configCipher.Seal(7, model.NotifyTypeTelegram, plain)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	got, err := configCipher.Open(7, model.NotifyTypeTelegram, sealed)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("Open() = %q, want %q", got, plain)
	}

	for _, tc := range []struct {
		name   string
		id     int64
		typ    model.NotifyType
		sealed []byte
	}{
		{name: "different channel", id: 8, typ: model.NotifyTypeTelegram, sealed: sealed},
		{name: "different type", id: 7, typ: model.NotifyTypeWebhook, sealed: sealed},
		{name: "tampered", id: 7, typ: model.NotifyTypeTelegram, sealed: append([]byte(nil), sealed...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "tampered" {
				tc.sealed[len(tc.sealed)-1] ^= 0xff
			}
			if _, err := configCipher.Open(tc.id, tc.typ, tc.sealed); !errors.Is(err, ErrConfigCiphertext) {
				t.Fatalf("Open() error = %v, want ErrConfigCiphertext", err)
			}
		})
	}
}
