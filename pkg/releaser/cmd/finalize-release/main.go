// Command finalize-release assembles the per-platform distribution tarballs
// from the artifacts the build command uploaded to encore-releases2, and
// stores them alongside:
//
//	<prefix>/encore-<os>_<arch>.tar.gz
//	<prefix>/encore-<os>_<arch>.tar.gz.sha256
//
// Each tarball contains bin/ (the CLI and its helper binaries), runtimes/go,
// runtimes/js (encore-runtime.node and the encore.dev package) and
// encore-go (the patched Go toolchain, taken from encore-go/ in the same
// bucket).
//
// Configuration (environment variables):
//
//	R_VERSION            release version, e.g. "v1.2.3-nightly.20231231"
//	R_RELEASE_PREFIX     object prefix in the bucket; defaults to "encore/<version>"
//	R_ENCORE_GO_VERSION  optional encore-go version to bundle ("1.25.4", as
//	                     under encore-go/ in the bucket); defaults to the
//	                     version named by encore-go/latest
//	RUNNER_TEMP          scratch directory
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"

	"encr.dev/internal/version"
	. "encr.dev/pkg/releaser/bu"
	"encr.dev/pkg/releaser/config"
	"encr.dev/pkg/releaser/steps/gcsupload"
)

var cfg struct {
	config.Common
	EncoreGoVersion string
}

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	common, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse environment variables")
	}
	cfg.Common = common
	cfg.EncoreGoVersion = os.Getenv("R_ENCORE_GO_VERSION")

	log.Info().Msg("starting release")

	if err := finalizeRelease(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("failed to finalize release")
	}
}

func finalizeRelease(ctx context.Context) error {
	client, err := config.NewClient(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to instantiate GCS client")
	}
	bucket := client.Bucket(config.ReleasesBucket)

	targets := []Platform{
		{OS: Linux, Arch: Amd64},
		{OS: Linux, Arch: Arm64},
		{OS: Darwin, Arch: Amd64},
		{OS: Darwin, Arch: Arm64},
		{OS: Windows, Arch: Amd64},
	}

	g, gCtx := errgroup.WithContext(ctx)
	for _, target := range targets {
		base := cfg.TempDir.Join(target.OS.String() + "-" + target.Arch.String())
		workDir := base.Join("work")
		outDir := base.Join("out")
		workDir.MkdirAll()
		outDir.MkdirAll()
		r := &releaser{
			TmpDir:  workDir,
			OutDir:  outDir,
			Target:  target,
			Bucket:  bucket,
			Version: cfg.Version,
		}
		g.Go(func() error {
			err := r.Run(gCtx)
			return errors.Wrapf(err, "unable to release %s-%s", target.OS, target.Arch)
		})
	}

	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "unable to release all platforms")
	}
	return nil
}

type releaser struct {
	TmpDir  FSPath
	OutDir  FSPath
	Target  Platform
	Bucket  *storage.BucketHandle
	Version string
}

func (r *releaser) Run(ctx context.Context) error {
	// Extract everything.
	{
		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			err := r.extractBin(ctx)
			return errors.Wrap(err, "unable to extract binaries")
		})
		g.Go(func() error {
			err := r.extractEncoreGoRuntime(ctx)
			return errors.Wrap(err, "unable to extract encore-go-runtime")
		})
		g.Go(func() error {
			err := r.extractEncoreJSRuntime(ctx)
			return errors.Wrap(err, "unable to extract encore-js-runtime")
		})
		g.Go(func() error {
			err := r.extractGo(ctx)
			return errors.Wrap(err, "unable to extract encore-go")
		})

		if err := g.Wait(); err != nil {
			return errors.Wrap(err, "unable to extract artifacts")
		}
	}

	// Upload the final artifact.
	if dst, err := r.createFinalArtifact(ctx); err != nil {
		return errors.Wrap(err, "unable to create final artifact")
	} else if err := r.uploadFinalArtifact(ctx, dst); err != nil {
		return errors.Wrap(err, "unable to upload final artifact")
	}

	return nil
}

