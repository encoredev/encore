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

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
)

type Config struct {
	// Build controls how to compile the binary.
	Build parsectx.BuildInfo

	// Codegen describes the code generation to apply.
	Codegen *Codegen

	// MainModuleDir is the directory of the main module.
	MainModuleDir paths.FS

	// MainPkg is the main package to build.
	MainPkg *pkginfo.Package

	// EncoreRuntimePath specifies the path to the Encore runtime.
	EncoreRuntimePath paths.FS

	// EncoreGoRoot, if set specifies the path to the Encore GOROOT.
	EncoreGoRoot option.Option[paths.FS]

	// KeepOutput keeps the temporary build directory from being deleted in the case of failure.
	KeepOutput bool
}

// Codegen describes the code generation changes to apply to the build.
type Codegen struct {
	Rewrites  map[*pkginfo.File][]byte
	Additions map[*pkginfo.Package][]GeneratedFile
}

type GeneratedFile struct {
	Name     string // The base name of the file
	Contents []byte
}

type Result struct {
	Dir string
	Exe string
}

func Build(errs *perr.List, cfg *Config) *Result {
	b := &builder{
		cfg:  cfg,
		errs: errs,
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

	// mainMod is the main module's parsed go.mod file.
	mainMod *modfile.File
}

func (b *builder) Build() *Result {
	workdir, err := os.MkdirTemp("", "encore-build")
	if err != nil {
		b.errs.AddStd(err)
	}
	b.workdir = paths.RootedFSPath(workdir, workdir)

	defer func() {
		// If we have a bailout or any errors, delete the workdir.
		if _, ok := perr.CatchBailout(recover()); ok || b.errs.Len() > 0 {
			if !b.cfg.KeepOutput && workdir != "" {
				_ = os.RemoveAll(workdir)
			}
		}
	}()

	res := &Result{
		Dir: workdir,
		Exe: filepath.Join(workdir, binaryName+b.exe()),
	}

	for _, fn := range []func(){
		b.writeModFile,
		b.writeSumFile,
		b.applyRewrites,
		b.buildMain,
	} {
		n := b.errs.Len()
		fn()
		// Abort early if we encountered any errors.
		if b.errs.Len() > n {
			break
		}
	}
	return res
}

func (b *builder) writeModFile() {
	newPath := b.cfg.EncoreRuntimePath.ToIO()
	oldPath := "encore.dev"

	// Make sure there's a dependency on encore.dev so it can be replaced.
	if err := b.mainMod.AddRequire("encore.dev", "v0.0.0"); err != nil {
		b.errs.Addf(token.NoPos, "unable to add 'require encore.dev' directive to go.mod: %v", err)
		return
	}
	if err := b.mainMod.AddReplace(oldPath, "", newPath, ""); err != nil {
		b.errs.Addf(token.NoPos, "unable to add 'replace encore.dev' directive to go.mod: %v", err)
		return
	}

	// We require Go 1.18+ now that we use generics in code gen.
	if !isGo118Plus(b.mainMod) {
		_ = b.mainMod.AddGoStmt("1.18")
	}
	b.mainMod.Cleanup()

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
	mergeModfiles(b.mainMod, runtimeModfile)

	modBytes := modfile.Format(b.mainMod.Syntax)
	b.o.Add(b.cfg.MainModuleDir.Join("go.mod"), "go.mod", modBytes)
}

func (b *builder) writeSumFile() {
	appSum, err := os.ReadFile(b.cfg.MainModuleDir.Join("go.sum").ToIO())
	if err != nil && !os.IsNotExist(err) {
		b.errs.Addf(token.NoPos, "unable to parse go.sum: %v", err)
		return
	}
	runtimeSum, err := os.ReadFile(b.cfg.EncoreRuntimePath.Join("go.sum").ToIO())
	if err != nil {
		b.errs.Addf(token.NoPos, "unable to parse encore runtime's go.sum: %v", err)
		return
	}
	if !bytes.HasSuffix(appSum, []byte{'\n'}) {
		appSum = append(appSum, '\n')
	}
	data := append(appSum, runtimeSum...)
	b.o.Add(b.cfg.MainModuleDir.Join("go.sum"), "go.sum", data)
}

func (b *builder) applyRewrites() {
	// Add the codegen rewrites to the overlay.
	for file, data := range b.cfg.Codegen.Rewrites {
		b.o.Add(file.FSPath, file.Pkg.Name+"__"+file.Name, data)
	}

	// Add the codegen additions to the overlay.
	for pkg, additions := range b.cfg.Codegen.Additions {
		for _, file := range additions {
			b.o.Add(pkg.FSPath.Join(file.Name), pkg.Name+"__"+file.Name, file.Contents)
		}
	}

}

func (b *builder) buildMain() {
	overlayData := b.o.Data()
	overlayPath := b.workdir.Join("overlay.json").ToIO()
	if err := os.WriteFile(overlayPath, overlayData, 0644); err != nil {
		b.errs.Addf(token.NoPos, "unable to write overlay file: %v", err)
		return
	}

	tags := append([]string{"encore", "encore_internal", "encore_app"}, b.cfg.Build.BuildTags...)
	args := []string{
		"build",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + overlayPath,
		"-mod=mod",
		"-o=" + b.workdir.Join(binaryName+b.exe()).ToIO(),
	}

	if b.cfg.Build.StaticLink {
		var ldflags string

		// Enable external linking if we use cgo.
		if b.cfg.Build.CgoEnabled {
			ldflags = "-linkmode external "
		}

		ldflags += `-extldflags "-static"`
		args = append(args, "-ldflags", ldflags)
	}

	args = append(args, b.cfg.MainPkg.ImportPath.String())

	var (
		cmd *exec.Cmd
		env []string
	)

	if b.cfg.EncoreGoRoot.IsPresent() {
		goroot := b.cfg.EncoreGoRoot.MustGet()
		cmd = exec.Command(goroot.Join("bin", "go"+b.exe()).ToIO(), args...)
		env = append(env, "GOROOT="+goroot.ToIO())
	} else {
		// Use system go if no GOROOT is provided
		cmd = exec.Command("go", args...)
	}

	if goos := b.cfg.Build.GOOS; goos != "" {
		env = append(env, "GOOS="+goos)
	}
	if goarch := b.cfg.Build.GOARCH; goarch != "" {
		env = append(env, "GOARCH="+goarch)
	}
	if !b.cfg.Build.CgoEnabled {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = b.cfg.MainModuleDir.ToIO()
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
	goos := b.cfg.Build.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "windows" {
		return ".exe"
	}
	return ""
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
