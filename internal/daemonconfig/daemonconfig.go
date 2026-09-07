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

type Ports struct {
	Dashboard     uint16
	DatabaseProxy uint16
	Runtime       uint16
	Debug         uint16
	ObjectStorage uint16
	MCP           uint16
}

// These is currently no alpha release
// To leave room for it offset 1 is skipped
//
// 	 Service           Stable    Beta    Nightly   Develop
//  ━━━━━━━━━━━━━━━━  ━━━━━━━━  ━━━━━━  ━━━━━━━━  ━━━━━━━━━━
//   Dashboard           9400    9402       9403      9404
//  ────────────────  ────────  ──────  ────────  ──────────
//   Database proxy      9500    9502       9503      9504
//  ────────────────  ────────  ──────  ────────  ──────────
//   Runtime             9600    9602       9603      9604
//  ────────────────  ────────  ──────  ────────  ──────────
//   Debug               9700    9702       9703      9704
//  ────────────────  ────────  ──────  ────────  ──────────
//   Object storage      9800    9802       9803      9804
//  ────────────────  ────────  ──────  ────────  ──────────
//   MCP                 9900    9902       9903      9904
func DefaultPorts() (Ports, error) {
	var offset uint16
	switch version.Channel {
	case version.GA:
		offset = 0
	case version.Beta:
		offset = 2
	case version.Nightly:
		offset = 3
	case version.DevBuild:
		offset = 4
	default:
		return Ports{}, fmt.Errorf(
			"unknown release channel %q", version.Channel,
		)
	}

	return Ports{
		Dashboard:     9400 + offset,
		DatabaseProxy: 9500 + offset,
		Runtime:       9600 + offset,
		Debug:         9700 + offset,
		ObjectStorage: 9800 + offset,
		MCP:           9900 + offset,
	}, nil
}
