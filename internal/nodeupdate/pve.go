package nodeupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePVEAssets accepts legacy packages without PVE assets. The separate
// manifest leaves release.env v1 readable by already installed Dash updaters.
func ValidatePVEAssets(deployDir, version string) (bool, error) {
	file := filepath.Join(deployDir, "linux", "pve-cache.env")
	f, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		for _, arch := range []string{"amd64", "arm64"} {
			if _, err := os.Lstat(filepath.Join(deployDir, "linux", "pve_cache_linux_"+arch)); !errors.Is(err, os.ErrNotExist) {
				return false, fmt.Errorf("PVE asset has no manifest")
			}
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return false, err
	}
	if len(raw) > 4096 {
		return false, errors.New("PVE manifest exceeds limit")
	}
	fields := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || fields[key] != "" || value == "" {
			return false, errors.New("invalid PVE manifest field")
		}
		fields[key] = value
	}
	if len(fields) != 4 || fields["format_version"] != "1" || fields["node_version"] != version {
		return false, errors.New("PVE manifest version mismatch")
	}
	for _, arch := range []string{"amd64", "arm64"} {
		expected := fields[arch+"_sha256"]
		if decoded, err := hex.DecodeString(expected); err != nil || len(decoded) != sha256.Size || expected != strings.ToLower(expected) {
			return false, errors.New("invalid PVE checksum")
		}
		file := filepath.Join(deployDir, "linux", "pve_cache_linux_"+arch)
		info, err := os.Lstat(file)
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, errors.New("PVE asset is not a regular file")
		}
		digest, err := cachedDigest(file)
		if err != nil {
			return false, err
		}
		if digest.SHA256 != expected {
			return false, fmt.Errorf("PVE %s checksum mismatch", arch)
		}
	}
	return true, nil
}

func BundledPVE(version string) bool {
	home, err := assetHome()
	if err != nil {
		return false
	}
	ok, err := ValidatePVEAssets(filepath.Join(home, "deploy"), version)
	return err == nil && ok
}
