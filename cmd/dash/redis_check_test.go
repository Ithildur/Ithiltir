package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRedisPasswordPreservesExactValue(t *testing.T) {
	file := filepath.Join(t.TempDir(), "redis-password")
	const password = "  -secret with spaces-  "
	if err := os.WriteFile(file, []byte(password), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	got, err := readRedisPassword(file)
	if err != nil {
		t.Fatalf("readRedisPassword() error = %v", err)
	}
	if got != password {
		t.Fatalf("readRedisPassword() = %q, want exact value %q", got, password)
	}
}

func TestReadRedisPasswordRejectsMultilineFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "redis-password")
	if err := os.WriteFile(file, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	if _, err := readRedisPassword(file); err == nil || !strings.Contains(err.Error(), "exactly one line") {
		t.Fatalf("readRedisPassword() error = %v, want one-line rejection", err)
	}
}
