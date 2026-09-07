package daemonconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"encr.dev/internal/version"
)

func SocketPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("get user cache dir: %w", err)
	}

	dir, err := socketDir(version.Channel)
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, dir, "encored.sock"), nil
}

func socketDir(channel version.ReleaseChannel) (string, error) {
	switch channel {
	case version.DevBuild:
		return "encore-develop", nil
	case version.Beta:
		return "encore-beta", nil
	case version.Nightly:
		return "encore-nightly", nil
	case version.GA:
		return "encore", nil
	default:
		return "", fmt.Errorf("unknown release channel %q", channel)
	}
}
