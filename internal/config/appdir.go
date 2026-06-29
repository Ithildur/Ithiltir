package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ithildur/EiluneKit/appdir"
)

const envDashHome = "DASH_HOME"

// DefaultAppDirOptions returns dash-specific home discovery options.
func DefaultAppDirOptions() appdir.Options {
	return appdir.Options{
		EnvVar: envDashHome,
		Markers: []string{
			"configs",
			"dist",
			"deploy",
			filepath.Join("web", "dist"),
		},
		RequireDirMarkers: true,
	}
}

func HomeDir() (string, error) {
	return discoverHome("app home")
}

func ThemeRootDir() (string, error) {
	home := strings.TrimSpace(os.Getenv(envDashHome))
	if home == "" {
		discovered, err := appdir.DiscoverHome(DefaultAppDirOptions())
		if err == nil && strings.TrimSpace(discovered) != "" {
			home = strings.TrimSpace(discovered)
		}
	}
	if home == "" {
		home = "."
	}
	return filepath.Join(home, "themes"), nil
}

func InstallIDPath() (string, error) {
	home, err := discoverHome("install id home")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "install_id"), nil
}

func discoverHome(name string) (string, error) {
	home, err := appdir.DiscoverHome(DefaultAppDirOptions())
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", fmt.Errorf("resolve %s: empty", name)
	}
	return home, nil
}
