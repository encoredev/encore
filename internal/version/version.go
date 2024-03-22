package version

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"runtime/debug"
	"strings"

	"golang.org/x/mod/semver"

	"encr.dev/internal/conf"
)

// Version is the version of the encore binary.
// It is set using `go build -ldflags "-X encr.dev/internal/version.Version=v1.2.3"`.
var Version string

// Channel tells us which ReleaseChannel this build of Encore is under
var Channel ReleaseChannel

type ReleaseChannel string

const (
	GA       ReleaseChannel = "ga"      // A general availability release of Encore in Semver: v1.10.0
	Beta     ReleaseChannel = "beta"    // A beta build of an upcoming Encore release: v1.10.0-beta.1
	Nightly  ReleaseChannel = "nightly" // A nightly build of Encore with the date of the build: v1.10.0-nightly.20221231
	DevBuild ReleaseChannel = "develop" // A development build of Encore with the commit of the build: v0.0.0-develop+0140ab0f78fd10d52673a961e900993b64b7b9e3
	unknown  ReleaseChannel = "unknown" // An unknown release stream (not exported as it should be an error case)
)

// ConfigHash reports a hash of the configuration that affects the behavior of the daemon.
// It is used to decide whether to restart the daemon.
func ConfigHash() (string, error) {
	h := sha256.New()
	configDir, err := conf.Dir()
	if err != nil {
		return "", err
	}

	fmt.Fprintf(h, "APIBaseURL=%s\n", conf.APIBaseURL)
	fmt.Fprintf(h, "ConfigDir=%s\n", configDir)

	digest := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(digest), nil
}

func init() {
	// If version is already set via a compiler link flag, then we don't need to do anything
	if Version == "" {
		// Otherwise, we want to read the information from this built binary
		Version = "v0.0.0-develop"

		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		// Add the commit info
		vcsVersion := ""
		vcsModified := ""
		for _, p := range info.Settings {
			switch p.Key {
			case "vcs.revision":
				vcsVersion = p.Value
			case "vcs.modified":
				if p.Value == "true" {
					vcsModified = "-modified"
				}
			}
		}
		if vcsVersion != "" {
			Version += "+" + vcsVersion + vcsModified
		}
	}
	Channel = ChannelFor(Version)
}

func ChannelFor(version string) ReleaseChannel {
	if !strings.HasPrefix(version, "v") {
		return unknown
	}
	// Now work out the release channel
	switch {
	case strings.Contains(version, "-beta."):
		return Beta
	case strings.Contains(version, "-nightly."):
		return Nightly
	case strings.HasSuffix(version, "-develop") || strings.Contains(version, "-develop+"):
		return DevBuild
	default:
		return GA
	}
}

// Compare compares this version of Encore against another version
// accounting for the release channel.
//
// If the releases are from the same channel, then it returns:
//   - 0 if the versions are the same
//   - a negative number if this version is older than the other
//   - a positive number if this version is newer than the other
//
// If the releases are from different channels, it always returns 1.
func Compare(againstVersion string) int {
	againstChannel := ChannelFor(againstVersion)

	if Channel != againstChannel {
		// If the channels are different, this "version" is always newer
		return 1
	}

	switch Channel {
	case GA, Beta, Nightly:
		return semver.Compare(Version, againstVersion)
	case DevBuild:
		return 0 // devel versions are always the same
	default:
		return 0 // never newer if we can't test
	}
}
