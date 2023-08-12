package export

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/vcs"
	daemonpb "encr.dev/proto/encore/daemon"
)

const (
	appExePath = "/encore-app"
)

// Docker exports the app as a docker image.
func Docker(ctx context.Context, app *apps.Instance, req *daemonpb.ExportRequest, log zerolog.Logger) (success bool, err error) {
	params := req.GetDocker()
	if params == nil {
		return false, errors.Newf("unsupported format: %T", req.Format)
	}

	expSet, err := app.Experiments(req.Environ)
	if err != nil {
		return false, errors.Wrap(err, "get experimental features")
	}

	vcsRevision := vcs.GetRevision(app.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          []string{"timetzdata"},
		CgoEnabled:         req.CgoEnabled,
		StaticLink:         true,
		Debug:              false,
		GOOS:               req.Goos,
		GOARCH:             req.Goarch,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	bld := builderimpl.Resolve(expSet)
	parse, err := bld.Parse(ctx, builder.ParseParams{
		Build:       buildInfo,
		App:         app,
		Experiments: expSet,
		WorkingDir:  ".",
		ParseTests:  false,
	})
	if err != nil {
		return false, err
	}

	log.Info().Msgf("compiling Encore application for %s/%s", req.Goos, req.Goarch)
	result, err := bld.Compile(ctx, builder.CompileParams{
		Build:       buildInfo,
		App:         app,
		Parse:       parse,
		OpTracker:   nil, // TODO
		Experiments: expSet,
		WorkingDir:  ".",
		CueMeta: &cueutil.Meta{
			// Dummy data to satisfy config validation.
			APIBaseURL: "http://localhost:0",
			EnvName:    "encore-eject",
			EnvType:    cueutil.EnvType_Development,
			CloudType:  cueutil.CloudType_Local,
		},
	})

	if err != nil {
		log.Info().Err(err).Msg("compilation failed")
		return false, errors.Wrap(err, "compilation failed")
	}

	img, err := buildDockerImage(ctx, log, req, result)
	if err != nil {
		return false, errors.Wrap(err, "build docker image")
	}

	if params.LocalDaemonTag != "" {
		tag, err := name.NewTag(params.LocalDaemonTag, name.WeakValidation)
		if err != nil {
			log.Error().Err(err).Msg("invalid image tag")
			return false, nil
		}
		log.Info().Msg("saving image to local docker daemon")

		_, err = daemon.Write(tag, img, daemon.WithUnbufferedOpener())
		if err != nil {
			log.Error().Err(err).Msg("unable to save docker image")
			return false, nil
		}
		log.Info().Msg("successfully saved local docker image")
	}

	if params.PushDestinationTag != "" {
		tag, err := name.NewTag(params.PushDestinationTag, name.WeakValidation)
		if err != nil {
			log.Error().Err(err).Msg("invalid image tag")
			return false, nil
		}
		log.Info().Msg("pushing image to docker registry")
		if err := pushDockerImage(ctx, log, img, tag); err != nil {
			log.Error().Err(err).Msg("unable to push docker image")
			return false, nil
		}
	}

	return true, nil
}

// buildDockerImage builds a docker image.
func buildDockerImage(ctx context.Context, log zerolog.Logger, req *daemonpb.ExportRequest, res *builder.CompileResult) (v1.Image, error) {
	baseImg, err := resolveBaseImage(ctx, log, req.GetDocker())
	if err != nil {
		return nil, errors.Wrap(err, "resolve base image")
	}

	log.Info().Msg("building docker image")
	opener, err := buildImageFilesystem(ctx, res)
	if err != nil {
		return nil, errors.Wrap(err, "build image fs")
	}

	layer, err := tarball.LayerFromOpener(opener,
		tarball.WithEstargz,
		tarball.WithEstargzOptions(
			estargz.WithPrioritizedFiles([]string{appExePath}),
		),
		tarball.WithCompressedCaching,
		tarball.WithCompressionLevel(5), // balance speed and compression
	)
	if err != nil {
		return nil, errors.Wrap(err, "create tarball layer")
	}

	created := v1.Time{Time: time.Now()}

	img, err := mutate.Append(baseImg, mutate.Addendum{
		Layer: layer,
		History: v1.History{
			Author:    "encore-app",
			Created:   created,
			CreatedBy: "encore.dev",
			Comment:   "Built with encore.dev, the backend development engine",
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "add layer")
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "get image config")
	}
	cfg = cfg.DeepCopy()
	cfg.Config.Entrypoint = []string{appExePath}
	cfg.Config.Cmd = nil
	cfg.Author = "encore.dev"
	cfg.Created = created
	cfg.Architecture = req.Goarch
	cfg.OS = req.Goos

	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "add config")
	}

	log.Info().Msg("successfully built docker image")
	return img, nil
}

