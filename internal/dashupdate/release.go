package dashupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	appversion "dash/internal/version"
)

const (
	updateReleaseAPI      = "https://api.github.com/repos/Ithildur/Ithiltir/releases"
	updateReleasePageSize = 100
	updateReleaseMaxPages = 10
	updateReleaseMaxBytes = 4 << 20
)

type releaseAsset struct {
	Version string
	URL     string
	Size    int64
}

type releaseSource struct {
	client *http.Client
	apiURL string
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

func newReleaseSource() releaseSource {
	return releaseSource{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				if !strings.EqualFold(req.URL.Scheme, "https") {
					return fmt.Errorf("release redirect must use HTTPS")
				}
				return nil
			},
		},
		apiURL: updateReleaseAPI,
	}
}

func (s releaseSource) Latest(ctx context.Context, channel Channel) (releaseAsset, error) {
	releases, err := s.list(ctx)
	if err != nil {
		return releaseAsset{}, err
	}

	var latest releaseAsset
	for _, item := range releases {
		asset, ok := releaseAssetFor(item)
		if !ok || !releaseInChannel(item, channel) {
			continue
		}
		if latest.Version == "" {
			latest = asset
			continue
		}
		cmp, compareErr := appversion.Compare(asset.Version, latest.Version)
		if compareErr == nil && cmp > 0 {
			latest = asset
		}
	}
	if latest.Version == "" {
		return releaseAsset{}, fmt.Errorf("no published %s release with asset %s", channel, releaseAssetName())
	}
	return latest, nil
}

func (s releaseSource) Find(ctx context.Context, version string) (releaseAsset, error) {
	if err := appversion.Validate(version); err != nil {
		return releaseAsset{}, err
	}
	releases, err := s.list(ctx)
	if err != nil {
		return releaseAsset{}, err
	}
	for _, item := range releases {
		if item.Draft || strings.TrimSpace(item.TagName) != version {
			continue
		}
		channel, err := appversion.ChannelFor(version)
		if err != nil || item.Prerelease != (channel == appversion.ChannelPrerelease) {
			return releaseAsset{}, fmt.Errorf("release %s metadata does not match its semantic version channel", version)
		}
		if asset, ok := releaseAssetFor(item); ok {
			return asset, nil
		}
		return releaseAsset{}, fmt.Errorf("release %s is missing asset %s", version, releaseAssetName())
	}
	return releaseAsset{}, fmt.Errorf("published release %s not found", version)
}

func (s releaseSource) list(ctx context.Context) ([]githubRelease, error) {
	if s.client == nil {
		return nil, errors.New("release HTTP client is nil")
	}
	if strings.TrimSpace(s.apiURL) == "" {
		return nil, errors.New("release API URL is empty")
	}

	var all []githubRelease
	for page := 1; page <= updateReleaseMaxPages; page++ {
		items, err := s.page(ctx, page)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < updateReleasePageSize {
			return all, nil
		}
	}
	return nil, fmt.Errorf("release list exceeds %d pages", updateReleaseMaxPages)
}

func (s releaseSource) page(ctx context.Context, page int) ([]githubRelease, error) {
	u, err := url.Parse(s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("parse release API URL: %w", err)
	}
	query := u.Query()
	query.Set("per_page", strconv.Itoa(updateReleasePageSize))
	query.Set("page", strconv.Itoa(page))
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Ithiltir-Dash-Updater")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release page %d: %w", page, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("fetch release page %d: HTTP %s", page, resp.Status)
	}

	limited := io.LimitReader(resp.Body, updateReleaseMaxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read release page %d: %w", page, err)
	}
	if len(raw) > updateReleaseMaxBytes {
		return nil, fmt.Errorf("release page %d exceeds %d bytes", page, updateReleaseMaxBytes)
	}
	var items []githubRelease
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode release page %d: %w", page, err)
	}
	return items, nil
}

func releaseInChannel(item githubRelease, channel Channel) bool {
	if item.Draft {
		return false
	}
	tagChannel, err := appversion.ChannelFor(strings.TrimSpace(item.TagName))
	if err != nil || tagChannel != channel {
		return false
	}
	return item.Prerelease == (tagChannel == appversion.ChannelPrerelease)
}

func releaseAssetFor(item githubRelease) (releaseAsset, bool) {
	version := strings.TrimSpace(item.TagName)
	if item.Draft || appversion.Validate(version) != nil {
		return releaseAsset{}, false
	}
	want := releaseAssetName()
	for _, asset := range item.Assets {
		if asset.Name != want || asset.Size <= 0 {
			continue
		}
		u, err := url.Parse(asset.URL)
		if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
			continue
		}
		return releaseAsset{Version: version, URL: u.String(), Size: asset.Size}, true
	}
	return releaseAsset{}, false
}

func releaseAssetName() string {
	return "Ithiltir_dash_linux_" + runtime.GOARCH + ".tar.gz"
}
