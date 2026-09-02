// Package config reads the settings shared by the releaser commands.
//
// The commands run inside GitHub Actions, so everything is passed as
// environment variables (prefixed R_) set by the workflow.
package config

import (
	"context"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/cockroachdb/errors"

	"encr.dev/internal/version"
	"encr.dev/pkg/releaser/bu"
)

// ReleasesBucket is the GCS bucket everything is stored in: the build
// artifacts under the release prefix, the distribution tarballs assembled
// from them, and the encore-go toolchain releases under "encore-go/".
const ReleasesBucket = "encore-releases2"

// Builds of main live under LatestPrefix/<commit sha>, and LatestCommitObject
// holds the sha (one line) of the newest of them whose distribution
// tarballs are complete; cmd/update-latest maintains it.
const (
	LatestPrefix       = "latest"
	LatestCommitObject = LatestPrefix + "/COMMIT"
)

// Common holds the settings every releaser command needs.
type Common struct {
	// Version is the release version being built, e.g. "v1.2.3-nightly.20231231".
	Version string

	// Prefix is the object-name prefix everything for this build lives
	// under. The releaser's proper releases use "encore/<version>"; builds
	// of main use "latest/<commit sha>".
	Prefix bu.RelSlashPath

	// TempDir is the runner's scratch directory (RUNNER_TEMP).
	TempDir bu.FSPath
}

// Load reads the common settings from the environment:
// R_VERSION, R_RELEASE_PREFIX (optional, defaults to "encore/<version>")
// and RUNNER_TEMP.
func Load() (Common, error) {
	ver, err := Required("R_VERSION")
	if err != nil {
		return Common{}, err
	} else if err := ValidateVersion(ver); err != nil {
		return Common{}, err
	}

	prefix := strings.Trim(os.Getenv("R_RELEASE_PREFIX"), "/")
	if prefix == "" {
		prefix = "encore/" + ver
	}

	tempDir, err := Required("RUNNER_TEMP")
	if err != nil {
		return Common{}, err
	}

	return Common{
		Version: ver,
		Prefix:  bu.RelSlashPath(prefix),
		TempDir: bu.FSPath(tempDir),
	}, nil
}

// Required returns the value of the named environment variable,
// or an error if it is unset or empty.
func Required(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", errors.Newf("missing required environment variable %s", name)
	}
	return val, nil
}

// ValidateVersion checks that ver is a release version the releaser knows
// how to build: "v1.2.3", "v1.2.3-beta.1", "v1.2.3-nightly.20231231"
// or "v0.0.0-develop+<commit>".
func ValidateVersion(ver string) error {
	if !strings.HasPrefix(ver, "v") {
		return errors.Newf("version %q must start with 'v'", ver)
	}
	switch version.ChannelFor(ver) {
	case version.GA, version.Beta, version.Nightly, version.DevBuild:
		return nil
	default:
		return errors.Newf("unknown version channel for %q", ver)
	}
}

// NewClient returns a GCS client authenticated with Application Default
// Credentials. In GitHub Actions those come from google-github-actions/auth,
// which exchanges the workflow run's OIDC token for GCP access via Workload
// Identity Federation; no service-account key is involved.
func NewClient(ctx context.Context) (*storage.Client, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "create GCS client")
	}
	return client, nil
}

// ReleasePrefix is the object-name prefix all artifacts for the build
// live under; see Prefix.
func (c Common) ReleasePrefix() bu.RelSlashPath {
	return c.Prefix
}
