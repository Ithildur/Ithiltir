package dashupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	appversion "dash/internal/version"
)

const installedVersionTimeout = 10 * time.Second

type Plan struct {
	Action                  Action
	Channel                 Channel
	TargetVersion           string
	ExpectedCurrentVersion  string
	ExpectedInstallRevision string
	Origin                  Origin
	Lang                    string
	ServiceManager          string
}

type installedState struct {
	Version  string
	Revision string
	Target   string
	Legacy   bool
}

func (p Plan) validate() error {
	if _, ok := ParseAction(p.Action); !ok {
		return fmt.Errorf("invalid action %q", p.Action)
	}
	channel, ok := ParseChannel(p.Channel)
	if !ok {
		return fmt.Errorf("invalid channel %q", p.Channel)
	}
	if err := appversion.Validate(p.TargetVersion); err != nil {
		return fmt.Errorf("invalid target version: %w", err)
	}
	if err := appversion.Validate(p.ExpectedCurrentVersion); err != nil {
		return fmt.Errorf("invalid expected current version: %w", err)
	}
	revision := strings.TrimSpace(p.ExpectedInstallRevision)
	if len(revision) != sha256.Size*2 {
		return errors.New("expected install revision must be a SHA-256 digest")
	}
	if _, err := hex.DecodeString(revision); err != nil {
		return errors.New("expected install revision must be a SHA-256 digest")
	}
	targetChannel, err := appversion.ChannelFor(p.TargetVersion)
	if err != nil {
		return fmt.Errorf("target version channel: %w", err)
	}
	if targetChannel != channel {
		return fmt.Errorf("target version %s does not belong to %s channel", p.TargetVersion, channel)
	}
	if _, ok := normalizeUpdateLang(p.Lang); !ok {
		return fmt.Errorf("invalid language %q", p.Lang)
	}
	if _, ok := ParseOrigin(p.Origin); !ok {
		return fmt.Errorf("invalid origin %q", p.Origin)
	}
	switch strings.TrimSpace(p.ServiceManager) {
	case "", "auto", "systemd", "none":
	default:
		return fmt.Errorf("invalid service manager %q", p.ServiceManager)
	}
	return nil
}

func inspectInstalled(ctx context.Context, home string) (installedState, error) {
	home = filepath.Clean(strings.TrimSpace(home))
	if !filepath.IsAbs(home) || home == string(filepath.Separator) {
		return installedState{}, fmt.Errorf("invalid Dash home %q", home)
	}
	current := filepath.Join(home, "current")
	bin := filepath.Join(home, "bin", "dash")

	state := installedState{}
	info, err := os.Lstat(current)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink == 0 {
			return installedState{}, fmt.Errorf("dash current path is not a symlink: %s", current)
		}
		target, readErr := os.Readlink(current)
		if readErr != nil {
			return installedState{}, fmt.Errorf("read Dash current target: %w", readErr)
		}
		if !validReleaseTarget(target) {
			return installedState{}, fmt.Errorf("invalid Dash current target %q", target)
		}
		state.Target = target
	case errors.Is(err, os.ErrNotExist):
		state.Legacy = true
	default:
		return installedState{}, fmt.Errorf("inspect Dash current path: %w", err)
	}

	version, err := binaryVersion(ctx, bin)
	if err != nil {
		return installedState{}, fmt.Errorf("inspect installed Dash version: %w", err)
	}
	state.Version = version

	h := sha256.New()
	if state.Legacy {
		if _, err := io.WriteString(h, "legacy\x00"); err != nil {
			return installedState{}, err
		}
		f, err := os.Open(bin)
		if err != nil {
			return installedState{}, fmt.Errorf("open legacy Dash binary: %w", err)
		}
		_, hashErr := io.Copy(h, f)
		closeErr := f.Close()
		if hashErr != nil || closeErr != nil {
			return installedState{}, errors.Join(hashErr, closeErr)
		}
	} else {
		_, _ = io.WriteString(h, "release\x00"+state.Target)
	}
	_, _ = io.WriteString(h, "\x00"+version)
	state.Revision = hex.EncodeToString(h.Sum(nil))
	return state, nil
}

func binaryVersion(ctx context.Context, path string) (string, error) {
	return commandVersion(ctx, path, "--version")
}

func updateCommandVersion(ctx context.Context, path string) (string, error) {
	return commandVersion(ctx, path, "update", "--version")
}

func updateCommandNodeVersion(ctx context.Context, path string) (string, error) {
	version, err := commandVersion(ctx, path, "update", "--node-version")
	if err != nil {
		return "", err
	}
	if err := appversion.ValidateNodeVersion(version); err != nil {
		return "", fmt.Errorf("%s returned invalid node version %q: %w", path, version, err)
	}
	return version, nil
}

func commandVersion(ctx context.Context, path string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, installedVersionTimeout)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, path, args...).CombinedOutput()
	if cmdCtx.Err() != nil {
		return "", cmdCtx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", path, strings.Join(args, " "), err)
	}
	version := strings.TrimSpace(string(out))
	if strings.ContainsAny(version, "\r\n") {
		return "", fmt.Errorf("%s returned multiple version lines", path)
	}
	if err := appversion.Validate(version); err != nil {
		return "", fmt.Errorf("%s returned invalid version %q: %w", path, version, err)
	}
	return version, nil
}

func validReleaseTarget(target string) bool {
	if filepath.IsAbs(target) || target != filepath.Clean(target) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(target), "/")
	return len(parts) == 2 && parts[0] == "releases" && parts[1] != "" && parts[1] != "." && parts[1] != ".."
}
