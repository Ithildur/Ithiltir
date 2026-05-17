package theme

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreRejectsEmptyRoot(t *testing.T) {
	if _, err := NewStore(""); err == nil {
		t.Fatal("NewStore(\"\") error = nil, want error")
	}
}

func TestNewStoreCreatesRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "themes")
	st, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if st.Root() != root {
		t.Fatalf("Root() = %q, want %q", st.Root(), root)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat(root) error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("theme root is not a directory")
	}
}

func TestCustomThemeInstallReadDeleteUsesStoreRoot(t *testing.T) {
	source := t.TempDir()
	writeThemeFile(t, source, "theme.json", `{
  "id": "custom_test",
  "name": "Custom Test",
  "version": "1.0.0",
  "author": "test",
  "description": "test theme",
  "skin": {
    "admin": { "shell": "sidebar", "frame": "layered" },
    "dashboard": { "summary": "cards", "density": "comfortable" }
  }
}`)
	writeThemeFile(t, source, "tokens.css", ":root { --theme-fg-default: #111111; }")

	_, archive, err := PackDir(source)
	if err != nil {
		t.Fatalf("PackDir() error = %v", err)
	}

	st, err := NewStore(filepath.Join(t.TempDir(), "themes"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	installed, err := st.InstallZip(archive)
	if err != nil {
		t.Fatalf("InstallZip() error = %v", err)
	}
	if installed.Manifest.ID != "custom_test" {
		t.Fatalf("InstallZip().Manifest.ID = %q, want custom_test", installed.Manifest.ID)
	}

	manifest, err := st.LoadCustomManifest("custom_test")
	if err != nil {
		t.Fatalf("LoadCustomManifest() error = %v", err)
	}
	if manifest.ID != "custom_test" {
		t.Fatalf("LoadCustomManifest().ID = %q, want custom_test", manifest.ID)
	}
	if _, err := st.LoadCustomCSS("custom_test"); err != nil {
		t.Fatalf("LoadCustomCSS() error = %v", err)
	}

	if err := st.RemoveCustom("custom_test"); err != nil {
		t.Fatalf("RemoveCustom() error = %v", err)
	}
	exists, err := st.CustomExists("custom_test")
	if err != nil {
		t.Fatalf("CustomExists() error = %v", err)
	}
	if exists {
		t.Fatal("CustomExists() = true, want false")
	}
}

func TestStoreFileMethodsRejectUnavailableStorage(t *testing.T) {
	tests := []struct {
		name string
		st   *Store
	}{
		{name: "nil"},
		{name: "zero", st: &Store{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.st.LoadCustomManifest("custom_test"); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("LoadCustomManifest() error = %v, want ErrThemeStorage", err)
			}
			if _, err := tt.st.LoadCustomCSS("custom_test"); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("LoadCustomCSS() error = %v, want ErrThemeStorage", err)
			}
			if _, err := tt.st.LoadCustomPreview("custom_test"); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("LoadCustomPreview() error = %v, want ErrThemeStorage", err)
			}
			if err := tt.st.RemoveCustom("custom_test"); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("RemoveCustom() error = %v, want ErrThemeStorage", err)
			}
			if _, err := tt.st.CustomExists("custom_test"); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("CustomExists() error = %v, want ErrThemeStorage", err)
			}
			if _, _, err := tt.st.ListCustomWithWarnings(); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("ListCustomWithWarnings() error = %v, want ErrThemeStorage", err)
			}
			if _, err := tt.st.InstallZip(nil); !errors.Is(err, ErrThemeStorage) {
				t.Fatalf("InstallZip() error = %v, want ErrThemeStorage", err)
			}
		})
	}
}

func writeThemeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}
