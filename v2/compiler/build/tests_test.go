package build

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
)

func TestGenerateTestSpec(t *testing.T) {
	ctx := context.Background()
	static := &config.Static{AppCommit: config.CommitInfo{Revision: "abc123", Uncommitted: true}, Testing: true}
	b := &builder{
		ctx: ctx,
		cfg: &Config{
			Ctx:          &parsectx.Context{Ctx: ctx, Log: zerolog.Nop(), Build: parsectx.BuildInfo{GOROOT: paths.FS("/goroot")}},
			StaticConfig: static,
		},
		errs:        perr.NewList(ctx, token.NewFileSet()),
		overlayPath: paths.FS("/overlay.json"),
	}

	spec := b.generateTestSpec(&GenerateTestSpecConfig{Args: []string{"./..."}})

	linked := linkedStaticConfig(t, spec.Args)
	if linked.AppCommit != (config.CommitInfo{}) {
		t.Errorf("linked commit = %+v, want none", linked.AppCommit)
	}
	if !linked.Testing {
		t.Error("linked config lost its other fields")
	}
	commitEnv := `ENCORE_APP_COMMIT={"revision":"abc123","uncommitted":true}`
	if !slices.Contains(spec.Environ, commitEnv) {
		t.Errorf("environment lacks %s", commitEnv)
	}
	if static.AppCommit.Revision != "abc123" {
		t.Errorf("builder's static config changed to %+v", static.AppCommit)
	}
}

// linkedStaticConfig decodes the static config that args link into the binary.
func linkedStaticConfig(t *testing.T, args []string) config.Static {
	t.Helper()
	i := slices.Index(args, "-ldflags")
	if i < 0 || i+1 >= len(args) {
		t.Fatalf("no -ldflags in %q", args)
	}
	_, encoded, found := strings.Cut(args[i+1], "appconf.static=")
	if !found {
		t.Fatalf("no static config in ldflags %q", args[i+1])
	}
	encoded, _, _ = strings.Cut(encoded, "'")
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode the linked static config: %v", err)
	}
	var static config.Static
	if err := json.Unmarshal(data, &static); err != nil {
		t.Fatalf("parse the linked static config: %v", err)
	}
	return static
}
