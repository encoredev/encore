package dockerbuild

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
)

// DefaultCACertsPath is the default path for where to write CA Certs.
// From https://go.dev/src/crypto/x509/root_linux.go.
const DefaultCACertsPath ImagePath = "/etc/ssl/certs/ca-certificates.crt"

type ImageBuildConfig struct {
	// The time to use when recording times in the image.
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
}

// BuildImage builds a docker image from the given spec.
func BuildImage(ctx context.Context, spec *ImageSpec, cfg ImageBuildConfig) (v1.Image, error) {
	options := []remote.Option{
		remote.WithPlatform(v1.Platform{
			OS:           spec.OS,
			Architecture: spec.Arch,
		}),
	}
	baseImg, err := resolveBaseImage(ctx, spec.DockerBaseImage, cfg.BaseImageOverride, options...)
	if err != nil {
		return nil, errors.Wrap(err, "resolve base image")
	}

	opener, err := buildImageFilesystem(ctx, spec, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "build image fs")
	}

	prioritizedFiles := fns.Map(spec.StargzPrioritizedFiles, func(s ImagePath) string { return string(s) })
	layer, err := tarball.LayerFromOpener(opener,
		tarball.WithEstargz,
		tarball.WithEstargzOptions(
			estargz.WithPrioritizedFiles(prioritizedFiles),
		),
		tarball.WithCompressedCaching,
		tarball.WithCompressionLevel(5), // balance speed and compression
	)

	if err != nil {
		return nil, errors.Wrap(err, "create tarball layer")
	}

	img, err := mutate.Append(baseImg, mutate.Addendum{
		Layer: layer,
		History: v1.History{
			Author:    "encore-app",
			Created:   v1.Time{Time: cfg.BuildTime},
			CreatedBy: "encore.dev",
			Comment:   "Built with encore.dev, the backend development engine",
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "add layer")
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

func buildImageFilesystem(ctx context.Context, spec *ImageSpec, cfg *ImageBuildConfig) (opener tarball.Opener, err error) {
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
	tc := newTarCopier(tw, setFileTimes(cfg.BuildTime))

	// Bundle the source code, if requested.
	if bundle, ok := spec.BundleSource.Get(); ok {
		if err := bundleSource(tc, spec, &bundle); err != nil {
			return nil, err
		}
	}

	// Copy data into the image.
	if err := tc.CopyData(spec); err != nil {
		return nil, err
	}

	// Copy Encore binaries into the image.
	if err := setupSupervisor(tc, spec, cfg); err != nil {
		return nil, err
	}

	// Add build information.
	if err := writeBuildInfo(tc, spec.BuildInfo); err != nil {
		return nil, err
	}

	// Write app meta.
	if err := writeMeta(tc, spec); err != nil {
		return nil, err
	}

	// Add CA certificates, if requested.
	if caCertsDest, ok := cfg.AddCACerts.Get(); ok {
		if caCertsDest == "" {
			caCertsDest = DefaultCACertsPath
		}
		if err := addCACerts(ctx, tw, caCertsDest); err != nil {
			return nil, errors.Wrap(err, "add ca certs")
		}
	}

	if err := tw.Close(); err != nil {
		return nil, errors.Wrap(err, "complete tar")
	}

	opener = func() (io.ReadCloser, error) {
		return os.Open(tarFile.Name())
	}
	return opener, nil
}

func writeMeta(tc *tarCopier, spec *ImageSpec) error {
	if err := tc.WriteFile(defaultMetaPath.String(), 0644, spec.Meta); err != nil {
		return errors.Wrap(err, "write meta")
	}
	return nil
}

func setupSupervisor(tc *tarCopier, spec *ImageSpec, cfg *ImageBuildConfig) error {
	super, ok := spec.Supervisor.Get()
	if !ok {
		return nil
	}

	// Add the supervisor binary.
	{
		hostPath, ok := cfg.SupervisorPath.Get()
		if !ok {
			return errors.New("supervisor requested, but not provided")
		}
		fi, err := os.Stat(string(hostPath))
		if err != nil {
			return errors.Wrap(err, "stat supervisor")
		}

		if err := tc.MkdirAll(super.MountPath.Dir(), 0755); err != nil {
			return errors.Wrap(err, "create supervisor dir")
		}
		if err := tc.CopyFile(super.MountPath, hostPath, fi, ""); err != nil {
			return errors.Wrap(err, "copy supervisor")
		}
	}

	// Write the supervisor configuration.
	{
		data, err := json.MarshalIndent(super.Config, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshal supervisor config")
		}
		if err := tc.WriteFile(string(super.ConfigPath), 0644, data); err != nil {
			return errors.Wrap(err, "write supervisor config")
		}
	}

	return nil
}

func bundleSource(tc *tarCopier, spec *ImageSpec, bundle *BundleSourceSpec) error {
	excludes := make(map[HostPath]bool, len(bundle.ExcludeSource))
	for _, ex := range bundle.ExcludeSource {
		absPath := bundle.Source.Join(string(ex))
		excludes[absPath] = true
	}

	err := tc.CopyDir(&dirCopyDesc{
		Spec:            spec,
		SrcPath:         bundle.Source,
		DstPath:         bundle.Dest,
		ExcludeSrcPaths: excludes,
	})
	return errors.Wrap(err, "bundle source")
}

func writeBuildInfo(tc *tarCopier, spec BuildInfoSpec) error {
	info, err := json.MarshalIndent(spec.Info, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal build info")
	}

	err = tc.WriteFile(string(spec.InfoPath), 0644, info)
	return errors.Wrap(err, "write build info")
}

func addCACerts(ctx context.Context, tw *tar.Writer, dest ImagePath) error {
	const (
		mozillaRootStoreWebsiteTrustBitEnabledURL = "https://ccadb-public.secure.force.com/mozilla/IncludedRootsPEMTxt?TrustBitsInclude=Websites"
	)

	req, err := http.NewRequestWithContext(ctx, "GET", mozillaRootStoreWebsiteTrustBitEnabledURL, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "get root certs")
	}
	defer func() { _ = resp.Body.Close() }()

	// We need to populate the body of the tar file before writing the contents.
	// Use the content length if it was provided. Otherwise, read the whole response
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
		Name:     string(dest),
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
