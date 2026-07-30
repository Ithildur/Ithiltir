package theme

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

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
	writeThemeFile(t, source, "README.md", "# Custom Test\n\nTheme documentation.\n")

	_, archive, err := PackDir(source)
	if err != nil {
		t.Fatalf("PackDir() error = %v", err)
	}

	root := filepath.Join(t.TempDir(), "themes")
	st, err := NewStore(root)
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

	active, err := st.LoadActive("custom_test")
	if err != nil {
		t.Fatalf("LoadActive() error = %v", err)
	}
	if active.State != ActiveReady || active.Manifest.ID != "custom_test" || len(active.CSS) == 0 {
		t.Fatalf("LoadActive() = %+v, want ready custom_test with CSS", active)
	}
	readme, err := os.ReadFile(filepath.Join(root, "custom_test", "README.md"))
	if err != nil {
		t.Fatalf("ReadFile(README.md) error = %v", err)
	}
	if string(readme) != "# Custom Test\n\nTheme documentation.\n" {
		t.Fatalf("README.md = %q", readme)
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

func TestValidatePreviewRejectsTruncatedPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	raw := encoded.Bytes()
	idat := bytes.Index(raw, []byte("IDAT"))
	if idat < 4 {
		t.Fatal("encoded PNG has no IDAT chunk")
	}
	truncated := raw[:idat-4]
	if _, err := png.DecodeConfig(bytes.NewReader(truncated)); err != nil {
		t.Fatalf("fixture must pass DecodeConfig(), got %v", err)
	}
	if err := validatePreview(truncated); err == nil {
		t.Fatal("validatePreview() error = nil, want truncated PNG rejection")
	}
}

func writeThemeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}