func resolveBaseImage(ctx context.Context, log zerolog.Logger, p *daemonpb.DockerExportParams) (v1.Image, error) {
	baseImgTag := p.BaseImageTag
	if baseImgTag == "" || baseImgTag == "scratch" {
		return empty.Image, nil
	}

	// Try to get it from the daemon if it exists.
	log.Info().Msgf("resolving base image %s", baseImgTag)
	baseImgRef, err := name.ParseReference(baseImgTag)
	if err != nil {
		return nil, errors.Wrap(err, "parse base image")
	}

	img, err := daemon.Image(baseImgRef)
	if err != nil {
		log.Info().Msg("could not get image from local daemon, fetching it remotely")
		keychain := authn.DefaultKeychain
		img, err = remote.Image(baseImgRef, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx))
		if err != nil {
			return nil, errors.Wrap(err, "unable to fetch image")
		}

		// If the user requested to push the image locally, save the remote image locally as well.
		if p.LocalDaemonTag != "" {
			if tag, err := name.NewTag(baseImgTag, name.WeakValidation); err == nil {
				log.Info().Msgf("saving remote image %s to local docker daemon", baseImgTag)
				if _, err = daemon.Write(tag, img); err != nil {
					log.Warn().Err(err).Msg("unable to save remote image to local docker daemon, skipping")
				} else {
					log.Info().Msgf("saved remote image to local docker daemon")
				}
			}
		}
	}

	return img, nil
}

func buildImageFilesystem(ctx context.Context, res *builder.CompileResult) (opener tarball.Opener, err error) {
	tarFile, err := os.CreateTemp("", "docker-img")
	if err != nil {
		return nil, errors.Wrap(err, "mktemp")
	}
	defer func() {
		if e := tarFile.Close(); e != nil && err == nil {
			err = errors.Wrap(e, "close docker-img file")
		}
	}()

	tw := tar.NewWriter(tarFile)

	{
		appFile, err := os.Open(res.Exe)
		if err != nil {
			return nil, errors.Wrap(err, "open")
		}
		defer func() { _ = appFile.Close() }()

		fi, err := appFile.Stat()
		if err != nil {
			return nil, errors.Wrap(err, "stat")
		}
		err = tw.WriteHeader(&tar.Header{
			Name:     appExePath,
			Typeflag: tar.TypeReg,
			Size:     fi.Size(),
			Mode:     0555,
		})
		if err != nil {
			return nil, errors.Wrap(err, "add file to tar")
		}
		if _, err := io.Copy(tw, appFile); err != nil {
			return nil, errors.Wrap(err, "copy file to tar")
		}
	}

	// Download ca certs
	const certsDest = "/etc/ssl/certs/ca-certificates.crt" // from https://go.dev/src/crypto/x509/root_linux.go
	if err := addCACerts(ctx, tw, certsDest); err != nil {
		return nil, errors.Wrap(err, "add ca certs")
	}

	if err := tw.Close(); err != nil {
		return nil, errors.Wrap(err, "complete tar")
	}

	opener = func() (io.ReadCloser, error) {
		return os.Open(tarFile.Name())
	}
	return opener, nil
}

// addCACerts downloads CA Certs from Mozilla's official source.
func addCACerts(ctx context.Context, tw *tar.Writer, dest string) error {
	const mozillaRootStoreWebsiteTrustBitEnabledURL = "https://ccadb-public.secure.force.com/mozilla/IncludedRootsPEMTxt?TrustBitsInclude=Websites"
	req, err := http.NewRequestWithContext(ctx, "GET", mozillaRootStoreWebsiteTrustBitEnabledURL, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "get root certs")
	}
	defer fns.CloseIgnore(resp.Body)

	// We need to populate the body of the tar file before writing the contents.
	// Use the content length if it was provided. Otherwise read the whole response
	// into memory and use its length.
	var body io.Reader = resp.Body
	size := resp.ContentLength
	if size < 0 {
		// Unknown body; read the whole response into memory
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "read cert data")
		}
		size = int64(len(data))
		body = bytes.NewReader(data)
	}

	// Add the file
	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     dest,
		Size:     size,
	})
	if err != nil {
		return errors.Wrap(err, "create cert file")
	}
	if _, err := io.Copy(tw, body); err != nil {
		return errors.Wrap(err, "write cert data")
	}
	return nil
}

func pushDockerImage(ctx context.Context, log zerolog.Logger, img v1.Image, destination name.Tag) error {
	log.Info().Msg("pushing docker image to container registry")
	keychain := authn.DefaultKeychain
	if err := remote.Write(destination, img, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx)); err != nil {
		return errors.WithStack(err)
	}
	log.Info().Msg("successfully pushed docker image")
	return nil
}
