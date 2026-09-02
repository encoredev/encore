package bu

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cockroachdb/errors"
)

type OS string

const (
	Windows OS = "windows"
	Darwin  OS = "darwin"
	Linux   OS = "linux"
)

type Arch string

const (
	Amd64 Arch = "amd64"
	Arm64 Arch = "arm64"
)

type Platform struct {
	OS   OS   // GOOS format
	Arch Arch // GOARCH format
}

func (p Platform) IsCross() bool {
	return p.OS != OS(runtime.GOOS) || p.Arch != Arch(runtime.GOARCH)
}

func (p Platform) String() string {
	return p.OS.String() + "/" + p.Arch.String()
}

func (os OS) String() string {
	return string(os)
}

func (arch Arch) String() string {
	return string(arch)
}

func ParsePlatform(val string) (Platform, error) {
	goos, arch, ok := strings.Cut(val, "/")
	if !ok {
		return Platform{}, errors.Newf("invalid platform spec: %s", val)
	}
	switch OS(goos) {
	case Windows, Darwin, Linux:
	default:
		return Platform{}, errors.Newf("invalid OS: %s", goos)
	}
	switch Arch(arch) {
	case Amd64, Arm64:
	default:
		return Platform{}, errors.Newf("invalid arch: %s", arch)
	}
	return Platform{OS: OS(goos), Arch: Arch(arch)}, nil
}

// FSPath is a filesystem path.
type FSPath string

func (p FSPath) ToIO() string {
	return string(p)
}

func (p FSPath) Join(elem ...string) FSPath {
	return FSPath(filepath.Join(append([]string{string(p)}, elem...)...))
}

func (p FSPath) Parent() (FSPath, bool) {
	dir := FSPath(filepath.Dir(string(p)))
	if dir == FSPath(filepath.Separator) || dir == "." || dir == p {
		return "", false
	}
	return dir, true
}

func (p FSPath) MustParent() FSPath {
	parent, ok := p.Parent()
	if !ok {
		panic("no parent")
	}
	return parent
}

func (p FSPath) MkdirAll() {
	if err := os.MkdirAll(string(p), 0755); err != nil {
		panic(err)
	}
}

func (p FSPath) ReadFile() ([]byte, error) {
	return os.ReadFile(p.ToIO())
}

// HostInfo holds information about the host.
type HostInfo struct {
	RepoPath FSPath
}

// RelSlashPath is a relative, slash-separated path
type RelSlashPath string

func (p RelSlashPath) Join(elem ...string) RelSlashPath {
	return RelSlashPath(path.Join(append([]string{string(p)}, elem...)...))
}

func (p RelSlashPath) String() string {
	return string(p)
}

// Sha256sum returns the hex-encoded SHA-256 digest of the file.
func Sha256sum(src FSPath) (string, error) {
	f, err := os.Open(src.ToIO())
	if err != nil {
		return "", errors.Wrap(err, "failed to open file")
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", errors.Wrap(err, "failed to hash file")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// WriteSha256sum writes the hex-encoded SHA-256 digest of the file to
// "<src>.sha256" and returns that path.
func WriteSha256sum(src FSPath) (FSPath, error) {
	hash, err := Sha256sum(src)
	if err != nil {
		return "", err
	}
	checksumFile := src + ".sha256"
	if err = os.WriteFile(checksumFile.ToIO(), []byte(hash), 0644); err != nil {
		return "", errors.Wrap(err, "failed to write checksum")
	}
	return checksumFile, nil
}
