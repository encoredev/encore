package version

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

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
	Nightly  ReleaseChannel = "nightly" // A nightly build of Encore with the date of the build: nightly-20221231
	DevBuild ReleaseChannel = "devel"   // A development build of Encore with the commit of the build: devel-0140ab0f78fd10d52673a961e900993b64b7b9e3
	unknown  ReleaseChannel = "unknown" // An unknown release stream (not exported as it should be an error case)
)

// ConfigHash reports a hash of the configuration that affects the behavior of the daemon.
// It is used to decide whether to restart the daemon.
func ConfigHash() (string, error) {
	envs := []string{
		"ENCORE_PLATFORM_API_URL",
		"ENCORE_CONFIG_DIR",
		"ENCORE_EXPERIMENT",
	}
	h := sha256.New()
	for _, e := range envs {
		fmt.Fprintf(h, "%s=%q\n", e, os.Getenv(e))
	}

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
		Version = "devel"

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
			Version += "-" + vcsVersion + vcsModified
		}
	}

	// Now work out the release channel
	switch {
	case strings.HasPrefix(Version, "v"):
		Channel = GA
	case strings.HasPrefix(Version, "nightly-"):
		Channel = Nightly
	case strings.HasPrefix(Version, "devel-"):
		Channel = DevBuild
	default:
		Channel = unknown
	}
}
