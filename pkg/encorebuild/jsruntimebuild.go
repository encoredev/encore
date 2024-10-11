package encorebuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog"

	"encr.dev/pkg/encorebuild/buildconf"
	. "encr.dev/pkg/encorebuild/buildutil"
	"encr.dev/pkg/encorebuild/compile"
	"encr.dev/pkg/encorebuild/gentypedefs"
)

func NewJSRuntimeBuilder(cfg *buildconf.Config) *JSRuntimeBuilder {
	if cfg.RepoDir == "" {
		Bailf("repo dir not set")
	} else if _, err := os.Stat(cfg.RepoDir); err != nil {
		Bailf("repo does not exist")
	}

	workdir := filepath.Join(cfg.CacheDir, "jsruntimebuild", cfg.OS, cfg.Arch)
	Check(os.MkdirAll(workdir, 0755))
	return &JSRuntimeBuilder{
		log:     cfg.Log,
		cfg:     cfg,
		workdir: workdir,
	}
}

type JSRuntimeBuilder struct {
	log     zerolog.Logger
	cfg     *buildconf.Config
	workdir string
}

func (b *JSRuntimeBuilder) Build() {
	b.log.Info().Msgf("Building local JS runtime targeting %s/%s", b.cfg.OS, b.cfg.Arch)
	b.buildRustModule()
	b.genTypeDefWrappers()
	b.makeDistFolder()

	if b.cfg.CopyToRepo {
		b.copyNativeModule()
	}
}

// buildRustModule builds the Rust module for the JS runtime.
func (b *JSRuntimeBuilder) buildRustModule() {
	b.log.Info().Msg("building rust module")

	// Figure out the names of the compiled and target binaries.
	compiledBinaryName := func() string {
		switch b.cfg.OS {
		case "darwin":
			return "libencore_js_runtime.dylib"
		case "linux":
			return "libencore_js_runtime.so"
		case "windows":
			return "encore_js_runtime.dll"
		default:
			Bailf("unknown OS: %s", b.cfg.OS)
			panic("unreachable")
		}
	}()

	compile.RustBinary(
		b.cfg,
		compiledBinaryName,
		b.NativeModuleOutput(),
		filepath.Join(b.cfg.RepoDir, "runtimes", "js"),
		"gnu",
		"TYPE_DEF_TMP_PATH="+b.typeDefPath(),
		"ENCORE_VERSION="+b.cfg.Version,
		"ENCORE_WORKDIR="+b.workdir,
	)
}

// genTypeDefWrappers generates the napi.cjs and napi.d.cts files for
// use by the JS SDK.
func (b *JSRuntimeBuilder) genTypeDefWrappers() {
	b.log.Info().Msg("generating napi type definitions")
	napiPath := filepath.Join(b.npmPackagePath(), napiRelPath)
	Check(os.MkdirAll(napiPath, 0755))

	cfg := gentypedefs.Config{
		ReleaseVersion: b.cfg.Version,
		TypeDefFile:    b.typeDefPath(),
		DtsOutputFile:  filepath.Join(napiPath, "napi.d.cts"),
		CjsOutputFile:  filepath.Join(napiPath, "napi.cjs"),
	}
	Check(gentypedefs.Generate(cfg))
}

// makeDistFolder creates the dist folder for the JS runtime,
// and fixes the imports to be ESM-compatible.
func (b *JSRuntimeBuilder) makeDistFolder() {
	b.log.Info().Msg("creating dist folder")
	// Sanity-check the runtime dir configuration so we don't delete the wrong thing.
	base := filepath.Base(b.cfg.RepoDir)
	if b.cfg.RepoDir == "" || (base != "encore" && base != "encr.dev") {
		Bailf("invalid repo directory %q, aborting", b.cfg.RepoDir)
	}

	pkgPath := filepath.Join(b.jsRuntimePath(), "encore.dev")
	distPath := filepath.Join(pkgPath, "dist")
	Check(os.RemoveAll(distPath))

	// Run 'npm install'.
	{
		cmd := exec.Command("npm", "install")
		cmd.Dir = pkgPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Check(cmd.Run())
	}

	// Run 'npm run build'.
	{
		cmd := exec.Command("npm", "run", "build")
		cmd.Dir = pkgPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Check(cmd.Run())
	}

	// Copy the napi directory over.
	{
		src := filepath.Join(b.npmPackagePath(), napiRelPath)
		dst := filepath.Join(distPath, napiRelPath)
		cmd := exec.Command("cp", "-r", src, dst)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Check(cmd.Run())
	}

	// Run 'tsc-esm-fix'.
	{
		cmd := exec.Command("./node_modules/.bin/tsc-esm-fix", "--target=dist")
		cmd.Dir = pkgPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Check(cmd.Run())
	}

}

func (b *JSRuntimeBuilder) copyNativeModule() {
	b.log.Info().Msg("copying native module")
	copyFile := func(src, dst string) {
		cmd := exec.Command("cp", src, dst)
		out, err := cmd.CombinedOutput()
		if err != nil {
			Bailf("unable to copy native module: %v: %s", err, out)
		}
	}

	src := b.NativeModuleOutput()
	suffix := ""
	if b.cfg.OS != runtime.GOOS || b.cfg.Arch != runtime.GOARCH {
		suffix = "-" + b.cfg.OS + "-" + b.cfg.Arch
	}
	dst := filepath.Join(b.jsRuntimePath(), "encore-runtime.node"+suffix)
	copyFile(src, dst)
}

func (b *JSRuntimeBuilder) NativeModuleOutput() string {
	return filepath.Join(b.workdir, "encore-runtime.node")
}

func (b *JSRuntimeBuilder) typeDefPath() string {
	return filepath.Join(b.workdir, "typedefs.ndjson")
}

func (b *JSRuntimeBuilder) npmPackagePath() string {
	return filepath.Join(b.jsRuntimePath(), "encore.dev")
}

func (b *JSRuntimeBuilder) jsRuntimePath() string {
	return filepath.Join(b.cfg.RepoDir, "runtimes", "js")
}

// napiRelPath is the relative path from the package root to the napi directory.
var napiRelPath = filepath.Join("internal", "runtime", "napi")

func PublishNPMPackages(repoDir, version string) {
	packages := []string{"encore.dev"}
	npmVersion := strings.TrimPrefix(version[1:], "v")

	npmTag := "latest"
	switch {
	case strings.Contains(version, "-beta."):
		npmTag = "beta"
	case strings.Contains(version, "-nightly."):
		npmTag = "nightly"
	}

	// Configure the auth token
	{
		authToken := os.Getenv("NPM_PUBLISH_TOKEN")
		if authToken == "" {
			Bailf("NPM_PUBLISH_TOKEN not set")
		}
		cmd := exec.Command("npm", "set", "//registry.npmjs.org/:_authToken="+authToken)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Check(cmd.Run())
	}

	for _, pkg := range packages {
		pkgDir := filepath.Join(repoDir, "runtimes", "js", pkg)

		// Run 'npm version'.
		{
			cmd := exec.Command("npm", "version", "--no-git-tag-version", "--no-commit-hooks", npmVersion)
			cmd.Dir = pkgDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			Check(cmd.Run())
		}

		// Run 'npm publish'.
		{
			cmd := exec.Command("npm", "publish",
				"--tolerate-republish",
				"--access", "public",
				"--tag", npmTag,
			)
			cmd.Dir = pkgDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			Check(cmd.Run())
		}
	}
}
