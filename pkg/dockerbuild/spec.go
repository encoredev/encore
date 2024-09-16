package dockerbuild

import (
	"fmt"
	pathpkg "path"
	"path/filepath"
	"slices"
	strconv "strconv"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"
	"github.com/rs/xid"

	"encr.dev/pkg/builder"
	"encr.dev/pkg/noopgateway"
	"encr.dev/pkg/noopgwdesc"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/pkg/supervisor"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type ImageSpecFile struct {
	Images []*ImageSpec
}

// ImageSpec is a specification for how to build a docker image.
type ImageSpec struct {
	// The operating system to use for the image.
	OS string

	// The architecture to use for the image.
	Arch string

	// The entrypoint to use for the image. It must be non-empty.
	// The first entry is the executable path, and the rest are the arguments.
	Entrypoint []string

	// Environment variables to set for the entrypoint.
	Env []string

	// The working dir to use for executing the entrypoint.
	WorkingDir ImagePath

	// BuildInfo contains information about the build.
	BuildInfo BuildInfoSpec

	// A map from the builder filesystem paths to the destination path in the image.
	// If the source is a directory, it will be copied recursively.
	CopyData map[ImagePath]HostPath

	// Whether to bundle source into the image.
	// It's handled separately from CopyData since we apply some filtering
	// on what's copied, like excluding .git directories and other build artifacts.
	BundleSource option.Option[BundleSourceSpec]

	// Supervisor specifies the supervisor configuration.
	Supervisor option.Option[SupervisorSpec]

	// The names of services bundled in this image.
	BundledServices []string

	// The names of gateways bundled in this image.
	BundledGateways []string

	// The docker base image to use. If None it defaults to the empty scratch image.
	DockerBaseImage string

	// StargzPrioritizedFiles are file paths in the image that should be prioritized for
	// stargz compression, allowing for faster streaming of those files.
	StargzPrioritizedFiles []ImagePath

	// FeatureFlags specifies feature flags enabled for this image.
	FeatureFlags map[FeatureFlag]bool

	// The app metadata, protobuf encoded.
	Meta []byte
}

type BuildInfoSpec struct {
	// The build info to include in the image.
	Info BuildInfo

	// The path in the image where the build info is written, as a JSON file.
	InfoPath ImagePath
}

type BuildInfo struct {
	// The version of Encore with which the app was compiled.
	// This is string is for informational use only, and its format should not be relied on.
	EncoreCompiler string

	// AppCommit describes the commit of the app.
	AppCommit CommitInfo
}

type CommitInfo struct {
	Revision    string
	Uncommitted bool
}

type BundleSourceSpec struct {
	Source HostPath
	Dest   ImagePath

	// Source paths to exclude from copying, relative to Source.
	ExcludeSource []RelPath
}

type SupervisorSpec struct {
	// Where to mount the supervisor binary in the image.
	MountPath ImagePath

	// Where to write the supervisor configuration in the image.
	ConfigPath ImagePath

	// The config to pass to the supervisor.
	Config *supervisor.Config
}

type DescribeConfig struct {
	// The parsed metadata.
	Meta *meta.Data

	// The compile result.
	Compile *builder.CompileResult

	// The directory containing the runtimes.
	Runtimes HostPath

	// The path to the node runtime, if any.
	NodeRuntime option.Option[HostPath]

	// The docker base image to use, if any. If None it defaults to the empty scratch image.
	DockerBaseImage option.Option[string]

	// BundleSource specifies whether to bundle source into the image,
	// and where the source is located on the host filesystem.
	BundleSource option.Option[BundleSourceSpec]

	// WorkingDir specifies the working directory to start the docker image in.
	WorkingDir option.Option[ImagePath]

	// BuildInfo contains information about the build.
	BuildInfo BuildInfo

	// ProcessPerService specifies whether to run each service in a separate process.
	ProcessPerService bool
}

