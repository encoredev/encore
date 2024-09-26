package export

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/env"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/dockerbuild"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/vcs"
	daemonpb "encr.dev/proto/encore/daemon"
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
		DebugMode:          builder.DebugModeDisabled,
		Environ:            req.Environ,
		GOOS:               req.Goos,
		GOARCH:             req.Goarch,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	bld := builderimpl.Resolve(app.Lang(), expSet)
	defer fns.CloseIgnore(bld)
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
	if err := app.CacheMetadata(parse.Meta); err != nil {
		log.Info().Err(err).Msg("failed to cache metadata")
		return false, errors.Wrap(err, "cache metadata")
	}

	// Validate the service configs.
	_, err = bld.ServiceConfigs(ctx, builder.ServiceConfigsParams{
		Parse: parse,
		CueMeta: &cueutil.Meta{
			// Dummy data to satisfy config validation.
			APIBaseURL: "http://localhost:0",
			EnvName:    "encore-eject",
			EnvType:    cueutil.EnvType_Development,
			CloudType:  cueutil.CloudType_Local,
		},
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
	})

	if err != nil {
		log.Info().Err(err).Msg("compilation failed")
		return false, errors.Wrap(err, "compilation failed")
	}

	var crossNodeRuntime option.Option[dockerbuild.HostPath]
	if app.Lang() == appfile.LangTS && buildInfo.IsCrossBuild() {
		binary, err := downloadBinary(req.Goos, req.Goarch, "encore-runtime.node", log)
		if err != nil {
			return false, errors.Wrap(err, "download runtime binaries")
		}
		crossNodeRuntime = option.Some(binary)
	}

	spec, err := dockerbuild.Describe(dockerbuild.DescribeConfig{
		Meta:              parse.Meta,
		Compile:           result,
		BundleSource:      option.Option[dockerbuild.BundleSourceSpec]{},
		DockerBaseImage:   option.AsOptional(params.BaseImageTag),
		Runtimes:          dockerbuild.HostPath(env.EncoreRuntimesPath()),
		NodeRuntime:       crossNodeRuntime,
		ProcessPerService: app.ProcessPerService(),
	})
	if err != nil {
		return false, errors.Wrap(err, "describe docker image")
	}

	var baseImgOverride option.Option[v1.Image]
	if params.BaseImageTag != "" {
		baseImg, err := resolveBaseImage(ctx, log, params, spec)
		if err != nil {
			return false, errors.Wrap(err, "resolve base image")
		}
		baseImgOverride = option.Some(baseImg)
	}

	var supervisorPath option.Option[dockerbuild.HostPath]
	if spec.Supervisor.Present() {
		binary, err := downloadBinary(req.Goos, req.Goarch, "supervisor-encore", log)
		if err != nil {
			return false, errors.Wrap(err, "download supervisor binaries")
		}
		supervisorPath = option.Some(binary)
	}
	img, err := dockerbuild.BuildImage(ctx, spec, dockerbuild.ImageBuildConfig{
		BuildTime:         time.Now(),
		BaseImageOverride: baseImgOverride,
		AddCACerts:        option.Some[dockerbuild.ImagePath](""),
		SupervisorPath:    supervisorPath,
	})
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

func resolveBaseImage(ctx context.Context, log zerolog.Logger, p *daemonpb.DockerExportParams, spec *dockerbuild.ImageSpec) (v1.Image, error) {
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

	fetchRemote := true
	img, err := daemon.Image(baseImgRef)
	if err == nil {
		file, err := img.ConfigFile()
		if err == nil {
			fetchRemote = file.OS != spec.OS || file.Architecture != spec.Arch
		}
	}
	if fetchRemote {
		log.Info().Msg("could not get image from local daemon, fetching it remotely")
		keychain := authn.DefaultKeychain
		img, err = remote.Image(baseImgRef, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx), remote.WithPlatform(v1.Platform{
			OS:           spec.OS,
			Architecture: spec.Arch,
		}))
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

func pushDockerImage(ctx context.Context, log zerolog.Logger, img v1.Image, destination name.Tag) error {
	log.Info().Msg("pushing docker image to container registry")
	keychain := authn.DefaultKeychain
	if err := remote.Write(destination, img, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx)); err != nil {
		return errors.WithStack(err)
	}
	log.Info().Msg("successfully pushed docker image")
	return nil
}
