package serverid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Prefix  = "server_"
	idBytes = 16
)

type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) GetOrCreate() (string, bool, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return "", false, fmt.Errorf("server install id path is empty")
	}

	raw, err := os.ReadFile(s.path)
	if err == nil {
		id := strings.TrimSpace(string(raw))
		if valid(id) {
			return id, false, nil
		}
		return "", false, fmt.Errorf("invalid server install id in %s", s.path)
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("read server install id: %w", err)
	}

	id, err := generate()
	if err != nil {
		return "", false, err
	}
	if err := writeFile(s.path, id); err != nil {
		if errors.Is(err, os.ErrExist) {
			raw, readErr := os.ReadFile(s.path)
			if readErr != nil {
				return "", false, fmt.Errorf("read raced server install id: %w", readErr)
			}
			current := strings.TrimSpace(string(raw))
			if valid(current) {
				return current, false, nil
			}
			return "", false, fmt.Errorf("invalid server install id in %s", s.path)
		}
		return "", false, err
	}
	return id, true, nil
}

func generate() (string, error) {
	var raw [idBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate server install id: %w", err)
	}
	return Prefix + hex.EncodeToString(raw[:]), nil
}

func valid(id string) bool {
	if !strings.HasPrefix(id, Prefix) {
		return false
	}
	suffix := id[len(Prefix):]
	if len(suffix) != idBytes*2 {
		return false
	}
	for _, ch := range suffix {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func writeFile(path, id string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create server install id dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create server install id: %w", err)
	}
	if _, err := f.WriteString(id + "\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("write server install id: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync server install id: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close server install id: %w", err)
	}
	return nil
}
