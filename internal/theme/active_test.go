package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadActiveMissingCustomThemeMarksMissing(t *testing.T) {
	st, err := NewStore(filepath.Join(t.TempDir(), "themes"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	active, err := st.LoadActive("missing_theme")
	if err != nil {
		t.Fatalf("LoadActive() error = %v", err)
	}
	if active.ID != DefaultID {
		t.Fatalf("LoadActive().ID = %q, want %q", active.ID, DefaultID)
	}
	if active.MissingID != "missing_theme" {
		t.Fatalf("LoadActive().MissingID = %q, want missing_theme", active.MissingID)
	}
	if active.BrokenID != "" || active.BrokenErr != nil {
		t.Fatalf("LoadActive() marked missing theme as broken: id=%q err=%v", active.BrokenID, active.BrokenErr)
	}
}

func TestLoadActiveBrokenCustomThemeMarksBroken(t *testing.T) {
	st, err := NewStore(filepath.Join(t.TempDir(), "themes"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	writeCustomTheme(t, st.Root(), "broken_theme", ".bad { --theme-fg-default: #111111; }")

	active, err := st.LoadActive("broken_theme")
	if err != nil {
		t.Fatalf("LoadActive() error = %v", err)
	}
	if active.ID != DefaultID {
		t.Fatalf("LoadActive().ID = %q, want %q", active.ID, DefaultID)
	}
	if active.MissingID != "" {
		t.Fatalf("LoadActive().MissingID = %q, want empty", active.MissingID)
	}
	if active.BrokenID != "broken_theme" {
		t.Fatalf("LoadActive().BrokenID = %q, want broken_theme", active.BrokenID)
	}
	if active.BrokenErr == nil || !strings.Contains(active.BrokenErr.Error(), "load custom theme css") {
		t.Fatalf("LoadActive().BrokenErr = %v, want css error", active.BrokenErr)
	}
}

func TestListCustomWithWarningsReportsSkippedThemes(t *testing.T) {
	st, err := NewStore(filepath.Join(t.TempDir(), "themes"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	writeCustomTheme(t, st.Root(), "good_theme", ":root { --theme-fg-default: #111111; }")
	writeCustomTheme(t, st.Root(), "bad_theme", ".bad { --theme-fg-default: #111111; }")

	items, warnings, err := st.ListCustomWithWarnings()
	if err != nil {
		t.Fatalf("ListCustomWithWarnings() error = %v", err)
	}
	if len(items) != 1 || items[0].Manifest.ID != "good_theme" {
		t.Fatalf("ListCustomWithWarnings() items = %+v, want only good_theme", items)
	}
	if len(warnings) != 1 {
		t.Fatalf("ListCustomWithWarnings() warnings len = %d, want 1", len(warnings))
	}
	if warnings[0].ID != "bad_theme" {
		t.Fatalf("ListCustomWithWarnings() warning ID = %q, want bad_theme", warnings[0].ID)
	}
	if warnings[0].Err == nil || !strings.Contains(warnings[0].Err.Error(), "load theme css") {
		t.Fatalf("ListCustomWithWarnings() warning err = %v, want css warning", warnings[0].Err)
	}
}

func writeCustomTheme(t *testing.T, root, id, tokens string) {
	t.Helper()

	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", id, err)
	}
	writeThemeFile(t, dir, "theme.json", validThemeManifest(id, id))
	writeThemeFile(t, dir, "tokens.css", tokens)
}

func validThemeManifest(id, name string) string {
	return fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "version": "1.0.0",
  "author": "test",
  "description": "test theme",
  "skin": {
    "admin": { "shell": "sidebar", "frame": "layered" },
    "dashboard": { "summary": "cards", "density": "comfortable" }
  }
}`, id, name)
}
