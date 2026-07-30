package theme

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestBuildThemeArchiveRejectsOversizedArchive(t *testing.T) {
	preview := make([]byte, ArchiveMaxBytes)
	if _, err := rand.NewChaCha8([32]byte{1}).Read(preview); err != nil {
		t.Fatalf("generate preview fixture: %v", err)
	}

	_, err := buildThemeArchive(map[string][]byte{
		"theme.json":  []byte(`{"id":"test"}`),
		"tokens.css":  []byte(":root {}"),
		"preview.png": preview,
	})
	if err == nil || !strings.Contains(err.Error(), "archive size limit") {
		t.Fatalf("buildThemeArchive() error = %v, want archive size limit", err)
	}
}
