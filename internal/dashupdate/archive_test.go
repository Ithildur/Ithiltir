//go:build linux

package dashupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtractReleaseRejectsUnsafeEntries(t *testing.T) {
	tests := []struct {
		name   string
		header tar.Header
		want   string
	}{
		{
			name:   "traversal",
			header: tar.Header{Name: "Ithiltir-dash/../../etc/passwd", Typeflag: tar.TypeReg},
			want:   "unsafe path",
		},
		{
			name:   "symlink",
			header: tar.Header{Name: "Ithiltir-dash/bin/dash", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"},
			want:   "not a regular file or directory",
		},
		{
			name:   "wrong root",
			header: tar.Header{Name: "other/bin/dash", Typeflag: tar.TypeReg},
			want:   "unexpected root",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "release.tar.gz")
			writeArchive(t, archive, []tar.Header{tt.header})
			_, err := extractRelease(archive, filepath.Join(t.TempDir(), "extract"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("extractRelease() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestReadReleaseManifestIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.env")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxReleaseManifestBytes+1)), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	_, err := readReleaseManifest(path)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("readReleaseManifest() error = %v, want size limit", err)
	}
}

func TestValidateReleasePackageRequiresVersionMatchedDash(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeReleaseRuntime(t, root)
	writeReleaseManifest(t, root, "1.2.3", "1.0.0")
	writeReleaseBinary(t, filepath.Join(bin, "dash"), "1.2.3", "1.0.0", "dash")
	if _, err := validateReleasePackage(context.Background(), root, "1.2.3"); err != nil {
		t.Fatalf("validateReleasePackage() error = %v", err)
	}

	writeReleaseBinary(t, filepath.Join(bin, "dash"), "1.2.4", "1.0.0", "dash-mismatch")
	if _, err := validateReleasePackage(context.Background(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("validateReleasePackage() mismatch error = %v", err)
	}

	writeReleaseBinary(t, filepath.Join(bin, "dash"), "1.2.3", "1.0.1", "node-mismatch")
	if _, err := validateReleasePackage(context.Background(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), "bundled node version does not match") {
		t.Fatalf("validateReleasePackage() node mismatch error = %v", err)
	}

	serverOnly := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '1.2.3\\n'; exit 0; fi\nexit 2\n"
	if err := os.WriteFile(filepath.Join(bin, "dash"), []byte(serverOnly), 0o755); err != nil {
		t.Fatalf("write server-only Dash: %v", err)
	}
	if _, err := validateReleasePackage(context.Background(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), "update --version") {
		t.Fatalf("validateReleasePackage() missing update command error = %v", err)
	}
}

func TestValidateReleasePackageRejectsChangedNodeAsset(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeReleaseRuntime(t, root)
	writeReleaseManifest(t, root, "1.2.3", "1.0.0")
	writeReleaseBinary(t, filepath.Join(root, "bin", "dash"), "1.2.3", "1.0.0", "dash")

	changed := releaseNodeAssets[0].path
	f, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(changed)), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", changed, err)
	}
	if _, err := f.WriteString("changed\n"); err != nil {
		_ = f.Close()
		t.Fatalf("change %s: %v", changed, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", changed, err)
	}

	if _, err := validateReleasePackage(context.Background(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), changed) {
		t.Fatalf("validateReleasePackage() changed asset error = %v, want %q", err, changed)
	}
}

func TestValidateReleasePackageRequiresRuntimeTree(t *testing.T) {
	for _, missing := range []string{
		"dist/index.html",
		"deploy/windows/runner_windows_amd64.exe",
		"deploy/windows/runner_windows_arm64.exe",
	} {
		t.Run(missing, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
				t.Fatalf("mkdir bin: %v", err)
			}
			writeReleaseRuntime(t, root)
			writeReleaseManifest(t, root, "1.2.3", "1.0.0")
			writeReleaseBinary(t, filepath.Join(root, "bin", "dash"), "1.2.3", "1.0.0", "dash")
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(missing))); err != nil {
				t.Fatalf("remove %s: %v", missing, err)
			}
			if _, err := validateReleasePackage(context.Background(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("validateReleasePackage() missing runtime error = %v, want %q", err, missing)
			}
		})
	}
}

func writeReleaseManifest(t *testing.T, root, dashVersion, nodeVersion string) {
	t.Helper()
	lines := []string{
		"format_version=1",
		"dash_version=" + dashVersion,
		"node_version=" + nodeVersion,
		"target_os=linux",
		"target_arch=" + runtime.GOARCH,
	}
	for _, asset := range releaseNodeAssets {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(asset.path)))
		if err != nil {
			t.Fatalf("read %s: %v", asset.path, err)
		}
		sum := sha256.Sum256(raw)
		lines = append(lines, asset.field+"="+hex.EncodeToString(sum[:]))
	}
	if err := os.WriteFile(filepath.Join(root, "release.env"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write release.env: %v", err)
	}
}

func writeReleaseBinary(t *testing.T, path, dashVersion, nodeVersion, marker string) {
	t.Helper()
	body := "#!/bin/sh\n# " + marker + "\n" +
		"if [ \"$1\" = \"update\" ] && [ \"$2\" = \"--version\" ]; then printf '%s\\n' '" + dashVersion + "'; exit 0; fi\n" +
		"if [ \"$1\" = \"update\" ] && [ \"$2\" = \"--node-version\" ]; then printf '%s\\n' '" + nodeVersion + "'; exit 0; fi\n" +
		"exit 2\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write release binary: %v", err)
	}
}

func writeReleaseRuntime(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{"configs", "deploy", "dist"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, file := range []string{
		"configs/config.example.yaml",
		"deploy/linux/node_linux_amd64",
		"deploy/linux/node_linux_arm64",
		"deploy/macos/node_macos_arm64",
		"deploy/windows/node_windows_amd64.exe",
		"deploy/windows/node_windows_arm64.exe",
		"deploy/windows/runner_windows_amd64.exe",
		"deploy/windows/runner_windows_arm64.exe",
		"dist/index.html",
		"install_dash_linux.sh",
		"update_dash_linux.sh",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(file))), 0o755); err != nil {
			t.Fatalf("mkdir parent for %s: %v", file, err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file)), []byte("test\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
}

func writeArchive(t *testing.T, path string, headers []tar.Header) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for i := range headers {
		header := headers[i]
		if err := tw.WriteHeader(&header); err != nil {
			t.Fatalf("write archive header: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
}
