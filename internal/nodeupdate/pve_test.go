package nodeupdate

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPVEAssetsCompatibilityAndIntegrity(t *testing.T) {
	dir := t.TempDir()
	if present, err := ValidatePVEAssets(dir, "1.0.0"); present || err != nil {
		t.Fatalf("legacy package: %v %v", present, err)
	}
	if err := os.Mkdir(filepath.Join(dir, "linux"), 0755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("test binary")
	for _, arch := range []string{"amd64", "arm64"} {
		if err := os.WriteFile(filepath.Join(dir, "linux", "pve_cache_linux_"+arch), raw, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ValidatePVEAssets(dir, "1.0.0"); err == nil {
		t.Fatal("accepted assets without manifest")
	}
	manifest := fmt.Sprintf("format_version=1\nnode_version=1.0.0\namd64_sha256=%x\narm64_sha256=%x\n", sha256.Sum256(raw), sha256.Sum256(raw))
	if err := os.WriteFile(filepath.Join(dir, "linux", "pve-cache.env"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if present, err := ValidatePVEAssets(dir, "1.0.0"); !present || err != nil {
		t.Fatalf("valid package: %v %v", present, err)
	}
	if _, err := ValidatePVEAssets(dir, "1.0.1"); err == nil {
		t.Fatal("accepted wrong node version")
	}
	if err := os.WriteFile(filepath.Join(dir, "linux", "pve_cache_linux_amd64"), []byte("corrupt"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePVEAssets(dir, "1.0.0"); err == nil {
		t.Fatal("accepted corrupt binary")
	}
}