// objectPath returns the object name for an artifact of this release.
func (r *releaser) objectPath(elems ...string) string {
	return cfg.ReleasePrefix().Join(elems...).String()
}

func (r *releaser) extractBin(ctx context.Context) error {
	prefix := r.objectPath(fmt.Sprintf("%s-%s", r.Target.OS, r.Target.Arch), "bin") + "/"
	iter := r.Bucket.Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	found := 0
	for {
		attrs, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		} else if err != nil {
			return errors.Wrap(err, "unable to list objects")
		}
		found++

		suffix := strings.TrimPrefix(attrs.Name, prefix)
		dst := r.OutDir.Join("bin", suffix)

		// Is this the encore binary? If so rewrite its name for beta/nightly releases.
		if filepath.Base(dst.ToIO()) == "encore" {
			switch version.ChannelFor(r.Version) {
			case version.Beta:
				dst += "-beta"
			case version.Nightly:
				dst += "-nightly"
			}
		}

		// Suffix all windows binaries with .exe.
		if r.Target.OS == Windows && !strings.HasSuffix(dst.ToIO(), ".exe") {
			dst += ".exe"
		}

		obj := r.Bucket.Object(attrs.Name)
		g.Go(func() error {
			err := slurpToFile(ctx, obj, dst)
			if err != nil {
				return errors.Wrapf(err, "unable to download %s", attrs.Name)
			}

			// Mark the file as executable.
			if err := os.Chmod(dst.ToIO(), 0755); err != nil {
				return errors.Wrapf(err, "unable to mark %s as executable", dst)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "unable to download binaries")
	} else if found == 0 {
		return errors.Newf("no binaries found under gs://%s/%s", config.ReleasesBucket, prefix)
	}
	return nil
}

func (r *releaser) extractEncoreGoRuntime(ctx context.Context) error {
	obj := r.Bucket.Object(r.objectPath("encore-go-runtime.tar.gz"))

	tmpFile := r.TmpDir.Join("encore-go-runtime.tar.gz")
	if err := slurpToFile(ctx, obj, tmpFile); err != nil {
		return errors.Wrap(err, "unable to download encore-go-runtime")
	}

	// Extract the tarball
	dst := r.OutDir.Join("runtimes", "go")
	dst.MkdirAll()
	cmd := exec.CommandContext(ctx, "tar", "-xzf", tmpFile.ToIO(), "-C", dst.ToIO())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "unable to extract encore-go-runtime")
	}
	return nil
}

func (r *releaser) extractEncoreJSRuntime(ctx context.Context) error {
	jsruntimeDst := r.OutDir.Join("runtimes", "js")
	jsruntimeDst.MkdirAll()

	// Extract the encore-runtime.node native module.
	{
		obj := r.Bucket.Object(r.objectPath(fmt.Sprintf("%s-%s", r.Target.OS, r.Target.Arch), "encore-runtime.node"))

		dst := jsruntimeDst.Join("encore-runtime.node")
		if err := slurpToFile(ctx, obj, dst); err != nil {
			return errors.Wrap(err, "unable to download encore-runtime.node")
		}
	}
	// Extract the encore.dev package.
	{
		obj := r.Bucket.Object(r.objectPath("npmpkg-encore-dev.tar.gz"))

		tmpFile := r.TmpDir.Join("encore-js-runtime.tar.gz")
		if err := slurpToFile(ctx, obj, tmpFile); err != nil {
			return errors.Wrap(err, "unable to download encore-js-runtime")
		}

		// Extract the tarball
		dst := jsruntimeDst.Join("encore.dev")
		dst.MkdirAll()
		cmd := exec.CommandContext(ctx, "tar",
			"-xzf", tmpFile.ToIO(),
			"--strip-components=1", // strip the "package" dir that 'npm pack' adds.
			"-C", dst.ToIO())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return errors.Wrap(err, "unable to extract encore-js-runtime")
		}
	}

	return nil
}

