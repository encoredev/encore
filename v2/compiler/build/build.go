package build

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
)

type Config struct {
	// Ctx controls the build.
	Ctx *parsectx.Context

	// Overlays describes the code generation overlays to apply,
	// in the form of rewritten files or generated files.
	Overlays []OverlayFile

	// MainPkg is the main package to build.
	MainPkg paths.Pkg

	// KeepOutput keeps the temporary build directory from being deleted in the case of failure.
	KeepOutput bool
}

// OverlayFile describes a file to generate or rewrite.
type OverlayFile struct {
	// Source is where on the filesystem the original file (in the case of a rewrite)
	// or where the generated file should be overlaid into.
	Source paths.FS

	// Contents are the file contents of the overlaid file.
	Contents []byte
}

type Result struct {
	Dir paths.FS
	Exe paths.FS
}

func Build(cfg *Config) *Result {
	b := &builder{
		cfg:  cfg,
		errs: cfg.Ctx.Errs,
	}
	return b.Build()
}

type builder struct {
	// inputs
	cfg *Config

	// internal state

	// errs is the error list to use.
	errs *perr.List

	// o is the overlay to use
	o *overlay

	// workdir is the temporary workdir for the build.
	workdir paths.FS
}

func (b *builder) Build() *Result {
	workdir, err := os.MkdirTemp("", "encore-build")
	if err != nil {
		b.errs.AddStd(err)
	}
	b.workdir = paths.RootedFSPath(workdir, workdir)
	b.o = newOverlay(b.errs, b.workdir)

	defer func() {
		// If we have a bailout or any errors, delete the workdir.
		if _, ok := perr.CatchBailout(recover()); ok || b.errs.Len() > 0 {
			if !b.cfg.KeepOutput && workdir != "" {
				_ = os.RemoveAll(workdir)
			}
		}
	}()

	res := &Result{
		Dir: b.workdir,
		Exe: b.binaryPath(),
	}

	for _, fn := range []func(){
		b.writeModFile,
		b.writeSumFile,
		b.applyRewrites,
		b.buildMain,
	} {
		fn()
		// Abort early if we encountered any errors.
		if b.errs.Len() > 0 {
			break
		}
	}
	return res
}

func (b *builder) writeModFile() {
	newPath := b.cfg.Ctx.Build.EncoreRuntime.ToIO()
	oldPath := "encore.dev"

	modData, err := os.ReadFile(b.cfg.Ctx.MainModuleDir.Join("go.mod").ToIO())
	if err != nil {
		b.errs.Addf(token.NoPos, "unable to read go.mod: %v", err)
		return
	}
	mainMod, err := modfile.Parse("go.mod", modData, nil)
	if err != nil {
		b.errs.Addf(token.NoPos, "unable to parse go.mod: %v", err)
		return
	}

	// Make sure there's a dependency on encore.dev so it can be replaced.
	if err := mainMod.AddRequire("encore.dev", "v0.0.0"); err != nil {
		b.errs.Addf(token.NoPos, "unable to add 'require encore.dev' directive to go.mod: %v", err)
		return
	}
	if err := mainMod.AddReplace(oldPath, "", newPath, ""); err != nil {
		b.errs.Addf(token.NoPos, "unable to add 'replace encore.dev' directive to go.mod: %v", err)
		return
	}

	// We require Go 1.18+ now that we use generics in code gen.
	if !isGo118Plus(mainMod) {
		_ = mainMod.AddGoStmt("1.18")
	}
	mainMod.Cleanup()

	runtimeModData, err := os.ReadFile(filepath.Join(newPath, "go.mod"))
	if err != nil {
		b.errs.Addf(token.NoPos, "unable to read encore runtime's go.mod: %v", err)
		return
	}
	runtimeModfile, err := modfile.Parse("encore-runtime/go.mod", runtimeModData, nil)
	if err != nil {
		b.errs.Addf(token.NoPos, "unable to parse encore runtime's go.mod: %v", err)
		return
	}
	mergeModfiles(mainMod, runtimeModfile)

	modBytes := modfile.Format(mainMod.Syntax)
	b.o.Add(b.cfg.Ctx.MainModuleDir.Join("go.mod"), "go.mod", modBytes)
}

func (b *builder) writeSumFile() {
	appSum, err := os.ReadFile(b.cfg.Ctx.MainModuleDir.Join("go.sum").ToIO())
	if err != nil && !os.IsNotExist(err) {
		b.errs.Addf(token.NoPos, "unable to parse go.sum: %v", err)
		return
	}
	runtimeSum, err := os.ReadFile(b.cfg.Ctx.Build.EncoreRuntime.Join("go.sum").ToIO())
	if err != nil {
		b.errs.Addf(token.NoPos, "unable to parse encore runtime's go.sum: %v", err)
		return
	}
	if !bytes.HasSuffix(appSum, []byte{'\n'}) {
		appSum = append(appSum, '\n')
	}
	data := append(appSum, runtimeSum...)
	b.o.Add(b.cfg.Ctx.MainModuleDir.Join("go.sum"), "go.sum", data)
}

