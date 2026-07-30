package theme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type activeIDStore struct {
	id string
}

func (s activeIDStore) GetActiveThemeID(context.Context) (string, error) {
	return s.id, nil
}

func (activeIDStore) SetActiveThemeID(context.Context, string) error {
	return nil
}

func TestResolveActiveInvalidIDFallsBack(t *testing.T) {
	store := activeIDStore{id: " INVALID ID "}
	active, err := ResolveActive(context.Background(), store, nil)
	if err != nil {
		t.Fatalf("ResolveActive() error = %v", err)
	}
	if active.ConfiguredID != "INVALID ID" || active.ResolvedID != DefaultID {
		t.Fatalf("ResolveActive() IDs = configured %q, resolved %q", active.ConfiguredID, active.ResolvedID)
	}
	if active.State != ActiveBroken || active.Err == nil {
		t.Fatalf("ResolveActive() state = %q, err = %v, want broken fallback", active.State, active.Err)
	}

	id, err := ReadActiveID(context.Background(), store)
	if err != nil {
		t.Fatalf("ReadActiveID() error = %v", err)
	}
	if id != DefaultID {
		t.Fatalf("ReadActiveID() = %q, want %q", id, DefaultID)
	}
}

func TestLoadActiveMissingCustomThemeFallsBack(t *testing.T) {
	st, err := NewStore(filepath.Join(t.TempDir(), "themes"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	active, err := st.LoadActive("missing_theme")
	if err != nil {
		t.Fatalf("LoadActive() error = %v", err)
	}
	if active.ConfiguredID != "missing_theme" || active.ResolvedID != DefaultID {
		t.Fatalf("LoadActive() IDs = configured %q, resolved %q", active.ConfiguredID, active.ResolvedID)
	}
	if active.State != ActiveMissing || active.Err != nil {
		t.Fatalf("LoadActive() state = %q, err = %v, want missing without error", active.State, active.Err)
	}
}

func TestLoadActiveBrokenCustomThemeFallsBack(t *testing.T) {
	root := filepath.Join(t.TempDir(), "themes")
	st, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	writeCustomTheme(t, root, "broken_theme", ".bad { --theme-fg-default: #111111; }")

	active, err := st.LoadActive("broken_theme")
	if err != nil {
		t.Fatalf("LoadActive() error = %v", err)
	}
	if active.ConfiguredID != "broken_theme" || active.ResolvedID != DefaultID {
		t.Fatalf("LoadActive() IDs = configured %q, resolved %q", active.ConfiguredID, active.ResolvedID)
	}
	if active.State != ActiveBroken || active.Err == nil || !strings.Contains(active.Err.Error(), "load custom theme css") {
		t.Fatalf("LoadActive() state = %q, err = %v, want broken css state", active.State, active.Err)
	}
}

func TestListCustomWithWarningsReportsBrokenTheme(t *testing.T) {
	root := filepath.Join(t.TempDir(), "themes")
	st, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	writeCustomTheme(t, root, "good_theme", ":root { --theme-fg-default: #111111; }")
	writeCustomTheme(t, root, "bad_theme", ".bad { --theme-fg-default: #111111; }")

	items, warnings, err := st.ListCustomWithWarnings()
	if err != nil {
		t.Fatalf("ListCustomWithWarnings() error = %v", err)
	}
	if len(items) != 1 || items[0].Manifest.ID != "good_theme" {
		t.Fatalf("ListCustomWithWarnings() items = %+v, want only good_theme", items)
	}
	if len(warnings) != 1 || warnings[0].ID != "bad_theme" {
		t.Fatalf("ListCustomWithWarnings() warnings = %+v, want bad_theme", warnings)
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