// extractGo bundles the patched Go toolchain from encore-go/ in the bucket.
func (r *releaser) extractGo(ctx context.Context) error {
	// If we don't have a Go version, grab the latest release.
	version := cfg.EncoreGoVersion
	if version == "" {
		var err error
		version, err = readLatestEncoreGoRelease(ctx, r.Bucket)
		if err != nil {
			return errors.Wrap(err, "unable to read latest encore-go release")
		}
	}

	objPath := fmt.Sprintf("encore-go/%s/%s-%s.tar.gz", version, r.Target.OS, r.Target.Arch)
	obj := r.Bucket.Object(objPath)

	tmpFile := r.TmpDir.Join("go.tar.gz")
	if err := slurpToFile(ctx, obj, tmpFile); err != nil {
		return errors.Wrap(err, "unable to download go")
	}

	// Extract the tarball
	dst := r.OutDir.Join("encore-go")
	dst.MkdirAll()
	cmd := exec.CommandContext(ctx, "tar", "-xzf", tmpFile.ToIO(), "-C", dst.ToIO())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "unable to extract go")
	}
	log.Info().Msgf("bundled encore-go %s for %s", version, r.Target)
	return nil
}

// readLatestEncoreGoRelease returns the encore-go version named by the
// encore-go/latest object, maintained by the encore-go release process.
func readLatestEncoreGoRelease(ctx context.Context, bucket *storage.BucketHandle) (string, error) {
	reader, err := bucket.Object("encore-go/latest").NewReader(ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to open encore-go/latest")
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		return "", errors.Wrap(err, "unable to read encore-go/latest")
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", errors.New("encore-go/latest is empty")
	}
	return version, nil
}

func (r *releaser) createFinalArtifact(ctx context.Context) (FSPath, error) {
	dst := r.TmpDir.Join("encore-final.tar.gz")
	cmd := exec.CommandContext(ctx, "tar", "-czf", dst.ToIO(), "-C", r.OutDir.ToIO(), ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "unable to create tarball")
	}
	return dst, nil
}

// uploadFinalArtifact uploads the tarball and a sidecar file with its
// hex-encoded SHA-256 digest.
func (r *releaser) uploadFinalArtifact(ctx context.Context, artifact FSPath) error {
	hash, err := hashFile(artifact)
	if err != nil {
		return errors.Wrap(err, "unable to hash tarball")
	}
	checksumFile := artifact + ".sha256"
	if err := os.WriteFile(checksumFile.ToIO(), []byte(hex.EncodeToString(hash)), 0644); err != nil {
		return errors.Wrap(err, "unable to write checksum")
	}

	name := fmt.Sprintf("encore-%s_%s.tar.gz", r.Target.OS, r.Target.Arch)
	return gcsupload.Upload(ctx, gcsupload.UploadInput{
		Bucket: r.Bucket,
		Entries: gcsupload.Entries{gcsupload.Dir{
			Name: cfg.ReleasePrefix().String(),
			Entries: gcsupload.Entries{
				gcsupload.File{Name: name, Source: artifact},
				gcsupload.File{Name: name + ".sha256", Source: checksumFile},
			},
		}},
	})
}

func slurpToFile(ctx context.Context, obj *storage.ObjectHandle, dst FSPath) error {
	// We've observed that this fails sometimes if the object has just been uploaded.
	// Try a few times in that case.
	var reader *storage.Reader
	var err error
	for i := 0; i < 5; i++ {
		reader, err = obj.NewReader(ctx)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return errors.Wrapf(err, "unable to open object %s", obj.ObjectName())
	}
	defer reader.Close()

	dst.MustParent().MkdirAll()
	f, err := os.Create(dst.ToIO())
	if err != nil {
		return errors.Wrap(err, "unable to create file")
	}
	_, err = io.Copy(f, reader)
	if err2 := f.Close(); err == nil {
		err = err2
	}
	return errors.Wrap(err, "unable to copy object to file")
}

func hashFile(name FSPath) ([]byte, error) {
	hasher := sha256.New()
	f, err := os.Open(name.ToIO())
	if err != nil {
		return nil, errors.Wrap(err, "unable to open file")
	}

	_, err = io.Copy(hasher, f)
	if err2 := f.Close(); err == nil {
		err = err2
	}
	return hasher.Sum(nil), errors.Wrap(err, "unable to hash file")
}
