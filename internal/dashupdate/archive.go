package dashupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	appversion "dash/internal/version"
)

const (
	maxReleaseArchiveBytes  = 1 << 30
	maxReleaseExtractBytes  = 4 << 30
	maxReleaseEntries       = 20_000
	maxReleaseManifestBytes = 64 << 10
	releaseRootName         = "Ithiltir-dash"
)

type releaseManifest struct {
	FormatVersion int
	DashVersion   string
	NodeVersion   string
	TargetOS      string
	TargetArch    string
	NodeSHA256    map[string]string
}

type releaseNodeAsset struct {
	path  string
	field string
}

var releaseNodeAssets = [...]releaseNodeAsset{
	{path: "deploy/linux/node_linux_amd64", field: "node_linux_amd64_sha256"},
	{path: "deploy/linux/node_linux_arm64", field: "node_linux_arm64_sha256"},
	{path: "deploy/macos/node_macos_arm64", field: "node_macos_arm64_sha256"},
	{path: "deploy/windows/node_windows_amd64.exe", field: "node_windows_amd64_sha256"},
	{path: "deploy/windows/node_windows_arm64.exe", field: "node_windows_arm64_sha256"},
	{path: "deploy/windows/runner_windows_amd64.exe", field: "runner_windows_amd64_sha256"},
	{path: "deploy/windows/runner_windows_arm64.exe", field: "runner_windows_arm64_sha256"},
}

func downloadRelease(ctx context.Context, asset releaseAsset, out string) error {
	if asset.Size > maxReleaseArchiveBytes {
		return fmt.Errorf("release archive exceeds %d bytes", maxReleaseArchiveBytes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return fmt.Errorf("create release download: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "Ithiltir-Dash-Updater")
	client := &http.Client{
		Timeout: 15 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !strings.EqualFold(req.URL.Scheme, "https") {
				return errors.New("release redirect must use HTTPS")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download release %s: %w", asset.Version, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("download release %s: HTTP %s", asset.Version, resp.Status)
	}
	if resp.ContentLength > maxReleaseArchiveBytes {
		return fmt.Errorf("release archive exceeds %d bytes", maxReleaseArchiveBytes)
	}

	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create release archive: %w", err)
	}
	written, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxReleaseArchiveBytes+1))
	syncErr := f.Sync()
	closeErr := f.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(out)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if written > maxReleaseArchiveBytes {
		_ = os.Remove(out)
		return fmt.Errorf("release archive exceeds %d bytes", maxReleaseArchiveBytes)
	}
	if asset.Size > 0 && written != asset.Size {
		_ = os.Remove(out)
		return fmt.Errorf("release archive size is %d, want %d", written, asset.Size)
	}
	return nil
}

func extractRelease(archive, dest string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", fmt.Errorf("open release archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("open release gzip stream: %w", err)
	}
	defer gz.Close()

	if err := os.Mkdir(dest, 0o700); err != nil {
		return "", fmt.Errorf("create release extraction directory: %w", err)
	}
	seen := make(map[string]struct{})
	tr := tar.NewReader(gz)
	var entries int
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read release archive: %w", err)
		}
		entries++
		if entries > maxReleaseEntries {
			return "", fmt.Errorf("release archive exceeds %d entries", maxReleaseEntries)
		}

		name, err := cleanArchivePath(hdr.Name)
		if err != nil {
			return "", err
		}
		if _, exists := seen[name]; exists {
			return "", fmt.Errorf("release archive contains duplicate path %q", name)
		}
		seen[name] = struct{}{}
		parts := strings.Split(name, "/")
		if len(parts) == 0 || parts[0] != releaseRootName {
			return "", fmt.Errorf("release archive contains unexpected root path %q", name)
		}

		target := filepath.Join(dest, filepath.FromSlash(name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", fmt.Errorf("create release directory %q: %w", name, err)
			}
		case tar.TypeReg, 0:
			if hdr.Size < 0 || hdr.Size > maxReleaseExtractBytes-total {
				return "", fmt.Errorf("release archive exceeds %d extracted bytes", maxReleaseExtractBytes)
			}
			total += hdr.Size
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", fmt.Errorf("create release file parent %q: %w", name, err)
			}
			mode := os.FileMode(0o644)
			if hdr.FileInfo().Mode().Perm()&0o111 != 0 {
				mode = 0o755
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return "", fmt.Errorf("create release file %q: %w", name, err)
			}
			written, copyErr := io.CopyN(out, tr, hdr.Size)
			syncErr := out.Sync()
			closeErr := out.Close()
			if copyErr != nil || syncErr != nil || closeErr != nil {
				return "", fmt.Errorf("extract release file %q: %w", name, errors.Join(copyErr, syncErr, closeErr))
			}
			if written != hdr.Size {
				return "", fmt.Errorf("release file %q size is %d, want %d", name, written, hdr.Size)
			}
		default:
			return "", fmt.Errorf("release archive path %q is not a regular file or directory", name)
		}
	}

	root := filepath.Join(dest, releaseRootName)
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect extracted release root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("release root is not a directory")
	}
	return root, nil
}

