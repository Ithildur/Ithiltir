package notify

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"

	"dash/internal/model"
)

const (
	ConfigKeySize       = 32
	configCipherVersion = byte(1)
)

var ErrConfigCiphertext = errors.New("invalid notification config ciphertext")

// ConfigCipher seals complete notification channel configurations. The channel
// identity is authenticated as associated data so ciphertext cannot be moved
// between channels or channel types.
type ConfigCipher struct {
	aead cipher.AEAD
}

func NewConfigCipher(key []byte) (*ConfigCipher, error) {
	if len(key) != ConfigKeySize {
		return nil, fmt.Errorf("notification config key must be %d bytes", ConfigKeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create notification config cipher: %w", err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("create notification config AEAD: %w", err)
	}
	return &ConfigCipher{aead: aead}, nil
}

func (c *ConfigCipher) Seal(id int64, typ model.NotifyType, plain []byte) ([]byte, error) {
	if err := validateConfigIdentity(id, typ); err != nil {
		return nil, err
	}
	if c == nil || c.aead == nil {
		return nil, errors.New("notification config cipher is not initialized")
	}
	if len(plain) == 0 {
		return nil, errors.New("notification config is empty")
	}

	sealed := make([]byte, 1, 1+c.aead.Overhead()+len(plain))
	sealed[0] = configCipherVersion
	return c.aead.Seal(sealed, nil, plain, configAAD(id, typ)), nil
}

func (c *ConfigCipher) Open(id int64, typ model.NotifyType, sealed []byte) ([]byte, error) {
	if err := validateConfigIdentity(id, typ); err != nil {
		return nil, err
	}
	if c == nil || c.aead == nil {
		return nil, errors.New("notification config cipher is not initialized")
	}
	if len(sealed) < 1+c.aead.Overhead() || sealed[0] != configCipherVersion {
		return nil, ErrConfigCiphertext
	}
	plain, err := c.aead.Open(nil, nil, sealed[1:], configAAD(id, typ))
	if err != nil {
		return nil, ErrConfigCiphertext
	}
	return plain, nil
}

func validateConfigIdentity(id int64, typ model.NotifyType) error {
	if id <= 0 {
		return fmt.Errorf("notification channel ID must be positive")
	}
	if typ == "" {
		return errors.New("notification channel type is empty")
	}
	return nil
}

func configAAD(id int64, typ model.NotifyType) []byte {
	const prefix = "notify-config/v1\x00"
	aad := make([]byte, len(prefix)+8, len(prefix)+8+len(typ))
	copy(aad, prefix)
	binary.BigEndian.PutUint64(aad[len(prefix):], uint64(id))
	return append(aad, typ...)
}
