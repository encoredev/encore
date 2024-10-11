package builder

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	pathspkg "path"
	"runtime"
	"slices"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

var LocalBuildTags = []string{
	"encore_local",
	"encore_no_gcp", "encore_no_aws", "encore_no_azure",
	"encore_no_datadog", "encore_no_prometheus",
}

// DebugMode specifies how to compile the application for debugging.
type DebugMode string

const (
	DebugModeDisabled DebugMode = "disabled"
	DebugModeEnabled  DebugMode = "enabled"
	DebugModeBreak    DebugMode = "break"
)

type BuildInfo struct {
	BuildTags          []string
	CgoEnabled         bool
	StaticLink         bool
	DebugMode          DebugMode
	Environ            []string
	GOOS, GOARCH       string
	KeepOutput         bool
	Revision           string
	UncommittedChanges bool

	// MainPkg is the path to the existing main package to use, if any.
	MainPkg option.Option[paths.Pkg]

	// Overrides to explicitly set the GoRoot and EncoreRuntime paths.
	// if not set, they will be inferred from the current executable.
	GoRoot         option.Option[paths.FS]
	EncoreRuntimes option.Option[paths.FS]

	// UseLocalJSRuntime specifies whether to override the installed Encore version
	// with the local JS runtime.
	UseLocalJSRuntime bool

	// Logger allows a custom logger to be used by the various phases of the builder.
	Logger option.Option[zerolog.Logger]
}

func (b *BuildInfo) IsCrossBuild() bool {
	return b.GOOS != runtime.GOOS || b.GOARCH != runtime.GOARCH
}

// DefaultBuildInfo returns a BuildInfo with default values.
// It can be modified afterwards.
func DefaultBuildInfo() BuildInfo {
	return BuildInfo{
		BuildTags:          slices.Clone(LocalBuildTags),
		CgoEnabled:         true,
		StaticLink:         false,
		DebugMode:          DebugModeDisabled,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         false,
		Revision:           "",
		UncommittedChanges: false,
	}
}

type ParseParams struct {
	Build       BuildInfo
	App         *apps.Instance
	Experiments *experiments.Set
	WorkingDir  string
	ParseTests  bool
}

type ParseResult struct {
	Meta *meta.Data
	Data any
}

type CompileParams struct {
	Build       BuildInfo
	App         *apps.Instance
	Parse       *ParseResult
	OpTracker   *optracker.OpTracker
	Experiments *experiments.Set
	WorkingDir  string

	// Override to explicitly allow the Encore version to be set.
	EncoreVersion option.Option[string]
}

type ArtifactString string

func (a ArtifactString) Join(strs ...string) ArtifactString {
	str := pathspkg.Join(strs...)
	return ArtifactString(pathspkg.Join(string(a), str))
}

func (a ArtifactString) Expand(artifactDir paths.FS) string {
	return os.Expand(string(a), func(key string) string {
		if key == "ARTIFACT_DIR" {
			return artifactDir.ToIO()
		}
		return ""
	})
}

type ArtifactStrings []ArtifactString

func (a ArtifactStrings) Expand(artifactDir paths.FS) []string {
	return fns.Map(a, func(a ArtifactString) string { return a.Expand(artifactDir) })
}

// CmdSpec is a specification for a command to run.
//
// The fields can refer to file paths within the artifact directory
// using the "${ARTIFACT_DIR}" placeholder (substituted with os.ExpandEnv).
// This is necessary when building docker images, as otherwise the file paths
// will refer to the wrong filesystem location in the built docker image.
type CmdSpec struct {
	// The command to execute. Can either be a filesystem path
	// or a path to a binary (using "${ARTIFACT_DIR}" as a placeholder).
	Command ArtifactStrings `json:"command"`

	// Additional env variables to pass in.
	Env ArtifactStrings `json:"env"`

	// PrioritizedFiles are file paths that should be prioritized when
	// building a streamable docker image.
	PrioritizedFiles ArtifactStrings `json:"prioritized_files"`
}

func (s *CmdSpec) Expand(artifactDir paths.FS) Cmd {
	return Cmd{
		Command: s.Command.Expand(artifactDir),
		Env:     s.Env.Expand(artifactDir),
	}
}

