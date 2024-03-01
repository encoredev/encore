package builder

import (
	"context"
	"io"
	"io/fs"
	"runtime"
	"slices"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

var LocalBuildTags = []string{
	"encore_local",
	"encore_no_gcp", "encore_no_aws", "encore_no_azure",
	"encore_no_datadog", "encore_no_prometheus",
}

type BuildInfo struct {
	BuildTags          []string
	CgoEnabled         bool
	StaticLink         bool
	Debug              bool
	GOOS, GOARCH       string
	KeepOutput         bool
	Revision           string
	UncommittedChanges bool

	// MainPkg is the path to the existing main package to use, if any.
	MainPkg option.Option[paths.Pkg]

	// Overrides to explicitly set the GoRoot and EncoreRuntime paths.
	// if not set, they will be inferred from the current executable.
	GoRoot        option.Option[paths.FS]
	EncoreRuntime option.Option[paths.FS]

	// Logger allows a custom logger to be used by the various phases of the builder.
	Logger option.Option[zerolog.Logger]
}

// DefaultBuildInfo returns a BuildInfo with default values.
// It can be modified afterwards.
func DefaultBuildInfo() BuildInfo {
	return BuildInfo{
		BuildTags:          slices.Clone(LocalBuildTags),
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              false,
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
	CueMeta     *cueutil.Meta

	// Override to explicitly allow the Encore version to be set.
	EncoreVersion option.Option[string]
}

type CompileResult struct {
	Dir         string
	Exe         string
	Configs     map[string]string
	ConfigFiles fs.FS
}

type TestParams struct {
	Compile CompileParams

	// Env sets environment variables for "go test".
	Env []string

	// Args sets extra arguments for "go test".
	Args []string

	// Stdout and Stderr are where to redirect "go test" output.
	Stdout, Stderr io.Writer
}

type GenUserFacingParams struct {
	App   *apps.Instance
	Parse *ParseResult
}

type Impl interface {
	Parse(context.Context, ParseParams) (*ParseResult, error)
	Compile(context.Context, CompileParams) (*CompileResult, error)
	Test(context.Context, TestParams) error
	GenUserFacing(context.Context, GenUserFacingParams) error
}
