package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
)

var ErrUnknownVersion = errors.New("unknown version")

// Check checks for the latest Encore version.
// It reports ErrUnknownVersion if it cannot determine the version.
func Check(ctx context.Context) (semver string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update.Check: %w", err)
		}
	}()

	url := "https://encore.dev/api/releases"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GET %s: responded with %s: %s", url, resp.Status, body)
	}

	var respData map[string]struct{ Version string }
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("GET %s: invalid json: %v", url, err)
	}

	key := ""
	switch runtime.GOOS {
	case "windows":
		key = "windows_amd64"
	case "darwin":
		key = "darwin_" + runtime.GOARCH
	default:
		key = "linux_amd64"
	}
	if ver := respData[key].Version; ver != "" {
		return "v" + ver, nil
	}
	return "", ErrUnknownVersion
}
