package nodeupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dash/internal/config"
	"github.com/Ithildur/EiluneKit/appdir"
)

var (
	ErrMissingPlatform     = errors.New("node platform is unknown")
	ErrUnsupportedPlatform = errors.New("node platform is unsupported")
	ErrAssetNotFound       = errors.New("node update asset not found")
)

type Asset struct {
	Platform string
	Arch     string
	URL      string
	SHA256   string
	Size     int64
}

type fileDigest struct {
	SHA256  string
	Size    int64
	ModTime time.Time
}

var digestCache = struct {
	sync.RWMutex
	items map[string]fileDigest
}{
	items: make(map[string]fileDigest),
}

func BundledAsset(cfg *config.Config, osName, arch string) (Asset, error) {
	platform, arch, file, err := assetName(osName, arch)
	if err != nil {
		return Asset{}, err
	}

	localPath, err := assetLocalPath(platform, file)
	if err != nil {
		return Asset{}, err
	}

	digest, err := cachedDigest(localPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Asset{}, fmt.Errorf("%w: %s", ErrAssetNotFound, localPath)
		}
		return Asset{}, err
	}

	return Asset{
		Platform: platform,
		Arch:     arch,
		URL:      assetURL(cfg, platform, file),
		SHA256:   digest.SHA256,
		Size:     digest.Size,
	}, nil
}

func assetLocalPath(platform, file string) (string, error) {
	home, err := assetHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "deploy", platform, file), nil
}

func assetHome() (string, error) {
	home, err := appdir.DiscoverHome(config.DefaultAppDirOptions())
	if err != nil {
		return "", fmt.Errorf("resolve app home: %w", err)
	}
	return home, nil
}

func assetName(osName, arch string) (string, string, string, error) {
	osName = strings.ToLower(strings.TrimSpace(osName))
	arch = strings.ToLower(strings.TrimSpace(arch))
	if osName == "" || arch == "" {
		return "", "", "", ErrMissingPlatform
	}

	switch arch {
	case "amd64", "x86_64":
		arch = "amd64"
	case "arm64", "aarch64":
		arch = "arm64"
	default:
		return "", "", "", fmt.Errorf("%w: arch %q", ErrUnsupportedPlatform, arch)
	}

	switch osName {
	case "linux":
		return "linux", arch, "node_linux_" + arch, nil
	case "darwin", "macos":
		return "macos", arch, "node_macos_" + arch, nil
	case "windows":
		return "windows", arch, "node_windows_" + arch + ".exe", nil
	default:
		return "", "", "", fmt.Errorf("%w: os %q", ErrUnsupportedPlatform, osName)
	}
}

func assetURL(cfg *config.Config, platform, file string) string {
	basePath := ""
	scheme := ""
	host := ""
	if cfg != nil {
		basePath = cfg.App.PublicURLBasePath
		scheme = cfg.App.PublicURLScheme
		host = cfg.App.PublicURLHost
	}
	fullPath := path.Join("/", strings.TrimPrefix(basePath, "/"), "deploy", platform, file)
	return (&url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   fullPath,
	}).String()
}

func cachedDigest(path string) (fileDigest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileDigest{}, err
	}
	if !info.Mode().IsRegular() {
		return fileDigest{}, fmt.Errorf("node asset is not a regular file: %s", path)
	}

	digestCache.RLock()
	cached, ok := digestCache.items[path]
	digestCache.RUnlock()
	if ok && cached.Size == info.Size() && cached.ModTime.Equal(info.ModTime()) {
		return cached, nil
	}

	sum, size, err := fileSHA256(path)
	if err != nil {
		return fileDigest{}, err
	}
	fresh := fileDigest{
		SHA256:  sum,
		Size:    size,
		ModTime: info.ModTime(),
	}

	digestCache.Lock()
	digestCache.items[path] = fresh
	digestCache.Unlock()
	return fresh, nil
}

func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("hash node asset %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
