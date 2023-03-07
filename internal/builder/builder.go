package builder

import (
	"io/fs"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/optracker"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/experiments"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type ParseParams struct {
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
	App         *apps.Instance
	Parse       *ParseResult
	OpTracker   *optracker.OpTracker
	Experiments *experiments.Set
	WorkingDir  string
	ListenAddr  string
	CueMeta     *cueutil.Meta
}

type CompileResult struct {
	Dir         string
	Exe         string
	Configs     map[string]string
	ConfigFiles fs.FS
}