type (
	// HostPath is a path on the host filesystem.
	HostPath string
	// ImagePath is a path in the docker image.
	ImagePath string
	// RelPath is a relative path.
	RelPath string
)

func (i ImagePath) Dir() ImagePath   { return ImagePath(pathpkg.Dir(string(i))) }
func (i ImagePath) Clean() ImagePath { return ImagePath(pathpkg.Clean(string(i))) }
func (i ImagePath) String() string   { return string(i) }
func (i ImagePath) Join(p ...string) ImagePath {
	return ImagePath(pathpkg.Join(string(i), pathpkg.Join(p...)))
}
func (i ImagePath) JoinImage(p ImagePath) ImagePath {
	return i.Join(string(p))
}
func (h HostPath) Dir() HostPath { return HostPath(filepath.Dir(string(h))) }
func (h HostPath) Join(p ...string) HostPath {
	return HostPath(filepath.Join(string(h), filepath.Join(p...)))
}
func (h HostPath) JoinHost(p HostPath) HostPath {
	return h.Join(string(p))
}
func (h HostPath) ToImage() ImagePath {
	return ImagePath(filepath.ToSlash(string(h)))
}
func (h HostPath) String() string { return string(h) }
func (h HostPath) Rel(target HostPath) (HostPath, error) {
	rel, err := filepath.Rel(string(h), string(target))
	return HostPath(rel), err
}
func (h HostPath) IsAbs() bool {
	return filepath.IsAbs(h.String())
}

// Describe describes the docker image to build.
func Describe(cfg DescribeConfig) (*ImageSpec, error) {
	return newImageSpecBuilder().Describe(cfg)
}

func newImageSpecBuilder() *imageSpecBuilder {
	return &imageSpecBuilder{
		procIDGen: randomProcID,
		spec: &ImageSpec{
			CopyData:        make(map[ImagePath]HostPath),
			FeatureFlags:    make(map[FeatureFlag]bool),
			BundledGateways: []string{},
			BundledServices: []string{},
		},
		seenArtifactDirs: make(map[HostPath]*imageArtifactDir),
		seenPrioFiles:    make(map[ImagePath]bool),
	}
}

type imageArtifactDir struct {
	Base           ImagePath
	BuildArtifacts ImagePath
}

type imageSpecBuilder struct {
	spec *ImageSpec

	// procIDGen generates a random id for each process.
	// Defaults to randomProcID.
	procIDGen func() string

	// The artifact dirs we've already seen, to avoid
	// duplicate copies into the image.
	seenArtifactDirs map[HostPath]*imageArtifactDir
	seenPrioFiles    map[ImagePath]bool
}

const (
	// defaultSupervisorMountPath is the path in the image where the supervisor is mounted.
	defaultSupervisorMountPath ImagePath = "/encore/bin/supervisor"

	// defaultSupervisorConfigPath is the path in the image where the supervisor config is located.
	defaultSupervisorConfigPath ImagePath = "/encore/supervisor.config.json"

	// defaultBuildInfoPath is the path in the image where the build information is located.
	defaultBuildInfoPath ImagePath = "/encore/build-info.json"

	// defaultMetaPath is the path in the image where the application metadata is located.
	defaultMetaPath ImagePath = "/encore/meta"
)

