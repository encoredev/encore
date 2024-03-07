package main

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

	"github.com/cockroachdb/errors"
)

type Release struct {
	Version  string // The version of the release
	Filename string // The filename of the release (inc extension)
	FileExt  string // The file extension
	URL      string // The URL to download the release from
	Checksum []byte // The checksum of the release
}

// getGithubRelease fetches the latest release from Github for the given org and repo
func getGithubRelease(org string, repo string, os string, arch string) (*Release, error) {
	rtn := &Release{}

	type GithubRelease struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	// Download the latest releases
	releasesResp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", org, repo))
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch latest release information")
	}
	defer func() { _ = releasesResp.Body.Close() }()

	if releasesResp.StatusCode != http.StatusOK {
		return nil, errors.Newf("Unexpected response status code: %s", releasesResp.Status)
	}

	releases := &GithubRelease{}
	if err := json.NewDecoder(releasesResp.Body).Decode(releases); err != nil {
		return nil, errors.Wrap(err, "unable to decode Github releases")
	}

	rtn.Version = releases.TagName

	// Build a list of possible file names
	osOptions := []string{os}
	if os == "darwin" {
		osOptions = append(osOptions, "macos")
	}
	archOptions := []string{arch}
	if arch == "amd64" {
		archOptions = append(archOptions, "x86_64", "x86-64")
	} else if arch == "arm64" {
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
		return nil, errors.New("unable to find checksum file in Github release")
	}
	if rtn.URL == "" || rtn.Filename == "" {
		return nil, errors.New("unable to find binary in Github release")
	}

	// Download the checksum file
	checksumResp, err := http.Get(checksumFileURL)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch checksum file")
	}
	defer func() { _ = checksumResp.Body.Close() }()
	if checksumResp.StatusCode != http.StatusOK {
		return nil, errors.Newf("Unexpected response status code for checksum file: %s", checksumResp.Status)
	}

	// Read the checksum file line by line
	scanner := bufio.NewScanner(checksumResp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasSuffix(line, rtn.Filename) {
			checksumStr := strings.Split(line, " ")[0]

			checksum, err := hex.DecodeString(checksumStr)
			if err != nil {
				return nil, errors.Wrap(err, "unable to decode checksum")
			}
			rtn.Checksum = checksum

			return rtn, nil
		}
	}
	return nil, errors.New("unable to find checksum for asset file in checksum file")
}

func downloadLatestGithubRelease(org, repo, os, arch string) (pathToFile string, rtnErr error) {
	// Find the latest release
	release, err := getGithubRelease(org, repo, os, arch)
	if err != nil {
		return "", err
	}

	// Create a cache dir for the download cache for this specific OS and architecture pair
	cacheDir, err := osPkg.UserCacheDir()
	if err != nil {
		return "", errors.Wrap(err, "user cache dir")
	}

	path := filepath.Join(cacheDir, "encore-build-cache", "github-releases", org, repo, os, arch)
	err = osPkg.MkdirAll(path, 0755)
	if err != nil {
		return "", errors.Wrap(err, "failed to make cache dir")
	}

	downloadFileName := fmt.Sprintf("%s-%s%s", release.Version, hex.EncodeToString(release.Checksum), release.FileExt)
	downloadPath := filepath.Join(path, downloadFileName)

	// Check if the file already exists
	if _, err := osPkg.Stat(downloadPath); err == nil {
		return downloadPath, nil
	} else if !osPkg.IsNotExist(err) {
		return "", errors.Wrap(err, "failed to stat existing download file")
	}

	// Now download the file
	downloadResp, err := http.Get(release.URL)
	if err != nil {
		return "", errors.Wrap(err, "unable to fetch release file")
	}
	defer func() { _ = downloadResp.Body.Close() }()
	if downloadResp.StatusCode != http.StatusOK {
		return "", errors.Newf("Unexpected response status code for release file: %s", downloadResp.Status)
	}

	// Create the file
	downloadFile, err := osPkg.Create(downloadPath)
	defer func() {
		_ = downloadFile.Close()
		if rtnErr != nil {
			_ = osPkg.Remove(downloadPath) // delete any partially written file
		}
	}()
	_, err = io.Copy(downloadFile, downloadResp.Body)
	if err != nil {
		return "", errors.Wrap(err, "unable to download release file")
	}

	// Now checksum the file
	if _, err := downloadFile.Seek(0, 0); err != nil {
		return "", errors.Wrap(err, "unable to seek to start of release file")
	}

	checksum, err := checksumFile(downloadFile)
	if err != nil {
		return "", errors.Wrap(err, "unable to checksum release file")
	}

	// Check the checksum
	if !bytes.Equal(checksum, release.Checksum) {
		return "", errors.Newf("checksum of downloaded file (%q) does not match expected checksum (%q)", hex.EncodeToString(checksum), hex.EncodeToString(release.Checksum))
	}

	return downloadPath, nil
}

func checksumFile(file *osPkg.File) ([]byte, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, errors.Wrap(err, "unable to checksum file")
	}
	return hash.Sum(nil), nil
}

func extractArchive(pathToArchive string, targetDir string) error {
	// Create the target dir
	if err := osPkg.MkdirAll(targetDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create target dir")
	}

	// Extract the archive
	cmd := exec.Command("tar", "-xzf", pathToArchive, "--strip-components", "1", "-C", targetDir)
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to extract archive: %s", out)
	}

	return nil
}

func TarGzip(srcDirectory string, tarFile string) error {
	// Create the tar.gz file from the src directory
	cmd := exec.Command("tar", "-czf", tarFile, "-C", srcDirectory, ".")
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to create tar.gz: %s", out)
	}
	return nil
}

func copyDir(src, dst string) error {
	cmd := exec.Command("cp", "-r", src+"/", dst)
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to copy dir: %s", out)
	}
	return nil
}
