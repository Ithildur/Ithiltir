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
	return discoverMutableHome("app home")
}

func ThemeRootDir() (string, error) {
	home := strings.TrimSpace(os.Getenv(envDashHome))
	if home == "" {
		discovered, err := appdir.DiscoverHome(DefaultAppDirOptions())
		if err == nil && strings.TrimSpace(discovered) != "" {
			home = mutableHome(strings.TrimSpace(discovered))
		}
	}
	if home == "" {
		home = "."
	}
	return filepath.Join(home, "themes"), nil
}

func InstallIDPath() (string, error) {
	home, err := discoverMutableHome("install id home")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "install_id"), nil
}

func NotifyConfigKeyPath() (string, error) {
	home, err := discoverMutableHome("notification config key home")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "configs", "notify-config.key"), nil
}

func discoverMutableHome(name string) (string, error) {
	home, err := discoverHome(name)
	if err != nil {
		return "", err
	}
	if _, explicit := os.LookupEnv(envDashHome); explicit {
		return home, nil
	}
	return mutableHome(home), nil
}

func mutableHome(home string) string {
	releases := filepath.Dir(home)
	if filepath.Base(releases) != "releases" {
		return home
	}
	root := filepath.Dir(releases)
	if info, err := os.Stat(filepath.Join(root, "configs")); err == nil && info.IsDir() {
		return root
	}
	return home
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
