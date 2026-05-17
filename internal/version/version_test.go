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

func TestLatest(t *testing.T) {
	versions := []string{
		"bad",
		"1.2.3-alpha.1",
		"1.2.3",
		"1.2.4-alpha.1",
		"1.2.3+build.2",
	}

	got, ok := Latest(versions, ChannelRelease)
	if !ok || got != "1.2.3" {
		t.Fatalf("Latest(release) = %q, %v; want %q, true", got, ok, "1.2.3")
	}

	got, ok = Latest(versions, ChannelPrerelease)
	if !ok || got != "1.2.4-alpha.1" {
		t.Fatalf("Latest(prerelease) = %q, %v; want %q, true", got, ok, "1.2.4-alpha.1")
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
			got, ok := Latest(tt.values, tt.channel)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("Latest() = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
