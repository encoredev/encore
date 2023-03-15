package builder

import (
	"io/fs"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/experiments"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

var LocalBuildTags = []string{"encore_local", "encore_no_gcp", "encore_no_aws", "encore_no_azure"}

type BuildInfo struct {
	BuildTags    []string
	CgoEnabled   bool
	StaticLink   bool
	Debug        bool
	GOOS, GOARCH string
	KeepOutput   bool
}

type ParseParams struct {
	Build         BuildInfo
	App           *apps.Instance
	Experiments   *experiments.Set
	WorkingDir    string
	ParseTests    bool
	ScriptMainPkg string
}

type Impl interface {
	Parse(ParseParams) (*ParseResult, error)
	Compile(CompileParams) (*CompileResult, error)
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
