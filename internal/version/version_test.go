package version

import "testing"

func TestChannelFor(t *testing.T) {
	tests := []struct {
		version string
		want    Channel
	}{
		{version: "1.2.3", want: ChannelRelease},
		{version: "1.2.3+build.7", want: ChannelRelease},
		{version: "1.2.3-alpha", want: ChannelPrerelease},
		{version: "1.2.3-alpha.1+build.7", want: ChannelPrerelease},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := ChannelFor(tt.version)
			if err != nil {
				t.Fatalf("ChannelFor(%q) returned error: %v", tt.version, err)
			}
			if got != tt.want {
				t.Fatalf("ChannelFor(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestValidateRejectsNonContractVersions(t *testing.T) {
	tests := []string{
		"",
		"v1.2.3",
		"1.2",
		"1.2.3.4",
		"1.02.3",
		"1.2.3-01",
		"1.2.3+",
		"1.2.3 alpha",
	}

	for _, version := range tests {
		t.Run(version, func(t *testing.T) {
			if err := Validate(version); err == nil {
				t.Fatalf("Validate(%q) succeeded, want error", version)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "prerelease below release", left: "1.0.0-alpha", right: "1.0.0", want: -1},
		{name: "build metadata ignored", left: "1.0.0+1", right: "1.0.0+2", want: 0},
		{name: "patch", left: "1.0.1", right: "1.0.0", want: 1},
		{name: "numeric prerelease", left: "1.0.0-alpha.2", right: "1.0.0-alpha.10", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compare(tt.left, tt.right)
			if err != nil {
				t.Fatalf("Compare(%q, %q) returned error: %v", tt.left, tt.right, err)
			}
			if got != tt.want {
				t.Fatalf("Compare(%q, %q) = %d, want %d", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestIsNodeUpdateTarget(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		want    bool
	}{
		{name: "new patch", current: "1.0.0", target: "1.0.1", want: true},
		{name: "same version", current: "1.0.0", target: "1.0.0", want: false},
		{name: "different build metadata", current: "1.0.0+build.1", target: "1.0.0+build.2", want: true},
		{name: "older target", current: "1.0.1", target: "1.0.0", want: false},
		{name: "release target from prerelease", current: "1.0.0-rc.1", target: "1.0.0", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsNodeUpdateTarget(tt.current, tt.target)
			if err != nil {
				t.Fatalf("IsNodeUpdateTarget(%q, %q) error = %v", tt.current, tt.target, err)
			}
			if got != tt.want {
				t.Fatalf("IsNodeUpdateTarget(%q, %q) = %t, want %t", tt.current, tt.target, got, tt.want)
			}
		})
	}
}

func TestSupportsNodeSelfUpdate(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "0.2.2", want: false},
		{version: "0.2.3", want: true},
		{version: "0.2.3+build.1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := SupportsNodeSelfUpdate(tt.version)
			if err != nil {
				t.Fatalf("SupportsNodeSelfUpdate(%q) error = %v", tt.version, err)
			}
			if got != tt.want {
				t.Fatalf("SupportsNodeSelfUpdate(%q) = %t, want %t", tt.version, got, tt.want)
			}
		})
	}
}

func TestLatestCompatible(t *testing.T) {
	versions := []string{
		"bad",
		"1.2.3-alpha.1",
		"1.2.3",
		"1.2.4-alpha.1",
		"1.2.3+build.2",
	}

	got, ok := LatestCompatible(versions, ChannelRelease)
	if !ok || got != "1.2.3" {
		t.Fatalf("LatestCompatible(release) = %q, %v; want %q, true", got, ok, "1.2.3")
	}

	got, ok = LatestCompatible(versions, ChannelPrerelease)
	if !ok || got != "1.2.4-alpha.1" {
		t.Fatalf("LatestCompatible(prerelease) = %q, %v; want %q, true", got, ok, "1.2.4-alpha.1")
	}

	tests := []struct {
		name    string
		channel Channel
		values  []string
		want    string
		wantOK  bool
	}{
		{
			name:    "prerelease falls back to release when prerelease is older",
			channel: ChannelPrerelease,
			values:  []string{"1.2.3", "1.2.3-alpha.1"},
			want:    "1.2.3",
			wantOK:  true,
		},
		{
			name:    "prerelease falls back to release when no prerelease exists",
			channel: ChannelPrerelease,
			values:  []string{"1.2.3"},
			want:    "1.2.3",
			wantOK:  true,
		},
		{
			name:    "prerelease can use prerelease when no release exists",
			channel: ChannelPrerelease,
			values:  []string{"1.2.4-alpha.1"},
			want:    "1.2.4-alpha.1",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LatestCompatible(tt.values, tt.channel)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("LatestCompatible() = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestLatestInChannel(t *testing.T) {
	versions := []string{
		"bad",
		"1.2.3-alpha.1",
		"1.2.3",
		"1.2.4-alpha.1",
		"1.2.5",
	}

	got, ok := LatestInChannel(versions, ChannelRelease)
	if !ok || got != "1.2.5" {
		t.Fatalf("LatestInChannel(release) = %q, %v; want %q, true", got, ok, "1.2.5")
	}

	got, ok = LatestInChannel(versions, ChannelPrerelease)
	if !ok || got != "1.2.4-alpha.1" {
		t.Fatalf("LatestInChannel(prerelease) = %q, %v; want %q, true", got, ok, "1.2.4-alpha.1")
	}
}
