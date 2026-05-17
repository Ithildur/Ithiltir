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
