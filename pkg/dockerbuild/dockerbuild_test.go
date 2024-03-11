package dockerbuild

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/builder"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestBuildImage(t *testing.T) {
	c := qt.New(t)

	artifacts := paths.FS(c.TempDir())
	writeFiles(c, artifacts, map[string]string{
		"entrypoint":       "echo hello",
		"package.json":     `{"name": "package/name"}`,
		"node_modules/foo": "foo",
	})
	runtimes := paths.FS(c.TempDir())
	writeFiles(c, runtimes, map[string]string{
		"js/encore-runtime.node":     "node runtime",
		"js/encore.dev/package.json": `{"name": "encore.dev"}`,
	})

	cfg := DescribeConfig{
		Meta:     &meta.Data{},
		Runtimes: HostPath(runtimes),
		Compile: &builder.CompileResult{Outputs: []builder.BuildOutput{
			&builder.JSBuildOutput{
				ArtifactDir: artifacts,
				PackageJson: artifacts.Join("package.json"),
				NodeModules: option.Some(artifacts.Join("node_modules")),
				Entrypoints: []builder.Entrypoint{{
					Cmd: builder.CmdSpec{
						Command:          builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
						PrioritizedFiles: builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
					},
					Services:           []string{"foo", "bar"},
					Gateways:           []string{"baz", "qux"},
					UseRuntimeConfigV2: true,
				}},
			},
		}},
	}

	spec, err := describe(cfg)
	c.Assert(err, qt.IsNil)

	encoreBinaries := paths.FS(c.TempDir())
	writeFiles(c, encoreBinaries, map[string]string{
		"supervisor.bin": "supervisor",
	})

	ctx := context.Background()
	buildTime := time.Unix(1234567890, 0)
	img, err := BuildImage(ctx, spec, ImageBuildConfig{
		BuildTime:      buildTime,
		SupervisorPath: option.Some(HostPath(encoreBinaries.Join("supervisor.bin"))),
	})
	c.Assert(err, qt.IsNil)

	_, err = img.Digest()
	c.Assert(err, qt.IsNil)
	// Note: this digest changes depending on the machine it's being built on
	// c.Assert(digest.String(), qt.Equals, "sha256:6e0032a1560c506901bbc1bb291d7655639d242f5ca09d5e119876830e34813d")
}

func writeFiles(c *qt.C, dir paths.FS, files map[string]string) {
	for name, content := range files {
		c.Assert(filepath.IsLocal(name), qt.IsTrue)
		path := string(dir.Join(name))

		err := os.MkdirAll(filepath.Dir(path), 0755)
		c.Assert(err, qt.IsNil)

		err = os.WriteFile(path, []byte(content), 0755)
		c.Assert(err, qt.IsNil)
	}
}
