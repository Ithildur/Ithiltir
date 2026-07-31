package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeRootDirUsesDashHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envDashHome, home)

	got, err := ThemeRootDir()
	if err != nil {
		t.Fatalf("ThemeRootDir() error = %v", err)
	}
	want := filepath.Join(home, "themes")
	if got != want {
		t.Fatalf("ThemeRootDir() = %q, want %q", got, want)
	}
}

func TestInstallIDPathDoesNotFallbackToCwd(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	t.Setenv(envDashHome, "")
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	if _, err := InstallIDPath(); err == nil {
		t.Fatalf("InstallIDPath() error = nil, want error")
	}
}

func TestMutableHome(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(root, "releases", "1.2.3")
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatalf("create install configs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(release, "configs"), 0o755); err != nil {
		t.Fatalf("create release configs: %v", err)
	}

	if got := mutableHome(release); got != root {
		t.Fatalf("mutableHome() = %q, want %q", got, root)
	}

	home := t.TempDir()
	if got := mutableHome(home); got != home {
		t.Fatalf("mutableHome() = %q, want %q", got, home)
	}
}
