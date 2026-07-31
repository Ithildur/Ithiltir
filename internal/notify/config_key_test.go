package notify

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigKeyCreateIsExclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notify-config.key")
	if err := CreateConfigKey(path); err != nil {
		t.Fatalf("CreateConfigKey() error = %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created key: %v", err)
	}
	if len(before) != ConfigKeySize {
		t.Fatalf("created key bytes = %d, want %d", len(before), ConfigKeySize)
	}
	if _, err := LoadConfigCipher(path); err != nil {
		t.Fatalf("LoadConfigCipher() error = %v", err)
	}

	if err := CreateConfigKey(path); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("CreateConfigKey(existing) error = %v, want fs.ErrExist", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read key after rejected replacement: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("CreateConfigKey(existing) replaced the installed key")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read key directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("key directory entries = %v, want only %q", entries, filepath.Base(path))
	}
}
