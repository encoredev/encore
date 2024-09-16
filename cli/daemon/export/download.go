package export

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/internal/conf"
	"encr.dev/internal/env"
	"encr.dev/internal/version"
	"encr.dev/pkg/dockerbuild"
)

const (
	DOWNLOAD_BASE_URL = "https://storage.googleapis.com/encore-optional/encore"
)

func downloadBinary(platform, arch string, binary string, log zerolog.Logger) (dockerbuild.HostPath, error) {
	if version.Channel == version.DevBuild {
		suffix := ""
		if platform != runtime.GOOS || arch != runtime.GOARCH {
			suffix = "-" + platform + "-" + arch
		}
		path := filepath.Join(env.EncoreRuntimesPath(), binary+suffix)
		if _, err := os.Stat(path); err == nil {
			return dockerbuild.HostPath(path), nil
		}
		return "", fmt.Errorf("development build of %s/%s %s not found at %s. Build it with `go run ./pkg/encorebuild/cmd/build-local-binary %[3]s --os=%[1]s --arch=%[2]s`", platform, arch, binary, path)
	}
	cacheDir, err := conf.CacheDir()
	if err != nil {
		return "", err
	}
	binDir := dockerbuild.HostPath(cacheDir).Join("bin")
	archDir := binDir.Join(version.Version, platform, arch)
	binaryPath := archDir.Join(binary)
	if _, err := os.Stat(binaryPath.String()); err == nil {
		return binaryPath, nil
	}
	if err := os.MkdirAll(archDir.String(), 0755); err != nil {
		return "", err
	}
	// Download the binaries
	archURL := fmt.Sprintf("%s/%s/%s-%s", DOWNLOAD_BASE_URL, version.Version, platform, arch)
	url := fmt.Sprintf("%s/%s", archURL, binary)
	log.Info().Msgf("Downloading %s/%s %s", platform, arch, binary)
	if err := downloadFile(url, binaryPath.String()); err != nil {
		return "", err
	}
	tryCleanupPreviousVersions(binDir)
	return binaryPath, nil
}

func tryCleanupPreviousVersions(binDir dockerbuild.HostPath) {
	// Clean up binaries for other versions
	entries, err := os.ReadDir(binDir.String())
	if err != nil {
		log.Warn().Msgf("failed to read directory %s: %v", binDir, err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != version.Version {
			oldVersionPath := filepath.Join(binDir.String(), entry.Name())
			if err := os.RemoveAll(oldVersionPath); err != nil {
				log.Warn().Msgf("failed to remove old version directory %s: %v", oldVersionPath, err)
			}
		}
	}
	return
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: %s", url, resp.Status)
	}

	out, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
