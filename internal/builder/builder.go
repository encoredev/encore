package builder

import (
	"context"
	"io"
	"io/fs"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/experiments"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

var LocalBuildTags = []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"}

type BuildInfo struct {
	BuildTags          []string
	CgoEnabled         bool
	StaticLink         bool
	Debug              bool
	GOOS, GOARCH       string
	KeepOutput         bool
	Revision           string
	UncommittedChanges bool
}

type ParseParams struct {
	Build         BuildInfo
	App           *apps.Instance
	Experiments   *experiments.Set
	WorkingDir    string
	ParseTests    bool
	ScriptMainPkg string
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

type Impl interface {
	Parse(context.Context, ParseParams) (*ParseResult, error)
	Compile(context.Context, CompileParams) (*CompileResult, error)
	Test(context.Context, TestParams) error
}