// Cmd defines a command to run. It's like CmdSpec, but uses expanded paths
// instead of ArtifactStrings. A CmdSpec can be turned into a Cmd using Expand.
type Cmd struct {
	// The command to execute, with arguments.
	Command []string

	// Additional env variables to pass in.
	Env []string
}

type CompileResult struct {
	OS      string
	Arch    string
	Outputs []BuildOutput
}

type BuildOutput interface {
	GetArtifactDir() paths.FS
	GetEntrypoints() []Entrypoint
}

type Entrypoint struct {
	// How to run this entrypoint.
	Cmd CmdSpec `json:"cmd"`
	// Services hosted by this entrypoint.
	Services []string `json:"services"`
	// Gateways hosted by this entrypoint.
	Gateways []string `json:"gateways"`
	// Whether this entrypoint uses the new runtime config.
	UseRuntimeConfigV2 bool `json:"use_runtime_config_v2"`
}

type GoBuildOutput struct {
	// The folder containing the build artifacts.
	// These artifacts are assumed to be relocatable.
	ArtifactDir paths.FS `json:"artifact_dir"`

	// The entrypoints that are part of this build output.
	Entrypoints []Entrypoint `json:"entrypoints"`
}

func (o *GoBuildOutput) GetArtifactDir() paths.FS     { return o.ArtifactDir }
func (o *GoBuildOutput) GetEntrypoints() []Entrypoint { return o.Entrypoints }

type JSBuildOutput struct {
	// NodeModules are the node modules that the build artifacts rely on.
	// It's None if the artifacts don't rely on any node modules.
	NodeModules option.Option[paths.FS] `json:"node_modules"`

	// The folder containing the build artifacts.
	// These artifacts are assumed to be relocatable.
	ArtifactDir paths.FS `json:"artifact_dir"`

	// PackageJson is the path to the package.json file to use.
	PackageJson paths.FS `json:"package_json"`

	// The entrypoints that are part of this build output.
	Entrypoints []Entrypoint `json:"entrypoints"`

	// Whether the build output uses the local runtime on the builder,
	// as opposed to installing a published release via e.g. 'npm install'.
	UsesLocalRuntime bool `json:"uses_local_runtime"`
}

func (o *JSBuildOutput) GetArtifactDir() paths.FS     { return o.ArtifactDir }
func (o *JSBuildOutput) GetEntrypoints() []Entrypoint { return o.Entrypoints }

type RunTestsParams struct {
	Spec *TestSpecResult

	// WorkingDir is the directory to invoke the test command from.
	WorkingDir paths.FS

	// Stdout and Stderr are where to redirect the command output.
	Stdout, Stderr io.Writer
}

type TestSpecParams struct {
	Compile CompileParams

	// Env sets environment variables for "go test".
	Env []string

	// Args sets extra arguments for "go test".
	Args []string
}

// ErrNoTests is reported by TestSpec when there aren't any tests to run.
var ErrNoTests = errors.New("no tests found")

type TestSpecResult struct {
	Command string
	Args    []string
	Environ []string

	// For use by the builder when invoking RunTests.
	BuilderData any
}

type GenUserFacingParams struct {
	Build BuildInfo
	App   *apps.Instance
	Parse *ParseResult
}

type ServiceConfigsParams struct {
	Parse   *ParseResult
	CueMeta *cueutil.Meta
}

type ServiceConfigsResult struct {
	Configs     map[string]string
	ConfigFiles fs.FS
}

type Impl interface {
	Parse(context.Context, ParseParams) (*ParseResult, error)
	Compile(context.Context, CompileParams) (*CompileResult, error)
	TestSpec(context.Context, TestSpecParams) (*TestSpecResult, error)
	RunTests(context.Context, RunTestsParams) error
	ServiceConfigs(context.Context, ServiceConfigsParams) (*ServiceConfigsResult, error)
	GenUserFacing(context.Context, GenUserFacingParams) error
	UseNewRuntimeConfig() bool
	NeedsMeta() bool
	Close() error
}
