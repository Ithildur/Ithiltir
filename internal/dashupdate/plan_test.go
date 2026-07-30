//go:build linux

package dashupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanRequiresDigestRevision(t *testing.T) {
	plan := Plan{
		Action:                  ActionUpdate,
		Channel:                 ChannelRelease,
		TargetVersion:           "1.1.0",
		ExpectedCurrentVersion:  "1.0.0",
		ExpectedInstallRevision: "not-a-digest",
		Origin:                  OriginManual,
		Lang:                    "en",
		ServiceManager:          "auto",
	}
	if err := plan.validate(); err == nil {
		t.Fatal("Plan.validate() accepted a non-digest install revision")
	}
	plan.ExpectedInstallRevision = strings.Repeat("a", 64)
	if err := plan.validate(); err != nil {
		t.Fatalf("Plan.validate() error = %v", err)
	}
}

func TestInspectInstalledRevisionTracksLegacyBinary(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(filepath.Join(home, "configs"), 0o700); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	dash := filepath.Join(bin, "dash")
	writeVersionBinary(t, dash, "1.0.0", "first")
	first, err := inspectInstalled(context.Background(), home)
	if err != nil {
		t.Fatalf("inspectInstalled(first) error = %v", err)
	}
	writeVersionBinary(t, dash, "1.0.0", "second")
	second, err := inspectInstalled(context.Background(), home)
	if err != nil {
		t.Fatalf("inspectInstalled(second) error = %v", err)
	}
	if !first.Legacy || !second.Legacy || first.Revision == second.Revision {
		t.Fatalf("legacy revisions did not track binary content: first=%+v second=%+v", first, second)
	}
}

func writeVersionBinary(t *testing.T, path, version, marker string) {
	t.Helper()
	body := "#!/bin/sh\n# " + marker + "\nprintf '%s\\n' '" + version + "'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write version binary: %v", err)
	}
}
