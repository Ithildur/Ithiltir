package version

import (
	"errors"
	"fmt"
	"strings"

	modsemver "golang.org/x/mod/semver"
)

const (
	// Allowed node version range (inclusive lower bound, exclusive upper bound).
	nodeMin = "0.0.0-0"
	nodeMax = ""
)

const FormatNote = "version format: MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD] (strict SemVer without a v prefix; numeric identifiers must not contain leading zeroes)"

type Channel string

const (
	ChannelRelease    Channel = "release"
	ChannelPrerelease Channel = "prerelease"
)

func Validate(version string) error {
	_, err := normalize(version)
	return err
}

func ValidateNodeVersion(version string) error {
	if err := Validate(version); err != nil {
		return invalidVersionErr()
	}

	if nodeMin != "" {
		cmp, err := compareBound(version, nodeMin, "nodeMin")
		if err != nil {
			return err
		}
		if cmp < 0 {
			return invalidVersionErr()
		}
	}

	if nodeMax != "" {
		cmp, err := compareBound(version, nodeMax, "nodeMax")
		if err != nil {
			return err
		}
		if cmp >= 0 {
			return invalidVersionErr()
		}
	}

	return nil
}

func IsNodeOutdated(version string) (bool, error) {
	if err := Validate(version); err != nil {
		return false, err
	}
	if nodeMin == "" {
		return false, nil
	}
	cmp, err := compareBound(version, nodeMin, "nodeMin")
	if err != nil {
		return false, err
	}
	return cmp < 0, nil
}

func ChannelFor(version string) (Channel, error) {
	v, err := normalize(version)
	if err != nil {
		return "", err
	}
	if modsemver.Prerelease(v) != "" {
		return ChannelPrerelease, nil
	}
	return ChannelRelease, nil
}

func Compare(a, b string) (int, error) {
	av, err := normalize(a)
	if err != nil {
		return 0, fmt.Errorf("left version: %w", err)
	}
	bv, err := normalize(b)
	if err != nil {
		return 0, fmt.Errorf("right version: %w", err)
	}
	return modsemver.Compare(av, bv), nil
}

func Latest(versions []string, channel Channel) (string, bool) {
	var latest string
	for _, raw := range versions {
		version := strings.TrimSpace(raw)
		if version == "" {
			continue
		}
		tagChannel, err := ChannelFor(version)
		if err != nil {
			continue
		}
		if tagChannel != channel && !(channel == ChannelPrerelease && tagChannel == ChannelRelease) {
			continue
		}
		if latest == "" {
			latest = version
			continue
		}
		cmp, err := Compare(version, latest)
		if err == nil && cmp > 0 {
			latest = version
		}
	}
	return latest, latest != ""
}

func ParseChannel(raw string) (Channel, error) {
	switch Channel(strings.TrimSpace(raw)) {
	case ChannelRelease:
		return ChannelRelease, nil
	case ChannelPrerelease:
		return ChannelPrerelease, nil
	default:
		return "", fmt.Errorf("invalid channel %q", raw)
	}
}

func compareBound(version, bound, name string) (int, error) {
	cmp, err := Compare(version, bound)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return cmp, nil
}

func invalidVersionErr() error {
	return errors.New("invalid version: " + FormatNote)
}

func normalize(raw string) (string, error) {
	version := strings.TrimSpace(raw)
	if version == "" || strings.HasPrefix(version, "v") || strings.ContainsAny(version, " \t\r\n") {
		return "", errors.New(FormatNote)
	}

	core := version
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		core = core[:i]
	}
	if strings.Count(core, ".") != 2 {
		return "", errors.New(FormatNote)
	}

	prefixed := "v" + version
	if !modsemver.IsValid(prefixed) {
		return "", errors.New(FormatNote)
	}
	return prefixed, nil
}
