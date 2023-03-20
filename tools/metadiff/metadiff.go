package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/testing/protocmp"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/builder"
	"encr.dev/internal/builder/builderimpl"
	"encr.dev/v2/v2builder"
)

func main() {
	v1Builder := builderimpl.Legacy{}
	v2Builder := v2builder.BuilderImpl{}

	buildInfo := builder.BuildInfo{
		BuildTags:  builder.LocalBuildTags,
		CgoEnabled: true,
		StaticLink: false,
		Debug:      false,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		KeepOutput: false,
	}

	rootDir := must(os.Getwd())
	app := apps.NewInstance(rootDir, "metadiff", "")
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

	diff := ansiDiff(v1Parse.Meta, v2Parse.Meta, protocmp.Transform())
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