func (b *builder) applyRewrites() {
	for _, f := range b.cfg.Overlays {
		io := f.Source.ToIO()
		baseName := filepath.Base(filepath.Dir(io)) + "__" + filepath.Base(io)
		b.o.Add(f.Source, baseName, f.Contents)
	}
}

func (b *builder) buildMain() {
	overlayData := b.o.Data()
	overlayPath := b.workdir.Join("overlay.json").ToIO()
	if err := os.WriteFile(overlayPath, overlayData, 0644); err != nil {
		b.errs.Addf(token.NoPos, "unable to write overlay file: %v", err)
		return
	}

	build := b.cfg.Ctx.Build
	tags := append([]string{"encore", "encore_internal", "encore_app"}, build.BuildTags...)
	args := []string{
		"build",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + overlayPath,
		"-mod=mod",
		"-o=" + b.binaryPath().ToIO(),
	}

	if b.cfg.Ctx.Build.StaticLink {
		var ldflags string

		// Enable external linking if we use cgo.
		if b.cfg.Ctx.Build.CgoEnabled {
			ldflags = "-linkmode external "
		}

		ldflags += `-extldflags "-static"`
		args = append(args, "-ldflags", ldflags)
	}

	args = append(args, b.cfg.MainPkg.String())

	goroot := build.GOROOT
	cmd := exec.Command(goroot.Join("bin", "go"+b.exe()).ToIO(), args...)

	env := []string{
		"GOROOT=" + goroot.ToIO(),
	}

	if goos := build.GOOS; goos != "" {
		env = append(env, "GOOS="+goos)
	}
	if goarch := build.GOARCH; goarch != "" {
		env = append(env, "GOARCH="+goarch)
	}
	if !build.CgoEnabled {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = b.cfg.Ctx.MainModuleDir.ToIO()
	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) == 0 {
			out = []byte(err.Error())
		}
		// TODO(andre) return this as a better error
		b.errs.Addf(token.NoPos, "unable to build main package: %s", out)
	}
}

// mergeModFiles merges two modfiles, adding the require statements from the latter to the former.
// If both files have the same module requirement, it keeps the one with the greater semver version.
func mergeModfiles(src, add *modfile.File) {
	reqs := src.Require
	for _, a := range add.Require {
		found := false
		for _, r := range src.Require {
			if r.Mod.Path == a.Mod.Path {
				found = true
				// Update the version if the one to add is greater.
				if semver.Compare(a.Mod.Version, r.Mod.Version) > 0 {
					r.Mod.Version = a.Mod.Version
				}
			}
		}
		if !found {
			reqs = append(reqs, a)
		}
	}
	src.SetRequire(reqs)
	src.Cleanup()
}

const binaryName = "encore_app_out"

func (b *builder) exe() string {
	goos := b.cfg.Ctx.Build.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

func (b *builder) binaryPath() paths.FS {
	return b.workdir.Join(binaryName + b.exe())
}

func isGo118Plus(f *modfile.File) bool {
	if f.Go == nil {
		return false
	}
	m := modfile.GoVersionRE.FindStringSubmatch(f.Go.Version)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 1 || (major == 1 && minor >= 18)
}

func newOverlay(errs *perr.List, workdir paths.FS) *overlay {
	return &overlay{
		errs:      errs,
		workdir:   workdir,
		seenNames: make(map[string]bool),
		overlay:   make(map[paths.FS]paths.FS),
	}
}

// overlay tracks a set of overlaid files, for feeding to
// 'go build -overlay'.
type overlay struct {
	errs *perr.List

	// workdir is the work directory to write files to.
	workdir paths.FS

	// seenNames is a set of the base names written to workdir.
	seenNames map[string]bool

	// overlays are the overlay files to use.
	// The keys are the names in the original packages to replace,
	// and the values are where the overlaid files live (in the workdir).
	overlay map[paths.FS]paths.FS
}

func (o *overlay) Add(src paths.FS, baseName string, contents []byte) {
	if _, exists := o.overlay[src]; exists {
		panic(fmt.Sprintf("duplicate overlay of %s", src.ToIO()))
	}

	// Get the base name without the extension
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	// Keep generating names until we get one that doesn't conflict.
	candidate := baseName
	for i := 1; o.seenNames[candidate]; i++ {
		candidate = fmt.Sprintf("%s_%d%s", nameWithoutExt, i, ext)
	}
	o.seenNames[candidate] = true

	dst := o.workdir.Join(candidate)

	o.overlay[src] = dst
	if err := os.WriteFile(dst.ToIO(), contents, 0644); err != nil {
		o.errs.Addf(token.NoPos, "write overlay file: %v", err)
	}
}

func (o *overlay) Data() []byte {
	replace := make(map[string]string, len(o.overlay))
	for k, v := range o.overlay {
		replace[k.ToIO()] = v.ToIO()
	}
	data, _ := json.Marshal(map[string]any{"Replace": replace})
	return data
}
