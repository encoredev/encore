package dockerbuild

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rs/zerolog/log"

	"encr.dev/pkg/option"
)

// DefaultCACertsPath is the default path for where to write CA Certs.
// From https://go.dev/src/crypto/x509/root_linux.go.
const DefaultCACertsPath ImagePath = "/etc/ssl/certs/ca-certificates.crt"

// layerEpoch is the fixed timestamp used for all files in image layers.
// Using a fixed time (rather than the build time) makes layer contents
// reproducible, so unchanged layers keep identical digests across builds.
var layerEpoch = time.Unix(0, 0)

type ImageBuildConfig struct {
	// The time to use when recording times in the image configuration,
	// such as the image creation time and history entries.
	// File timestamps in the image layers always use a fixed epoch
	// so that layer digests are reproducible across builds.
	BuildTime time.Time

	// AddCACerts, if set, specifies where in the image to mount the CA certificates.
	// If set to Some(""), defaults to DefaultCACertsPath.
	AddCACerts option.Option[ImagePath]

	// SupervisorPath is the path to the supervisor binary to use.
	// It must be set if the image includes the supervisor.
	SupervisorPath option.Option[HostPath]

	// BaseImageOverride overrides the base image to use.
	// If None it resolves the image from the spec using ResolveRemoteImage.
	BaseImageOverride option.Option[v1.Image]

	// A URL to a http proxy used to fetch images
	DockerOptions []remote.Option
}

// BuildImage builds a docker image from the given spec.
func BuildImage(ctx context.Context, spec *ImageSpec, cfg ImageBuildConfig) (v1.Image, error) {
	options := append(cfg.DockerOptions,
		remote.WithPlatform(v1.Platform{
			OS:           spec.OS,
			Architecture: spec.Arch,
		}),
	)
	baseImg, err := resolveBaseImage(ctx, spec.DockerBaseImage, cfg.BaseImageOverride, options...)
	if err != nil {
		return nil, errors.Wrap(err, "resolve base image")
	}

	layers, err := buildImageFilesystem(ctx, spec, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "build image fs")
	}

	adds := make([]mutate.Addendum, 0, len(layers))
	for _, l := range layers {
		layer, err := tarball.LayerFromOpener(l.opener,
			tarball.WithCompressionLevel(5), // balance speed and compression
		)
		if err != nil {
			return nil, errors.Wrapf(err, "create %s layer", l.kind)
		}
		adds = append(adds, mutate.Addendum{
			Layer: layer,
			History: v1.History{
				Author:    "encore-app",
				Created:   v1.Time{Time: cfg.BuildTime},
				CreatedBy: "encore.dev",
				Comment:   fmt.Sprintf("%s layer, built with encore.dev", l.kind),
			},
		})
	}

	log.Info().Int("layers", len(adds)).Msg("adding layers to base image")
	img, err := mutate.Append(baseImg, adds...)
	if err != nil {
		return nil, errors.Wrap(err, "add layers")
	}

	// Copy the base image's environment variables.
	baseCfg, err := baseImg.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "get base image config")
	}
	envs := newEnvMap(baseCfg.Config.Env)

	imgCfg, err := img.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "get image config")
	}
	imgCfg = imgCfg.DeepCopy()

	// Add the image spec's environment.
	envs.Update(spec.Env)

	imgCfg.Config.Entrypoint = spec.Entrypoint
	imgCfg.Config.Cmd = nil
	imgCfg.Config.Env = envs.ToSlice()
	imgCfg.Config.WorkingDir = string(spec.WorkingDir)
	imgCfg.Author = "encore.dev"
	imgCfg.Created = v1.Time{Time: cfg.BuildTime}
	imgCfg.Architecture = spec.Arch
	imgCfg.OS = spec.OS

	img, err = mutate.ConfigFile(img, imgCfg)
	if err != nil {
		return nil, errors.Wrap(err, "add config")
	}
	return img, nil
}

// ResolveRemoteImage resolves the base image with the given reference.
// If imageRef is the empty string or "scratch" it resolves to the empty image.
func ResolveRemoteImage(ctx context.Context, imageRef string, options ...remote.Option) (v1.Image, error) {
	if imageRef == "" || imageRef == "scratch" {
		return empty.Image, nil
	}

	// Try to get it from the daemon if it exists.
	baseImgRef, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "parse image ref")
	}

	img, err := remote.Image(baseImgRef, append(options, remote.WithContext(ctx))...)
	if err != nil {
		return nil, errors.Wrap(err, "fetch image")
	}
	return img, nil
}

func resolveBaseImage(ctx context.Context, baseImgTag string, overrideBaseImage option.Option[v1.Image], options ...remote.Option) (v1.Image, error) {
	if override, ok := overrideBaseImage.Get(); ok {
		return override, nil
	}
	return ResolveRemoteImage(ctx, baseImgTag, options...)
}

