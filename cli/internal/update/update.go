package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"

	"encr.dev/internal/conf"
	"encr.dev/internal/version"
)

var ErrUnknownVersion = errors.New("unknown version")

// Check checks for the latest Encore version.
// It reports ErrUnknownVersion if it cannot determine the version.
func Check(ctx context.Context) (latestVersion *LatestVersion, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update.Check: %w", err)
		}
	}()

	releaseAPI, err := url.Parse("https://encore.dev/api/releases")
	if err != nil {
		return nil, fmt.Errorf("parse release api url: %w", err)
	}

	// Filter the request down to the release for the current version.
	qry := releaseAPI.Query()

	// These three are used to determine the latest release for the given channel, os and arch
	qry.Set("channel", string(version.Channel))
	qry.Set("os", runtime.GOOS)
	qry.Set("arch", runtime.GOARCH)

	// This is used to determine if the returned release contains security updates not present
	// in the currently running version of Encore, as well as if we need to force an upgrade
	// on the user due to a critical security issue.
	qry.Set("current", version.Version)

	// For specific app ID's or user ID's we can provide pre-releases to them
	// Mainly used if they've encountered a bug and we need to get them a fix asap for testing
	if cfg, err := conf.CurrentUser(); err == nil && cfg != nil {
		qry.Set("actor", cfg.Actor)
	}

	releaseAPI.RawQuery = qry.Encode()

	// url := "https://encore.dev/api/releases"
	req, err := http.NewRequestWithContext(ctx, "GET", releaseAPI.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: responded with %s: %s", releaseAPI, resp.Status, body)
	}

	latestVersion = &LatestVersion{}
	if err := json.NewDecoder(resp.Body).Decode(latestVersion); err != nil {
		return nil, fmt.Errorf("GET %s: invalid json: %v", releaseAPI, err)
	}

	if !latestVersion.Supported && latestVersion.Channel != version.DevBuild {
		return nil, ErrUnknownVersion
	}

	return latestVersion, nil
}

// LatestVersion contains the parsed response from the update server
type LatestVersion struct {
	// The channel the release is from
	Channel version.ReleaseChannel `json:"channel"`

	// Whether the requested target is supported or not
	Supported bool `json:"supported"`

	// The latest version available
	// Access via Version() to ensure the version is prefixed with "v" for GA releases
	RawVersion string `json:"version"`

	// The URL for that version (if supported)
	URL string `json:"url,omitempty"`

	// Whether the version contains a security fix from the current version running
	SecurityUpdate bool `json:"security_update"`

	// Optional notes about what the security update fixes and why the user should install it
	SecurityNotes string `json:"security_notes,omitempty"`

	// If we need to force an upgrade. This is only used for security updates and only for
	// the most urgent ones, i.e we should never use it unless the world is on fire.
	ForceUpgrade bool `json:"force_upgrade,omitempty"`
}

// Version returns the version string referenced by the LatestVersion.
// ensuring that it is prefixed with "v" for GA releases.
func (lv *LatestVersion) Version() string {
	// Server side doesn't include the "v" in nightly versions.
	if lv.Channel == version.GA {
		// Note: this trim prefix is future proofing in case we decide to start returning versions
		// which include the "v" prefix
		return "v" + strings.TrimPrefix(lv.RawVersion, "v")
	}

	return lv.RawVersion
}

// IsNewer returns true if LatestVersion is newer than current
//
// This is safe to call on a nil LatestVersion
func (lv *LatestVersion) IsNewer(current string) bool {
	if lv == nil {
		return false
	}

	switch lv.Channel {
	case version.GA:
		return semver.Compare(lv.Version(), current) > 0
	case version.Nightly:
		return nightlyToNumber(lv.Version()) > nightlyToNumber(current)
	}

	return false
}

// DoUpgrade upgrades Encore.
//
// Adapted from flyctl: https://github.com/superfly/flyctl
func (lv *LatestVersion) DoUpgrade(stdout, stderr io.Writer) error {
	// What shell do we need to run?
	arg := "-c"
	shell, ok := os.LookupEnv("SHELL")
	if !ok {
		//goland:noinspection GoBoolExpressions
		if runtime.GOOS == "windows" {
			shell = "powershell.exe"
			arg = "-Command"
		} else {
			shell = "/bin/bash"
		}
	}

	// Base script for *nix systems
	script := "curl -L \"https://encore.dev/install.sh\" | sh"

	brewManaged := false

	// Script overrides for windows and systems with homebrew installed
	switch runtime.GOOS {
	case "windows":
		script = "iwr https://encore.dev/install.ps1 -useb | iex"
	case "darwin", "linux":
		// Upgrade via homebrew if we can
		if wasInstalledViaHomebrew(shell, arg, lv.Channel) {
			brewManaged = true
			script = "brew upgrade encore --fetch-head"
		}
	}

	// Sainty check we can perform the update
	switch lv.Channel {
	case version.GA:
	// no-op
	case version.Nightly:
		if brewManaged {
			script = "brew upgrade encore-nightly --fetch-head"
		} else {
			return errors.New("nightly can not be automatically updated without homebrew")
		}
	case version.DevBuild:
		return errors.New("dev builds can not be automatically updated")
	default:
		return fmt.Errorf("unknown release channel %s", lv.Channel)
	}

	fmt.Println("Running update [" + script + "]")
	// nosemgrep
	cmd := exec.Command(shell, arg, script)

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func nightlyToNumber(version string) int64 {
	// version looks like: nightly-20221010
	if !strings.HasPrefix(version, "nightly-") || len(version) != 16 {
		return 0
	}

	// slice(8) removes "nightly-"
	date, err := strconv.ParseInt(version[8:], 10, 64)
	if err != nil {
		return 0
	}

	return date
}

func wasInstalledViaHomebrew(shell string, arg string, channel version.ReleaseChannel) bool {
	if _, err := exec.LookPath("brew"); err != nil {
		return false
	}

	formulaName := "encore"
	if channel == version.Nightly {
		formulaName = "encore-nightly"
	}

	buf := new(bytes.Buffer)
	// nosemgrep
	cmd := exec.Command(shell, arg, fmt.Sprintf("brew list %s -1", formulaName))
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Stdin = os.Stdin

	// No error means it was installed via homebrew, error means homebrew doesn't know about it
	// or isn't installed
	return cmd.Run() == nil
}
