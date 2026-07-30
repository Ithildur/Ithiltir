package dashupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReleaseSourceUsesSemanticVersionChannel(t *testing.T) {
	asset := func() []githubAsset {
		return []githubAsset{{Name: releaseAssetName(), URL: "https://example.com/dash.tar.gz", Size: 123}}
	}
	releases := []githubRelease{
		{TagName: "9.0.0", Prerelease: true, Assets: asset()},
		{TagName: "1.2.0", Assets: asset()},
		{TagName: "1.3.0-beta.1", Prerelease: true, Assets: asset()},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Fatalf("encode releases: %v", err)
		}
	}))
	defer server.Close()
	source := releaseSource{client: server.Client(), apiURL: server.URL}

	stable, err := source.Latest(context.Background(), ChannelRelease)
	if err != nil {
		t.Fatalf("Latest(release) error = %v", err)
	}
	if stable.Version != "1.2.0" {
		t.Fatalf("Latest(release) = %s, want 1.2.0", stable.Version)
	}
	prerelease, err := source.Latest(context.Background(), ChannelPrerelease)
	if err != nil {
		t.Fatalf("Latest(prerelease) error = %v", err)
	}
	if prerelease.Version != "1.3.0-beta.1" {
		t.Fatalf("Latest(prerelease) = %s, want 1.3.0-beta.1", prerelease.Version)
	}
	if _, err := source.Find(context.Background(), "9.0.0"); err == nil {
		t.Fatal("Find() accepted a stable semantic version marked as prerelease")
	}
}
