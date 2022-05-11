package version

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"runtime/debug"

	"encr.dev/cli/internal/conf"
)

// Version is the version of the encore binary.
// It is set using `go build -ldflags "-X encr.dev/cli/internal/version.Version=v1.2.3"`.
var Version string

// ConfigHash reports a hash of the configuration that affects the behavior of the daemon.
// It is used to decide whether to restart the daemon.
func ConfigHash() (string, error) {
	envs := []string{
		"ENCORE_DAEMON_DEV",
		"ENCORE_PLATFORM_API_URL",
		"ENCORE_CONFIG_DIR",
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
	if Version != "" {
		return
	}

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