func buildImageFilesystem(ctx context.Context, spec *ImageSpec, cfg *ImageBuildConfig) ([]imageLayer, error) {
	lc := newLayeredTarCopier(setFileTimes(layerEpoch))

	// Bundle the source code, if requested.
	if bundle, ok := spec.BundleSource.Get(); ok {
		if err := bundleSource(lc, spec, &bundle); err != nil {
			return nil, err
		}
	}

	// Copy data into the image.
	if err := lc.CopyData(spec); err != nil {
		return nil, err
	}

	// Copy Encore binaries into the image.
	if err := setupSupervisor(lc, spec, cfg); err != nil {
		return nil, err
	}

	// Add build information.
	if err := writeBuildInfo(lc.Layer(configLayer), spec.BuildInfo); err != nil {
		return nil, err
	}

	// Write additional files
	if err := writeExtraFiles(lc.Layer(configLayer), spec.WriteFiles); err != nil {
		return nil, err
	}

	// Add CA certificates, if requested.
	if caCertsDest, ok := cfg.AddCACerts.Get(); ok {
		if caCertsDest == "" {
			caCertsDest = DefaultCACertsPath
		}
		if err := addCACerts(ctx, lc.Layer(certsLayer), caCertsDest); err != nil {
			return nil, errors.Wrap(err, "add ca certs")
		}
	}

	return lc.Layers(), nil
}

func writeExtraFiles(tc *tarCopier, files map[ImagePath][]byte) error {
	// Sort the paths so the layer contents are deterministic.
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if err := tc.WriteFile(path, 0644, files[path]); err != nil {
			return errors.Wrap(err, "write image data")
		}
	}
	return nil
}

func setupSupervisor(lc *layeredTarCopier, spec *ImageSpec, cfg *ImageBuildConfig) error {
	super, ok := spec.Supervisor.Get()
	if !ok {
		return nil
	}

	// Add the supervisor binary to the runtime layer.
	{
		hostPath, ok := cfg.SupervisorPath.Get()
		if !ok {
			return errors.New("supervisor requested, but not provided")
		}
		fi, err := os.Stat(string(hostPath))
		if err != nil {
			return errors.Wrap(err, "stat supervisor")
		}

		tc := lc.Layer(runtimeLayer)
		if err := tc.MkdirAll(super.MountPath.Dir(), 0755); err != nil {
			return errors.Wrap(err, "create supervisor dir")
		}
		if err := tc.CopyFile(super.MountPath, hostPath, fi, ""); err != nil {
			return errors.Wrap(err, "copy supervisor")
		}
	}

	// Write the supervisor configuration to the config layer.
	{
		data, err := json.MarshalIndent(super.Config, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshal supervisor config")
		}
		if err := lc.Layer(configLayer).WriteFile(super.ConfigPath, 0644, data); err != nil {
			return errors.Wrap(err, "write supervisor config")
		}
	}

	return nil
}

func bundleSource(lc *layeredTarCopier, spec *ImageSpec, bundle *BundleSourceSpec) error {
	includes := []HostPath{bundle.Source.Join(filepath.FromSlash(string(bundle.AppRootRelpath)))}
	for _, ex := range bundle.IncludeSource {
		includes = append(includes, bundle.Source.Join(string(ex)))
	}

	excludes := make(map[HostPath]bool, len(bundle.ExcludeSource))
	for _, ex := range bundle.ExcludeSource {
		absPath := bundle.Source.Join(string(ex))
		excludes[absPath] = true
	}

	err := lc.Layer(appLayer).CopyDir(&dirCopyDesc{
		Spec:            spec,
		SrcPath:         bundle.Source,
		DstPath:         bundle.Dest,
		ExcludeSrcPaths: excludes,
		IncludeSrcPaths: includes,
		DepsCopier:      lc.Layer(depsLayer),
	})
	return errors.Wrap(err, "bundle source")
}

func writeBuildInfo(tc *tarCopier, spec BuildInfoSpec) error {
	info, err := json.MarshalIndent(spec.Info, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal build info")
	}

	err = tc.WriteFile(spec.InfoPath, 0644, info)
	return errors.Wrap(err, "write build info")
}

func tryFetch(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "get root certs")
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, errors.Newf("root cert url returned status code: %s", resp.Status)
	}
	return resp, nil
}

func addCACerts(ctx context.Context, tc *tarCopier, dest ImagePath) error {
	const (
		encoreCachedRootCerts = "https://api.encore.dev/artifacts/build/root-certs"
		curlCACertStore       = "https://curl.se/ca/cacert.pem"
	)
	var (
		resp *http.Response
		err  error
	)
	for _, url := range []string{encoreCachedRootCerts, curlCACertStore} {
		resp, err = tryFetch(ctx, url)
		if err == nil {
			break
		}
		log.Warn().Err(err).Msgf("failed to fetch root certs from: %s", url)
	}
	if err != nil {
		return errors.Wrap(err, "failed to fetch cert file")
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read cert data")
	}

	// Add the file
	err = tc.WriteFile(dest, 0644, data)
	return err
}

type envMap map[string]string

func (m envMap) Update(envs []string) {
	for _, e := range envs {
		key, value, _ := strings.Cut(e, "=")
		m[key] = value
	}
}

func (m envMap) ToSlice() []string {
	envs := make([]string, 0, len(m))
	for k, v := range m {
		envs = append(envs, k+"="+v)
	}
	slices.Sort(envs)
	return envs
}

func newEnvMap(envs []string) envMap {
	m := make(envMap, len(envs))
	for _, e := range envs {
		key, value, _ := strings.Cut(e, "=")
		m[key] = value
	}
	return m
}
