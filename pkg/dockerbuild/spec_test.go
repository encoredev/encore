package dockerbuild

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"encr.dev/pkg/builder"
	"encr.dev/pkg/option"
	"encr.dev/pkg/supervisor"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func TestBuild_Node(t *testing.T) {
	c := qt.New(t)
	cfg := DescribeConfig{
		Meta:     &meta.Data{Language: meta.Lang_TYPESCRIPT},
		Runtimes: "/host/runtimes",
		BundleSource: option.Some(BundleSourceSpec{
			Source:         "/host/app",
			Dest:           "/image",
			AppRootRelpath: ".",
		}),
		Compile: &builder.CompileResult{Outputs: []builder.BuildOutput{
			&builder.JSBuildOutput{
				ArtifactDir: "/host/artifacts",
				Entrypoints: []builder.Entrypoint{{
					Cmd: builder.CmdSpec{
						Command:          builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
						PrioritizedFiles: builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
					},
					Services: []string{"foo", "bar"},
					Gateways: []string{"baz", "qux"},
				}},
			},
		}},
	}
	spec, err := describe(cfg)
	c.Assert(err, qt.IsNil)

	meta, err := proto.Marshal(cfg.Meta)
	c.Assert(err, qt.IsNil)

	opts := append([]cmp.Option{cmpopts.EquateEmpty()}, option.CmpOpts()...)
	c.Assert(spec, qt.CmpEquals(opts...), &ImageSpec{
		Entrypoint: []string{"/image/.encore/build/entrypoint"},
		Env: []string{
			"ENCORE_RUNTIME_LIB=/encore/runtimes/js/encore-runtime.node",
		},
		WorkingDir: "/",
		BuildInfo:  BuildInfoSpec{InfoPath: defaultBuildInfoPath},
		WriteFiles: map[ImagePath][]byte{defaultMetaPath: meta},
		CopyData: map[ImagePath]HostPath{
			"/encore/runtimes/js": "/host/runtimes/js",
		},
		BundleSource: option.Some(BundleSourceSpec{
			Source:         "/host/app",
			Dest:           "/image",
			AppRootRelpath: ".",
			ExcludeSource:  []RelPath{},
			IncludeSource:  []RelPath{},
		}),
		Supervisor:      option.None[SupervisorSpec](),
		BundledServices: []string{"bar", "foo"},
		BundledGateways: []string{"baz", "qux"},
		DockerBaseImage: "scratch",
		FeatureFlags:    map[FeatureFlag]bool{NewRuntimeConfig: true},
		StargzPrioritizedFiles: []ImagePath{
			"/image/.encore/build/entrypoint",
			"/encore/runtimes/js/encore-runtime.node",
		},
	})
}

func TestBuild_Go_SingleBinary(t *testing.T) {
	c := qt.New(t)
	cfg := DescribeConfig{
		Meta: &meta.Data{},
		Compile: &builder.CompileResult{Outputs: []builder.BuildOutput{
			&builder.GoBuildOutput{
				ArtifactDir: "/host/artifacts",
				Entrypoints: []builder.Entrypoint{
					{
						Cmd: builder.CmdSpec{
							Command:          builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
							PrioritizedFiles: builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
						},
						Services: []string{"foo", "bar"},
					},
				},
			},
		}},
	}

	spec, err := describe(cfg)
	c.Assert(err, qt.IsNil)

	opts := append([]cmp.Option{cmpopts.EquateEmpty()}, option.CmpOpts()...)
	c.Assert(spec, qt.CmpEquals(opts...), &ImageSpec{
		Entrypoint: []string{"/artifacts/0/build/entrypoint"},
		Env:        nil,
		WorkingDir: "/",
		BuildInfo:  BuildInfoSpec{InfoPath: defaultBuildInfoPath},
		CopyData: map[ImagePath]HostPath{
			"/artifacts/0/build": "/host/artifacts",
		},
		BundledServices: []string{"bar", "foo"},
		BundleSource:    option.Option[BundleSourceSpec]{},
		Supervisor:      option.None[SupervisorSpec](),
		DockerBaseImage: "scratch",
		FeatureFlags:    map[FeatureFlag]bool{},
		StargzPrioritizedFiles: []ImagePath{
			"/artifacts/0/build/entrypoint",
		},
		WriteFiles: map[ImagePath][]byte{
			defaultMetaPath: {},
		},
	})
}

func TestBuild_Go_MultiProc(t *testing.T) {
	c := qt.New(t)
	cfg := DescribeConfig{
		Meta: &meta.Data{Language: meta.Lang_TYPESCRIPT},
		Compile: &builder.CompileResult{Outputs: []builder.BuildOutput{
			&builder.GoBuildOutput{
				ArtifactDir: "/host/artifacts",
				Entrypoints: []builder.Entrypoint{
					{
						Cmd: builder.CmdSpec{
							Command:          builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
							PrioritizedFiles: builder.ArtifactStrings{"${ARTIFACT_DIR}/entrypoint"},
						},
						Services: []string{"foo"},
					},
					{
						Cmd: builder.CmdSpec{
							Command:          builder.ArtifactStrings{"${ARTIFACT_DIR}/other-entrypoint"},
							PrioritizedFiles: builder.ArtifactStrings{"${ARTIFACT_DIR}/other-entrypoint"},
						},
						Services: []string{"bar"},
					},
				},
			},
		}},
	}

	spec, err := describe(cfg)
	c.Assert(err, qt.IsNil)

	meta, err := proto.Marshal(cfg.Meta)
	c.Assert(err, qt.IsNil)

	opts := append([]cmp.Option{cmpopts.EquateEmpty()}, option.CmpOpts()...)
	c.Assert(spec, qt.CmpEquals(opts...), &ImageSpec{
		Entrypoint: []string{"/encore/bin/supervisor", "-c", string(defaultSupervisorConfigPath)},
		Env:        nil,
		WorkingDir: "/",
		BuildInfo:  BuildInfoSpec{InfoPath: defaultBuildInfoPath},
		CopyData: map[ImagePath]HostPath{
			"/artifacts/0/build": "/host/artifacts",
		},
		BundledServices: []string{"bar", "foo"},
		BundleSource:    option.Option[BundleSourceSpec]{},
		Supervisor: option.Some(SupervisorSpec{
			MountPath:  "/encore/bin/supervisor",
			ConfigPath: defaultSupervisorConfigPath,
			Config: &supervisor.Config{
				Procs: []supervisor.Proc{
					{
						ID:       "proc-id",
						Command:  []string{"/artifacts/0/build/entrypoint"},
						Services: []string{"foo"},
						Gateways: []string{},
					},
					{
						ID:       "proc-id",
						Command:  []string{"/artifacts/0/build/other-entrypoint"},
						Services: []string{"bar"},
						Gateways: []string{},
					},
				},
			},
		}),
		DockerBaseImage: "scratch",
		FeatureFlags:    map[FeatureFlag]bool{NewRuntimeConfig: true},
		StargzPrioritizedFiles: []ImagePath{
			"/encore/bin/supervisor",
			"/artifacts/0/build/entrypoint",
			"/artifacts/0/build/other-entrypoint",
		},
		WriteFiles: map[ImagePath][]byte{
			defaultMetaPath: meta,
		},
	})
}

// describe is like Describe but mocks the proc id generation
// for reproducible tests.
func describe(cfg DescribeConfig) (*ImageSpec, error) {
	b := newImageSpecBuilder()
	b.procIDGen = func() string { return "proc-id" }
	return b.Describe(cfg)
}