func cleanArchivePath(raw string) (string, error) {
	if raw == "" || strings.Contains(raw, "\\") || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("release archive contains unsafe path %q", raw)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("release archive contains unsafe path %q", raw)
	}
	return strings.TrimSuffix(clean, "/"), nil
}

func validateReleasePackage(ctx context.Context, root, targetVersion string) (releaseManifest, error) {
	manifest, err := readReleaseManifest(filepath.Join(root, "release.env"))
	if err != nil {
		return releaseManifest{}, err
	}
	if manifest.FormatVersion != 1 {
		return releaseManifest{}, fmt.Errorf("unsupported release format %d", manifest.FormatVersion)
	}
	if manifest.DashVersion != targetVersion {
		return releaseManifest{}, fmt.Errorf("release manifest version is %s, want %s", manifest.DashVersion, targetVersion)
	}
	if manifest.TargetOS != "linux" {
		return releaseManifest{}, fmt.Errorf("release target OS is %s, want linux", manifest.TargetOS)
	}
	if manifest.TargetArch != runtime.GOARCH {
		return releaseManifest{}, fmt.Errorf("release target architecture is %s, want %s", manifest.TargetArch, runtime.GOARCH)
	}
	for _, dir := range []string{"configs", "deploy", "dist"} {
		if err := requireReleaseDir(root, dir); err != nil {
			return releaseManifest{}, err
		}
	}
	for _, file := range []string{
		"configs/config.example.yaml",
		"dist/index.html",
		"install_dash_linux.sh",
		"update_dash_linux.sh",
	} {
		if err := requireReleaseFile(root, file); err != nil {
			return releaseManifest{}, err
		}
	}
	for _, asset := range releaseNodeAssets {
		if err := requireReleaseFile(root, asset.path); err != nil {
			return releaseManifest{}, err
		}
	}
	if err := validateReleaseNodeAssets(root, manifest.NodeSHA256); err != nil {
		return releaseManifest{}, err
	}

	dashPath := filepath.Join(root, "bin", "dash")
	dashVersion, err := updateCommandVersion(ctx, dashPath)
	if err != nil {
		return releaseManifest{}, fmt.Errorf("validate release Dash binary: %w", err)
	}
	if dashVersion != manifest.DashVersion {
		return releaseManifest{}, fmt.Errorf(
			"release Dash version does not match manifest: dash=%s manifest=%s",
			dashVersion,
			manifest.DashVersion,
		)
	}
	nodeVersion, err := updateCommandNodeVersion(ctx, dashPath)
	if err != nil {
		return releaseManifest{}, fmt.Errorf("validate release bundled node version: %w", err)
	}
	if nodeVersion != manifest.NodeVersion {
		return releaseManifest{}, fmt.Errorf(
			"release bundled node version does not match manifest: dash=%s manifest=%s",
			nodeVersion,
			manifest.NodeVersion,
		)
	}
	return manifest, nil
}

func validateReleaseNodeAssets(root string, expected map[string]string) error {
	for _, asset := range releaseNodeAssets {
		got, err := releaseFileSHA256(filepath.Join(root, filepath.FromSlash(asset.path)))
		if err != nil {
			return fmt.Errorf("hash release node asset %s: %w", asset.path, err)
		}
		if got != expected[asset.path] {
			return fmt.Errorf("release node asset %s does not match release.env SHA-256", asset.path)
		}
	}
	return nil
}

