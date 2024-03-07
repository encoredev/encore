package githubrelease

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	osPkg "os"
	"os/exec"
	"path/filepath"
	"strings"

	"encr.dev/pkg/encorebuild/buildconf"
	. "encr.dev/pkg/encorebuild/buildutil"
	"github.com/cockroachdb/errors"
)

type Info struct {
	Version  string // The version of the release
	Filename string // The filename of the release (inc extension)
	FileExt  string // The file extension
	URL      string // The URL to download the release from
	Checksum []byte // The checksum of the release
}

// getGithubRelease fetches the latest release from Github for the given org and repo.
func FetchInfo(cfg *buildconf.Config, org string, repo string) *Info {
	rtn := &Info{}

	type GithubRelease struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	// Download the latest releases
	releasesResp := Must(http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", org, repo)))
	defer func() { _ = releasesResp.Body.Close() }()

	if releasesResp.StatusCode != http.StatusOK {
		Bailf("Unexpected response status code: %s", releasesResp.Status)
	}

	releases := &GithubRelease{}
	Check(json.NewDecoder(releasesResp.Body).Decode(releases))

	rtn.Version = releases.TagName

	// Build a list of possible file names
	osOptions := []string{cfg.OS}
	if cfg.OS == "darwin" {
		osOptions = append(osOptions, "macos")
	}
	archOptions := []string{cfg.Arch}
	if cfg.Arch == "amd64" {
		archOptions = append(archOptions, "x86_64", "x86-64")
	} else if cfg.Arch == "arm64" {
		archOptions = append(archOptions, "aarch64")
	}
	extOptions := []string{"tar.gz"} // , "zip"}

	var fileNameOptions []string
	for _, osOption := range osOptions {
		for _, archOption := range archOptions {
			for _, extOption := range extOptions {
				fileNameOptions = append(fileNameOptions,
					fmt.Sprintf("%s_%s.%s", osOption, archOption, extOption),
					fmt.Sprintf("%s-%s.%s", osOption, archOption, extOption),
				)
			}
		}
	}

	// Find the checksum file
	checksumFileURL := ""
	for _, asset := range releases.Assets {
		// We want to know the checksum URL
		if strings.EqualFold(asset.Name, "checksums.txt") {
			checksumFileURL = asset.BrowserDownloadURL

		}

		for _, filenameOption := range fileNameOptions {
			if strings.EqualFold(asset.Name, filenameOption) {
				// We also want to know the asset name and download URL
				rtn.Filename = asset.Name
				rtn.URL = asset.BrowserDownloadURL

				if strings.HasSuffix(asset.Name, ".tar.gz") {
					rtn.FileExt = ".tar.gz"
				} else {
					rtn.FileExt = filepath.Ext(asset.Name)
				}
			}
		}
	}
	if checksumFileURL == "" {
		Bailf("unable to find checksum file in Github release")
	}
	if rtn.URL == "" || rtn.Filename == "" {
		Bailf("unable to find binary in Github release")
	}

	// Download the checksum file
	checksumResp := Must(http.Get(checksumFileURL))
	defer func() { _ = checksumResp.Body.Close() }()
	if checksumResp.StatusCode != http.StatusOK {
		Bailf("Unexpected response status code for checksum file: %s", checksumResp.Status)
	}

	// Read the checksum file line by line
	scanner := bufio.NewScanner(checksumResp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasSuffix(line, rtn.Filename) {
			checksumStr := strings.Split(line, " ")[0]

			checksum := Must(hex.DecodeString(checksumStr))
			rtn.Checksum = checksum

			return rtn
		}
	}
	Bailf("unable to find checksum for asset file in checksum file")
	panic("unreachable")
}

func DownloadLatest(cfg *buildconf.Config, org, repo string) (pathToFile string) {
	// Find the latest release
	release := FetchInfo(cfg, org, repo)

	// Create a cache dir for the download cache for this specific OS and architecture pair
	path := filepath.Join(cfg.CacheDir, "github-releases", org, repo, cfg.OS, cfg.Arch)
	Check(osPkg.MkdirAll(path, 0755))

	downloadFileName := fmt.Sprintf("%s-%s%s", release.Version, hex.EncodeToString(release.Checksum), release.FileExt)
	downloadPath := filepath.Join(path, downloadFileName)

	// Check if the file already exists
	if _, err := osPkg.Stat(downloadPath); err == nil {
		return downloadPath
	} else if !osPkg.IsNotExist(err) {
		Bailf("failed to stat existing download file: %v", err)
	}

	// Now download the file
	downloadResp := Must(http.Get(release.URL))
	defer func() { _ = downloadResp.Body.Close() }()
	if downloadResp.StatusCode != http.StatusOK {
		Bailf("Unexpected response status code for release file: %s", downloadResp.Status)
	}

	// Create the file
	downloadFile := Must(osPkg.Create(downloadPath))
	defer func() {
		_ = downloadFile.Close()
		if r := recover(); r != nil {
			_ = osPkg.Remove(downloadPath) // delete any partially written file
			panic(r)                       // re-panic
		}
	}()
	_ = Must(io.Copy(downloadFile, downloadResp.Body))

	// Now checksum the file
	_ = Must(downloadFile.Seek(0, 0))

	checksum := Must(checksumFile(downloadFile))

	// Check the checksum
	if !bytes.Equal(checksum, release.Checksum) {
		Bailf("checksum of downloaded file (%q) does not match expected checksum (%q)", hex.EncodeToString(checksum), hex.EncodeToString(release.Checksum))
	}

	return downloadPath
}

func checksumFile(file *osPkg.File) ([]byte, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, errors.Wrap(err, "unable to checksum file")
	}
	return hash.Sum(nil), nil
}

func Extract(pathToArchive string, targetDir string) {
	// Create the target dir
	Check(osPkg.MkdirAll(targetDir, 0755))

	// Extract the archive
	cmd := exec.Command("tar", "-xzf", pathToArchive, "--strip-components", "1", "-C", targetDir)
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		Bailf("failed to extract archive: %s", out)
	}
}

func copyDir(src, dst string) error {
	cmd := exec.Command("cp", "-r", src+"/", dst)
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to copy dir: %s", out)
	}
	return nil
}
