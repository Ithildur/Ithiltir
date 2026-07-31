package notify

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func LoadConfigCipher(path string) (*ConfigCipher, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat notification config key: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("notification config key is not a regular file")
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 || perm&0o400 == 0 || perm&0o111 != 0 {
		return nil, fmt.Errorf("notification config key permissions are %04o, want owner-readable only", perm)
	}

	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read notification config key: %w", err)
	}
	defer clear(key)
	configCipher, err := NewConfigCipher(key)
	if err != nil {
		return nil, fmt.Errorf("load notification config key: %w", err)
	}
	return configCipher, nil
}

// CreateConfigKey creates the one installation-wide raw AES-256 key. It never
// replaces an existing file; callers must decide whether a missing key can be
// created without orphaning existing ciphertext.
func CreateConfigKey(path string) error {
	if path == "" {
		return errors.New("notification config key path is empty")
	}
	dirPath := filepath.Dir(path)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("create notification config key directory: %w", err)
	}

	key := make([]byte, ConfigKeySize)
	defer clear(key)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate notification config key: %w", err)
	}

	f, err := os.CreateTemp(dirPath, ".notify-config.key-*")
	if err != nil {
		return fmt.Errorf("create temporary notification config key: %w", err)
	}
	tempPath := f.Name()
	defer func() {
		_ = f.Close()
		_ = os.Remove(tempPath)
	}()

	if err := f.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary notification config key permissions: %w", err)
	}
	n, err := f.Write(key)
	if err != nil {
		return fmt.Errorf("write temporary notification config key: %w", err)
	}
	if n != len(key) {
		return fmt.Errorf("write temporary notification config key: wrote %d of %d bytes", n, len(key))
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync temporary notification config key: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temporary notification config key: %w", err)
	}
	if err := unix.Renameat2(
		unix.AT_FDCWD,
		tempPath,
		unix.AT_FDCWD,
		path,
		unix.RENAME_NOREPLACE,
	); err != nil {
		return fmt.Errorf("publish notification config key: %w", err)
	}

	dir, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("open notification config key directory: %w", err)
	}
	if err := errors.Join(dir.Sync(), dir.Close()); err != nil {
		return fmt.Errorf("sync notification config key directory: %w", err)
	}
	return nil
}