func releaseFileSHA256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, hashErr := io.Copy(h, f)
	closeErr := f.Close()
	if hashErr != nil || closeErr != nil {
		return "", errors.Join(hashErr, closeErr)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func requireReleaseDir(root, name string) error {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return fmt.Errorf("release directory %s: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("release path %s is not a directory", name)
	}
	return nil
}

func requireReleaseFile(root, name string) error {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return fmt.Errorf("release file %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("release path %s is not a regular file", name)
	}
	if info.Size() == 0 {
		return fmt.Errorf("release file %s is empty", name)
	}
	return nil
}

func readReleaseManifest(file string) (releaseManifest, error) {
	f, err := os.Open(file)
	if err != nil {
		return releaseManifest{}, fmt.Errorf("read release manifest: %w", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxReleaseManifestBytes+1))
	if err != nil {
		return releaseManifest{}, fmt.Errorf("read release manifest: %w", err)
	}
	if len(raw) > maxReleaseManifestBytes {
		return releaseManifest{}, fmt.Errorf("release manifest exceeds %d bytes", maxReleaseManifestBytes)
	}
	fields := make(map[string]string)
	for lineNo, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || strings.TrimSpace(key) != key || strings.TrimSpace(value) != value {
			return releaseManifest{}, fmt.Errorf("release.env:%d: invalid field", lineNo+1)
		}
		if _, exists := fields[key]; exists {
			return releaseManifest{}, fmt.Errorf("release.env:%d: duplicate field %q", lineNo+1, key)
		}
		fields[key] = value
	}
	allowed := map[string]struct{}{
		"format_version": {},
		"dash_version":   {},
		"node_version":   {},
		"target_os":      {},
		"target_arch":    {},
	}
	for _, asset := range releaseNodeAssets {
		allowed[asset.field] = struct{}{}
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return releaseManifest{}, fmt.Errorf("release.env contains unknown field %q", key)
		}
	}
	for _, key := range []string{"format_version", "dash_version", "node_version", "target_os", "target_arch"} {
		if fields[key] == "" {
			return releaseManifest{}, fmt.Errorf("release.env is missing %s", key)
		}
	}
	nodeSHA256 := make(map[string]string, len(releaseNodeAssets))
	for _, asset := range releaseNodeAssets {
		sum := fields[asset.field]
		if sum == "" {
			return releaseManifest{}, fmt.Errorf("release.env is missing %s", asset.field)
		}
		if len(sum) != sha256.Size*2 || sum != strings.ToLower(sum) {
			return releaseManifest{}, fmt.Errorf("release.env contains invalid SHA-256 field %s", asset.field)
		}
		if _, err := hex.DecodeString(sum); err != nil {
			return releaseManifest{}, fmt.Errorf("release.env contains invalid SHA-256 field %s", asset.field)
		}
		nodeSHA256[asset.path] = sum
	}
	if fields["format_version"] != "1" {
		return releaseManifest{}, fmt.Errorf("invalid release format version %q", fields["format_version"])
	}
	if err := appversion.Validate(fields["dash_version"]); err != nil {
		return releaseManifest{}, fmt.Errorf("invalid Dash version in release.env: %w", err)
	}
	if err := appversion.ValidateNodeVersion(fields["node_version"]); err != nil {
		return releaseManifest{}, fmt.Errorf("invalid node version in release.env: %w", err)
	}
	if fields["target_os"] != "linux" {
		return releaseManifest{}, fmt.Errorf("invalid target OS %q", fields["target_os"])
	}
	if fields["target_arch"] != "amd64" && fields["target_arch"] != "arm64" {
		return releaseManifest{}, fmt.Errorf("invalid target architecture %q", fields["target_arch"])
	}
	return releaseManifest{
		FormatVersion: 1,
		DashVersion:   fields["dash_version"],
		NodeVersion:   fields["node_version"],
		TargetOS:      fields["target_os"],
		TargetArch:    fields["target_arch"],
		NodeSHA256:    nodeSHA256,
	}, nil
}
