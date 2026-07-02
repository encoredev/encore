package dockerbuild

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"encr.dev/pkg/builder"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func testImageConfig(c *qt.C) (DescribeConfig, HostPath) {
	artifacts := paths.FS(c.TempDir())
	writeFiles(c, artifacts, map[string]string{
		"entrypoint":       "echo hello",
		"package.json":     `{"name": "package/name"}`,
		"node_modules/foo": "foo",
		// pnpm/npm/yarn bookkeeping files that must be excluded from the deps
		// layer so its digest stays stable across installs. See pnpm#9474.
		"node_modules/.modules.yaml":                 "prunedAt: Mon, 01 Jan 2024 00:00:00 GMT\n",
		"node_modules/.pnpm-workspace-state.json":    `{"lastValidatedTimestamp":1}`,
		"node_modules/.pnpm-workspace-state-v1.json": `{"lastValidatedTimestamp":1}`,
	})
	runtimes := paths.FS(c.TempDir())
	writeFiles(c, runtimes, map[string]string{
		"js/encore-runtime.node":     "node runtime",
		"js/encore.dev/package.json": `{"name": "encore.dev"}`,
	})

	cfg := DescribeConfig{
		Meta:     &meta.Data{},
		Runtimes: HostPath(runtimes),
		BundleSource: option.Some(BundleSourceSpec{
			Source:         HostPath(artifacts),
			Dest:           "/workspace",
			AppRootRelpath: ".",
		}),
		Compile: &builder.CompileResult{Outputs: []builder.BuildOutput{
			&builder.JSBuildOutput{
				ArtifactDir: artifacts,
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

	encoreBinaries := paths.FS(c.TempDir())
	writeFiles(c, encoreBinaries, map[string]string{
		"supervisor.bin": "supervisor",
	})
	supervisorPath := HostPath(encoreBinaries.Join("supervisor.bin"))

	return cfg, supervisorPath
}

func TestBuildImage(t *testing.T) {
	c := qt.New(t)
	cfg, supervisorPath := testImageConfig(c)

	spec, err := describe(cfg)
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	buildTime := time.Unix(1234567890, 0)
	img, err := BuildImage(ctx, spec, ImageBuildConfig{
		BuildTime:      buildTime,
		SupervisorPath: option.Some(supervisorPath),
	})
	c.Assert(err, qt.IsNil)

	_, err = img.Digest()
	c.Assert(err, qt.IsNil)

	// The image contents should be split into layers by volatility:
	// runtime, dependencies, application, and configuration.
	layers, err := img.Layers()
	c.Assert(err, qt.IsNil)
	c.Assert(len(layers), qt.Equals, 4)

	c.Assert(layerEntryNames(c, layers[0]), qt.DeepEquals, []string{
		"encore/",
		"encore/runtimes/",
		"encore/runtimes/js/",
		"encore/runtimes/js/encore-runtime.node",
		"encore/runtimes/js/encore.dev/",
		"encore/runtimes/js/encore.dev/package.json",
	})
	c.Assert(layerEntryNames(c, layers[1]), qt.DeepEquals, []string{
		"workspace/",
		"workspace/node_modules/",
		"workspace/node_modules/foo",
	})
	c.Assert(layerEntryNames(c, layers[2]), qt.DeepEquals, []string{
		"workspace/",
		"workspace/entrypoint",
		"workspace/package.json",
	})
	c.Assert(layerEntryNames(c, layers[3]), qt.DeepEquals, []string{
		"encore/build-info.json",
		"encore/meta",
	})
}

// TestBuildImage_ReproducibleLayers verifies that building the same app twice
// produces identical layer digests, even with different build times, so that
// unchanged layers can be reused from registry and pull caches.
func TestBuildImage_ReproducibleLayers(t *testing.T) {
	c := qt.New(t)
	cfg, supervisorPath := testImageConfig(c)

	ctx := context.Background()
	build := func(buildTime time.Time) []v1.Layer {
		spec, err := describe(cfg)
		c.Assert(err, qt.IsNil)
		img, err := BuildImage(ctx, spec, ImageBuildConfig{
			BuildTime:      buildTime,
			SupervisorPath: option.Some(supervisorPath),
		})
		c.Assert(err, qt.IsNil)
		layers, err := img.Layers()
		c.Assert(err, qt.IsNil)
		return layers
	}

	first := build(time.Unix(1234567890, 0))
	second := build(time.Unix(2222222222, 0))
	c.Assert(len(second), qt.Equals, len(first))

	for i := range first {
		firstDigest, err := first[i].Digest()
		c.Assert(err, qt.IsNil)
		secondDigest, err := second[i].Digest()
		c.Assert(err, qt.IsNil)
		c.Assert(secondDigest, qt.Equals, firstDigest, qt.Commentf("layer %d digest differs", i))
	}
}

// TestBuildImage_DeterministicDepsLayerAcrossInstalls verifies that two installs
// producing identical dependencies but a different pnpm prunedAt timestamp in
// node_modules/.modules.yaml yield the same dependency-layer digest, so the
// (largest) node_modules layer is not needlessly re-pushed and re-pulled on every
// deploy. See https://github.com/pnpm/pnpm/issues/9474.
func TestBuildImage_DeterministicDepsLayerAcrossInstalls(t *testing.T) {
	c := qt.New(t)
	cfg, supervisorPath := testImageConfig(c)

	source, ok := cfg.BundleSource.Get()
	c.Assert(ok, qt.IsTrue)
	modulesYAML := filepath.Join(string(source.Source), "node_modules", ".modules.yaml")

	ctx := context.Background()
	depsDigest := func() string {
		spec, err := describe(cfg)
		c.Assert(err, qt.IsNil)
		img, err := BuildImage(ctx, spec, ImageBuildConfig{
			BuildTime:      time.Unix(1234567890, 0),
			SupervisorPath: option.Some(supervisorPath),
		})
		c.Assert(err, qt.IsNil)
		layers, err := img.Layers()
		c.Assert(err, qt.IsNil)
		c.Assert(len(layers), qt.Equals, 4)

		// Layer index 1 is the dependency (node_modules) layer; the volatile
		// bookkeeping files must be excluded from it.
		deps := layers[1]
		c.Assert(layerEntryNames(c, deps), qt.DeepEquals, []string{
			"workspace/",
			"workspace/node_modules/",
			"workspace/node_modules/foo",
		})
		digest, err := deps.Digest()
		c.Assert(err, qt.IsNil)
		return digest.String()
	}

	c.Assert(os.WriteFile(modulesYAML, []byte("prunedAt: Mon, 01 Jan 2024 00:00:00 GMT\n"), 0644), qt.IsNil)
	first := depsDigest()

	c.Assert(os.WriteFile(modulesYAML, []byte("prunedAt: Tue, 02 Jul 2030 12:34:56 GMT\n"), 0644), qt.IsNil)
	second := depsDigest()

	c.Assert(second, qt.Equals, first, qt.Commentf("deps layer digest changed when only .modules.yaml content changed"))
}

func layerEntryNames(c *qt.C, layer v1.Layer) []string {
	rc, err := layer.Uncompressed()
	c.Assert(err, qt.IsNil)
	defer func() { _ = rc.Close() }()

	tr := tar.NewReader(rc)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		names = append(names, hdr.Name)
	}
	return names
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
