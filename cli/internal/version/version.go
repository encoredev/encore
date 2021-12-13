package version

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
)

// Version is the version of the encore binary.
// It is set using `go build -ldflags "-X encr.dev/cli/internal/version.Version=v1.2.3"`.
var Version string

// ConfigHash reports a hash of the configuration that affects the behavior of the daemon.
// It is used to decide whether to restart the daemon.
func ConfigHash() string {
	envs := []string{
		"ENCORE_DAEMON_DEV",
		"ENCORE_PLATFORM_API_URL",
		"ENCORE_CONFIG_DIR",
	}
	h := sha256.New()
	for _, e := range envs {
		fmt.Fprintf(h, "%s=%q\n", e, os.Getenv(e))
	}
	digest := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(digest)
}
