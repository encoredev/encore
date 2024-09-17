package encorebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"encr.dev/internal/version"
	"encr.dev/pkg/encorebuild/buildconf"
	. "encr.dev/pkg/encorebuild/buildutil"
	"encr.dev/pkg/encorebuild/compile"
	"encr.dev/pkg/encorebuild/githubrelease"
)

// A DistBuilder is a builder for a specific distribution of Encore.
//
// Anything which does not need to be built for a specific distribution
// should be built in the main builder before these are invoked.
//
// Make release will run multiple of these in parallel to build all the
// distributions.
type DistBuilder struct {
	Cfg              *buildconf.Config
	DistBuildDir     string // The directory to build into
	ArtifactsTarFile string // The directory to put the final tar.gz artifact into
}

func (d *DistBuilder) buildEncoreCLI() {
	// Build the CLI binaries.
	d.Cfg.Log.Info().Msg("building encore binary...")

	linkerOpts := []string{
		"-X", fmt.Sprintf("'encr.dev/internal/version.Version=%s'", d.Cfg.Version),
	}

	// If we're building a nightly, devel or beta version, we need to set the default config directory
	var versionSuffix string
	switch version.ChannelFor(d.Cfg.Version) {
	case version.GA:
		versionSuffix = ""
	case version.Beta:
		versionSuffix = "-beta"
	case version.Nightly:
		versionSuffix = "-nightly"
	case version.DevBuild:
		versionSuffix = "-develop"
	default:
		Bailf("unknown version channel for %s", d.Cfg.Version)
	}

	if versionSuffix != "" {
		linkerOpts = append(linkerOpts,
			"-X", "'encr.dev/internal/conf.defaultConfigDirectory=encore"+versionSuffix+"'",
		)
	}

	compile.GoBinary(
		d.Cfg,
		join(d.DistBuildDir, "bin", "encore"+versionSuffix),
		"./cli/cmd/encore",
		linkerOpts,
	)
	d.Cfg.Log.Info().Msg("encore built successfully")
}

func (d *DistBuilder) buildGitHook() {
	// Build the git-remote-encore binary.
	d.Cfg.Log.Info().Msg("building git-remote-encore binary...")
	compile.GoBinary(
		d.Cfg,
		join(d.DistBuildDir, "bin", "git-remote-encore"),
		"./cli/cmd/git-remote-encore",
		nil,
	)
	d.Cfg.Log.Info().Msg("git-remote-encore built successfully")
}

func (d *DistBuilder) buildTSBundler() {
	// Build the TS bundler.
	d.Cfg.Log.Info().Msg("building tsbundler binary...")

	linkerOpts := []string{
		"-X", fmt.Sprintf("'encr.dev/internal/version.Version=%s'", d.Cfg.Version),
	}

	compile.GoBinary(
		d.Cfg,
		join(d.DistBuildDir, "bin", "tsbundler-encore"),
		"./cli/cmd/tsbundler-encore",
		linkerOpts,
	)
	d.Cfg.Log.Info().Msg("tsbundler built successfully")
}

func (d *DistBuilder) buildTSParser() {
	// Build the TS parser.
	d.Cfg.Log.Info().Msg("building tsparser binary...")
	compile.RustBinary(
		d.Cfg,
		"tsparser-encore",
		join(d.DistBuildDir, "bin", "tsparser-encore"),
		"./tsparser",
		"gnu",
		fmt.Sprintf("ENCORE_VERSION=%s", d.Cfg.Version),
	)
	d.Cfg.Log.Info().Msg("tsparser built successfully")
}

func (d *DistBuilder) buildNodePlugin() {
	builder := NewJSRuntimeBuilder(d.Cfg)
	builder.Build()

	d.Cfg.Log.Info().Msg("copying encore runtime for JS...")
	{
		cmd := exec.Command("cp", "-r", "runtimes/js/.", join(d.DistBuildDir, "runtimes", "js")+"/")
		cmd.Dir = d.Cfg.RepoDir
		// nosemgrep
		if out, err := cmd.CombinedOutput(); err != nil {
			Bailf("failed to copy encore go runtime: %v: %s", err, out)
		}
	}

	{
		src := builder.NativeModuleOutput()
		dst := join(d.DistBuildDir, "runtimes", "js", "encore-runtime.node")
		cmd := exec.Command("cp", src, dst)
		// nosemgrep
		if out, err := cmd.CombinedOutput(); err != nil {
			Bailf("failed to copy encore go runtime: %v: %s", err, out)
		}
	}

	d.Cfg.Log.Info().Msg("encore runtime for js copied successfully")
}

func (d *DistBuilder) downloadEncoreGo() {
	// Step 1: Find out the latest release version for Encore's Go distribution
	d.Cfg.Log.Info().Msg("downloading latest encore-go...")
	encoreGoArchive := githubrelease.DownloadLatest(d.Cfg, "encoredev", "go")

	d.Cfg.Log.Info().Msg("extracting encore-go...")
	githubrelease.Extract(encoreGoArchive, d.DistBuildDir)
	d.Cfg.Log.Info().Msg("encore-go extracted successfully")
}

func (d *DistBuilder) copyEncoreRuntimeForGo() {
	d.Cfg.Log.Info().Msg("copying encore runtime for Go...")
	cmd := exec.Command("cp", "-r", "runtimes/go/.", join(d.DistBuildDir, "runtimes", "go")+"/")
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		Bailf("failed to copy encore go runtime: %v: %s", err, out)
	}
	d.Cfg.Log.Info().Msg("encore runtime for go copied successfully")
}

// Build builds the distribution running each step in order
func (d *DistBuilder) Build() {
	d.Cfg.Log.Info().Msg("building distribution...")

	if d.DistBuildDir == "" {
		Bailf("DistBuildDir not set")
	}

	// Prepare the target directory.
	Check(os.RemoveAll(d.DistBuildDir))
	Check(os.MkdirAll(d.DistBuildDir, 0755))
	Check(os.MkdirAll(join(d.DistBuildDir, "bin"), 0755))
	Check(os.MkdirAll(join(d.DistBuildDir, "runtimes"), 0755))
	Check(os.MkdirAll(join(d.DistBuildDir, "runtimes", "go"), 0755))
	Check(os.MkdirAll(join(d.DistBuildDir, "runtimes", "js"), 0755))

	// Now we're prepped, start building.
	RunParallel(
		d.buildEncoreCLI,
		d.buildTSBundler,
		d.buildGitHook,
		d.buildTSParser,
		d.buildNodePlugin,
		d.copyEncoreRuntimeForGo,
		d.downloadEncoreGo,
	)

	// Now tar gzip the directory
	d.Cfg.Log.Info().Str("tar_file", d.ArtifactsTarFile).Msg("creating distribution tar file...")
	TarGzip(d.DistBuildDir, d.ArtifactsTarFile)

	d.Cfg.Log.Info().Str("tar_file", d.ArtifactsTarFile).Msg("distribution built successfully")
}

func join(segs ...string) string {
	return filepath.Join(segs...)
}
