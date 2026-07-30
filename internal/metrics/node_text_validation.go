package metrics

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxNodeVersionChars   = 64
	maxNodeHostnameChars  = 255
	maxSystemNameChars    = 32
	maxSystemVersionChars = 255
	maxDiskNameChars      = 255
	maxDiskRefChars       = 320
	maxDiskKindChars      = 16
	maxDiskRoleChars      = 16
	maxDiskStateChars     = 32
	maxFSTypeChars        = 32
	maxInterfaceChars     = 64
	maxRaidHealthChars    = 16
)

func validateTextLength(path, value string, maxChars int) error {
	if utf8.RuneCountInString(strings.TrimSpace(value)) <= maxChars {
		return nil
	}
	return fmt.Errorf("%s exceeds %d characters", path, maxChars)
}

// ValidateStaticText checks the static text fields that cross into bounded
// PostgreSQL columns. Paths and hardware descriptions remain unbounded.
func ValidateStaticText(snapshot StaticMetrics) error {
	fields := []struct {
		path     string
		value    string
		maxChars int
	}{
		{"version", snapshot.Version, maxNodeVersionChars},
		{"system.hostname", snapshot.System.Hostname, maxNodeHostnameChars},
		{"system.os", snapshot.System.OS, maxSystemNameChars},
		{"system.platform", snapshot.System.Platform, maxSystemNameChars},
		{"system.platform_version", snapshot.System.PlatformVersion, maxSystemVersionChars},
		{"system.kernel_version", snapshot.System.KernelVersion, maxSystemVersionChars},
		{"system.arch", snapshot.System.Arch, maxSystemNameChars},
	}
	for _, field := range fields {
		if err := validateTextLength(field.path, field.value, field.maxChars); err != nil {
			return err
		}
	}
	for i, disk := range snapshot.Disk.Logical {
		for mountpoint, info := range disk.Mountpoints {
			path := fmt.Sprintf("disk.logical[%d].mountpoints[%q].fs_type", i, mountpoint)
			if err := validateTextLength(path, info.FSType, maxFSTypeChars); err != nil {
				return err
			}
		}
	}
	for i, filesystem := range snapshot.Disk.Filesystems {
		path := fmt.Sprintf("disk.filesystems[%d].fs_type", i)
		if err := validateTextLength(path, filesystem.FsType, maxFSTypeChars); err != nil {
			return err
		}
	}
	return nil
}