func (b *imageSpecBuilder) Describe(cfg DescribeConfig) (*ImageSpec, error) {
	// Allocate artifact directories for each output.
	for _, out := range cfg.Compile.Outputs {
		b.allocArtifactDir(out)
	}

	// Determine if we should use the supervisor.
	// We must use the supervisor if we have more than one service or gateway.
	useSupervisor := cfg.ProcessPerService || len(cfg.Compile.Outputs) > 1 || len(cfg.Compile.Outputs[0].GetEntrypoints()) > 1

	if !useSupervisor {
		ep := cfg.Compile.Outputs[0].GetEntrypoints()[0]
		out := cfg.Compile.Outputs[0]
		imageArtifacts, ok := b.seenArtifactDirs[HostPath(out.GetArtifactDir())]
		if !ok {
			return nil, errors.Errorf("missing image artifact dir for %q", out.GetArtifactDir())
		}
		cmd := ep.Cmd.Expand(paths.FS(imageArtifacts.BuildArtifacts))
		b.spec.Entrypoint = cmd.Command
		b.spec.Env = cmd.Env
	} else {
		config := &supervisor.Config{
			NoopGateways: make(map[string]*noopgateway.Description),
		}
		super := SupervisorSpec{
			MountPath:  defaultSupervisorMountPath,
			ConfigPath: defaultSupervisorConfigPath,
			Config:     config,
		}

		seenGateways := make(map[string]bool)
		for _, out := range cfg.Compile.Outputs {
			imageArtifacts, ok := b.seenArtifactDirs[HostPath(out.GetArtifactDir())]
			if !ok {
				return nil, errors.Errorf("missing image artifact dir for %q", out.GetArtifactDir())
			}

			for _, ep := range out.GetEntrypoints() {
				cmd := ep.Cmd.Expand(paths.FS(imageArtifacts.BuildArtifacts))
				proc := supervisor.Proc{
					ID:       b.procIDGen(),
					Command:  cmd.Command,
					Env:      cmd.Env,
					Services: slices.Clone(ep.Services),
					Gateways: slices.Clone(ep.Gateways),
				}
				slices.Sort(proc.Services)
				slices.Sort(proc.Gateways)

				for _, gw := range ep.Gateways {
					seenGateways[gw] = true
				}

				config.Procs = append(config.Procs, proc)
			}
		}

		// We need all gateways to be provided by some docker image. But for now, since we only support
		// a single docker image, we need all gateways to be provided by this image.
		// Each gateway that's not hosted by this image should be provided by a noop-gateway.
		if cfg.Meta != nil { // nil check for backwards compatibility
			for _, gw := range cfg.Meta.Gateways {
				if !seenGateways[gw.EncoreName] {
					config.NoopGateways[gw.EncoreName] = noopgwdesc.Describe(cfg.Meta, nil)
				}
			}
		}

		b.addPrio(super.MountPath)
		b.spec.Supervisor = option.Some(super)
		b.spec.Entrypoint = []string{string(super.MountPath), "-c", string(super.ConfigPath)}
		b.spec.Env = nil // not needed by supervisor
	}

	// TS apps use runtime config v2.
	if cfg.Meta.Language == meta.Lang_TYPESCRIPT {
		b.spec.FeatureFlags[NewRuntimeConfig] = true
	}

	// Compute bundled services and gateways.
	{
		for _, out := range cfg.Compile.Outputs {
			for _, ep := range out.GetEntrypoints() {
				b.spec.BundledServices = append(b.spec.BundledServices, ep.Services...)
				b.spec.BundledGateways = append(b.spec.BundledGateways, ep.Gateways...)
			}
		}

		// If we have any noop-gateways, consider them bundled, too.
		if super, ok := b.spec.Supervisor.Get(); ok {
			for name := range super.Config.NoopGateways {
				b.spec.BundledGateways = append(b.spec.BundledGateways, name)
			}
		}

		// Sort and deduplicate.
		slices.Sort(b.spec.BundledServices)
		slices.Compact(b.spec.BundledServices)

		slices.Sort(b.spec.BundledGateways)
		slices.Compact(b.spec.BundledGateways)
	}

	// Add entrypoint files to prioritized files.
	for _, out := range cfg.Compile.Outputs {
		hostArtifacts := HostPath(out.GetArtifactDir())
		imageArtifacts, ok := b.seenArtifactDirs[hostArtifacts]
		if !ok {
			return nil, errors.Errorf("missing image artifact dir for %q", hostArtifacts)
		}

		// If this is a JS build, copy the node modules and package.json to out dir.
		if jsOut, ok := out.(*builder.JSBuildOutput); ok {
			if nodeModules, ok := jsOut.NodeModules.Get(); ok {
				dst := imageArtifacts.Base.Join("node_modules")
				b.spec.CopyData[dst] = HostPath(nodeModules)
			}

			pkgJsonPath := imageArtifacts.Base.Join("package.json")
			b.spec.CopyData[pkgJsonPath] = HostPath(jsOut.PackageJson)
			b.addPrio(pkgJsonPath)
		}

		for _, ep := range out.GetEntrypoints() {
			// For each entrypoint, add prioritized files.
			files := ep.Cmd.PrioritizedFiles.Expand(paths.FS(imageArtifacts.BuildArtifacts))
			for _, file := range files {
				b.addPrio(ImagePath(file))
			}
		}
	}

	// If we have any JS outputs that need the local runtime, copy it into the image.
	{
		for _, out := range cfg.Compile.Outputs {
			if _, ok := out.(*builder.JSBuildOutput); ok {
				// Include the encore.dev package, at the same location.
				runtimeSrc := cfg.Runtimes.Join("js", "encore.dev")
				b.spec.CopyData[runtimeSrc.ToImage()] = runtimeSrc

				// Add the encore-runtime.node file, and set the environment variable to point to it.
				nativeRuntimeHost := cfg.NodeRuntime.GetOrElse(cfg.Runtimes.Join("js", "encore-runtime.node"))
				nativeRuntimeImg := nativeRuntimeHost.ToImage()
				b.spec.CopyData[nativeRuntimeImg] = nativeRuntimeHost
				b.spec.Env = append(b.spec.Env, fmt.Sprintf("ENCORE_RUNTIME_LIB=%s", nativeRuntimeImg))
				b.addPrio(nativeRuntimeImg)
				break
			}
		}
	}

	b.spec.DockerBaseImage = cfg.DockerBaseImage.GetOrElse("scratch")
	b.spec.BundleSource = cfg.BundleSource
	b.spec.WorkingDir = cfg.WorkingDir.GetOrElse("/")
	b.spec.OS = cfg.Compile.OS
	b.spec.Arch = cfg.Compile.Arch

	// Include build information.
	b.spec.BuildInfo = BuildInfoSpec{
		Info:     cfg.BuildInfo,
		InfoPath: defaultBuildInfoPath,
	}

	// Include the app metadata.
	{
		md, err := proto.Marshal(cfg.Meta)
		if err != nil {
			return nil, errors.Wrap(err, "marshal meta")
		}
		b.spec.Meta = md
	}

	return b.spec, nil
}

