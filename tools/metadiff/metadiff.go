package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/testing/protocmp"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/builder"
	"encr.dev/internal/builder/builderimpl"
	"encr.dev/pkg/vcs"
	metav1 "encr.dev/proto/encore/parser/meta/v1"
	schemav1 "encr.dev/proto/encore/parser/schema/v1"
	"encr.dev/v2/v2builder"
)

func main() {
	rootDir := must(os.Getwd())
	app := apps.NewInstance(rootDir, "metadiff", "")

	v1Builder := builderimpl.Legacy{}
	v2Builder := v2builder.BuilderImpl{}

	vcsRevision := vcs.GetRevision(app.Root())
	buildInfo := builder.BuildInfo{
		BuildTags:          builder.LocalBuildTags,
		CgoEnabled:         true,
		StaticLink:         false,
		Debug:              false,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		KeepOutput:         false,
		Revision:           vcsRevision.Revision,
		UncommittedChanges: vcsRevision.Uncommitted,
	}

	parseParams := builder.ParseParams{
		Build:       buildInfo,
		App:         app,
		Experiments: nil,
		WorkingDir:  ".",
		ParseTests:  false,
	}

	// Hide verbose logging
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	v1Parse := must(v1Builder.Parse(parseParams))
	v2Parse := must(v2Builder.Parse(parseParams))

	baseOpts := []cmp.Option{
		protocmp.Transform(),
		protocmp.IgnoreFields(&schemav1.Decl{}, "loc", "id"),
		protocmp.IgnoreFields(&schemav1.Named{}, "id"),
		protocmp.IgnoreFields(&metav1.AuthHandler{}, "loc"),
		protocmp.IgnoreFields(&metav1.RPC{}, "loc"),
		protocmp.IgnoreFields(&metav1.Package{}, "trace_nodes"),
		protocmp.SortRepeatedFields((*metav1.Service)(nil), "rpcs"),
		protocmp.FilterField((*metav1.Service)(nil), "rpcs", protocmp.SortRepeated(func(a, b *schemav1.Decl) bool {
			return a.Name < b.Name
		})),
		protocmp.FilterField((*metav1.Data)(nil), "decls", protocmp.SortRepeated(func(a, b *schemav1.Decl) bool {
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.String() < b.String()
		})),
		protocmp.FilterField((*metav1.Data)(nil), "pkgs", protocmp.SortRepeated(func(a, b *metav1.Package) bool {
			return a.RelPath < b.RelPath
		})),
	}

	ignorePointerComparer := cmp.Comparer(func(a, b *schemav1.Type) bool {
		if ptr := a.GetPointer(); ptr != nil {
			a = ptr.Base
		}
		if ptr := b.GetPointer(); ptr != nil {
			b = ptr.Base
		}
		return cmp.Equal(a, b, baseOpts...)
	})

	allOpts := append(slices.Clone(baseOpts),
		protocmp.FilterField((*metav1.PubSubTopic)(nil), "message_type", ignorePointerComparer),
		protocmp.FilterField((*metav1.AuthHandler)(nil), "params", ignorePointerComparer),
		protocmp.FilterField((*metav1.RPC)(nil), "request_schema", ignorePointerComparer),
		protocmp.FilterField((*metav1.RPC)(nil), "response_schema", ignorePointerComparer),
	)

	diff := ansiDiff(v1Parse.Meta, v2Parse.Meta, allOpts...)
	if diff == "" {
		fmt.Println("identical metadata")
	} else {
		fmt.Printf("metadata differs: (-v1 +v2):\n%s\n", diff)
	}
}

func must[T any](val T, err error) T {
	if err != nil {
		log.Fatalln(err)
	}
	return val
}

// ansiDiff is from https://github.com/google/go-cmp/issues/230#issuecomment-665750648
func ansiDiff(x, y interface{}, opts ...cmp.Option) string {
	escapeCode := func(code int) string {
		return fmt.Sprintf("\x1b[%dm", code)
	}
	diff := cmp.Diff(x, y, opts...)
	if diff == "" {
		return ""
	}
	ss := strings.Split(diff, "\n")
	for i, s := range ss {
		switch {
		case strings.HasPrefix(s, "-"):
			ss[i] = escapeCode(31) + s + escapeCode(0)
		case strings.HasPrefix(s, "+"):
			ss[i] = escapeCode(32) + s + escapeCode(0)
		}
	}
	return strings.Join(ss, "\n")
}