func (b *imageSpecBuilder) addPrio(path ImagePath) {
	if !b.seenPrioFiles[path] {
		b.seenPrioFiles[path] = true
		b.spec.StargzPrioritizedFiles = append(b.spec.StargzPrioritizedFiles, path)
	}
}

func (b *imageSpecBuilder) allocArtifactDir(out builder.BuildOutput) *imageArtifactDir {
	hostArtifacts := HostPath(out.GetArtifactDir())
	if s := b.seenArtifactDirs[hostArtifacts]; s != nil {
		// Already copied this artifact dir.
		return s
	}

	// This artifact directory has not been copied yet.
	// Determine a reasonable name for it.
	basePath := "/artifacts"

	for i := 0; ; i++ {
		candidatePath := ImagePath(pathpkg.Join(basePath, strconv.Itoa(i)))
		candidate := &imageArtifactDir{
			Base:           candidatePath,
			BuildArtifacts: candidatePath.Join("build"),
		}
		if b.spec.CopyData[candidate.Base] == "" && b.spec.CopyData[candidate.BuildArtifacts] == "" {
			// This name is available.
			b.spec.CopyData[candidate.BuildArtifacts] = hostArtifacts
			b.seenArtifactDirs[hostArtifacts] = candidate
			return candidate
		}

		// This path already exists. Keep trying.
	}
}

func randomProcID() string {
	return fmt.Sprintf("proc_%s", xid.New())
}
