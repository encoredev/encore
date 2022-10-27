package compiler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"encr.dev/compiler/internal/codegen"
	"encr.dev/compiler/internal/cuegen"
	"encr.dev/internal/optracker"
	"encr.dev/parser"
	"encr.dev/parser/est"
	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/errlist"
)

type Config struct {
	// Revision specifies the app version to encode
	// into the app metadata.
	Revision string

	// This boolean returns if there are uncommitted changes
	UncommittedChanges bool

	// WorkingDir is the path relative to the app root from which the user
	// is running the build. It is used to resolve relative filenames.
	// If empty it defaults to "." which resolves to the app root.
	WorkingDir string

	// GOOS sets the GOOS to build for, if nonempty.
	GOOS string

	// GOARCH sets the GOARCH to build for, if nonempty.
	GOARCH string

	// CgoEnabled decides whether to build with cgo enabled.
	CgoEnabled bool

	// Debug specifies whether to compile in debug mode.
	Debug bool

	// BuildTags are additional build tags to specify when building.
	BuildTags []string

	// StaticLink enables static linking of C libraries.
	StaticLink bool

	// EncoreCompilerVersion is the version of the compiler used to build the app
	// it is used purely for information purposes within the healthz response.
	EncoreCompilerVersion string

	// EncoreRuntimePath if set, causes builds to introduce a temporary replace directive
	// that replaces the module path to the "encore.dev" module.
	// This lets us replace the implementation for building.
	EncoreRuntimePath string

	// EncoreGoRoot is the path to the Encore GOROOT.
	EncoreGoRoot string

	// Test is the specific settings for running tests.
	Test *TestConfig

	// The meta config we pass to CUE when computing the runtime configuration for the services within this
	// application
	Meta *cueutil.Meta

	// If Parse is set, the build will skip parsing the app again
	// and use the information provided.
	Parse *parser.Result

	// KeepOutput keeps the temporary build directory from being deleted in the case of failure.
	KeepOutput bool

	// OpTracker is an option tracker to output the progress to the UI
	OpTracker *optracker.OpTracker
}

// Validate validates the config.
func (cfg *Config) Validate() error {
	if cfg.EncoreRuntimePath == "" {
		return errors.New("empty EncoreRuntimePath")
	} else if cfg.EncoreGoRoot == "" {
		return errors.New("empty EncoreGoRoot")
	}
	return nil
}

// Result is the combined results of a build.
type Result struct {
	Dir         string            // absolute path to build temp dir
	Exe         string            // absolute path to the build executable
	Parse       *parser.Result    // set only if build succeeded
	ConfigFiles fs.FS             // all found configuration files within the application source
	Configs     map[string]string // each services runtime config as defined
}

// Build builds the application.
//
// On success, it is the caller's responsibility to delete the temp dir
// returned in Result.Dir.
func Build(appRoot string, cfg *Config) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	} else if appRoot, err = filepath.Abs(appRoot); err != nil {
		return nil, err
	}

	b := &builder{
		cfg:      cfg,
		appRoot:  appRoot,
		configs:  make(map[string]string),
		lastOpID: optracker.NoOperationID,
	}
	return b.Build()
}

type builder struct {
	// inputs
	cfg        *Config
	appRoot    string
	forTesting bool

	workdir string
	modfile *modfile.File
	overlay map[string]string
	codegen *codegen.Builder
	cuegen  *cuegen.Generator

	res         *parser.Result
	configFiles fs.FS
	configs     map[string]string // Configs by service name -> config JSON

	appCheckOpID optracker.OperationID
	codegenOpID  optracker.OperationID
	lastOpID     optracker.OperationID
}

func (b *builder) Build() (res *Result, err error) {
	defer func() {
		if e := recover(); e != nil {
			if bailoutErr, ok := e.(bailout); ok {
				err = bailoutErr.err
			} else {
				if err == nil {
					err = srcerrors.UnhandledPanic(e)
				}
			}

			if list := errlist.Convert(err); list != nil {
				list.MakeRelative(b.appRoot, "")
				err = list
			}

			b.cfg.OpTracker.Fail(b.lastOpID, err)
		}
	}()

	b.workdir, err = ioutil.TempDir("", "encore-build")
	if err != nil {
		return nil, err
	}
	res = &Result{
		Dir: b.workdir,
		Exe: filepath.Join(b.workdir, binaryName+b.exe()),
	}
	defer func() {
		if err != nil && !b.cfg.KeepOutput {
			os.RemoveAll(b.workdir)
		}
	}()

	for _, fn := range []func() error{
		b.parseApp,
		b.startAppCheck,
		b.pickupConfigFiles,
		b.checkApp,
		b.endAppCheck,
		b.startCodeGenTracker,
		b.writeModFile,
		b.writeSumFile,
		b.writePackages,
		b.writeHandlers,
		b.writeMainPkg,
		b.writeEtypePkg,
		b.writeConfigUnmarshallers,
		b.endCodeGenTracker,
		b.buildMain,
	} {
		if err := fn(); err != nil {
			b.cfg.OpTracker.Fail(b.lastOpID, err)
			return res, err
		}
	}

	res.Parse = b.res
	res.ConfigFiles = b.configFiles
	res.Configs = b.configs
	return res, nil
}

func (b *builder) startAppCheck() error {
	b.appCheckOpID = b.cfg.OpTracker.Add("Verifying application configuration", time.Now())
	b.lastOpID = b.appCheckOpID
	return nil
}

func (b *builder) endAppCheck() error {
	b.cfg.OpTracker.Done(b.appCheckOpID, 50*time.Millisecond)
	return nil
}

func (b *builder) startCodeGenTracker() error {
	b.codegenOpID = b.cfg.OpTracker.Add("Generating boilerplate code", time.Now())
	b.lastOpID = b.codegenOpID
	return nil
}

func (b *builder) endCodeGenTracker() error {
	b.cfg.OpTracker.Done(b.codegenOpID, 450*time.Millisecond)
	return nil
}

// parseApp parses the app situated at appRoot.
func (b *builder) parseApp() error {
	modPath := filepath.Join(b.appRoot, "go.mod")
	modData, err := ioutil.ReadFile(modPath)
	if err != nil {
		return err
	}
	b.modfile, err = modfile.Parse(modPath, modData, nil)
	if err != nil {
		return err
	}

	if pc := b.cfg.Parse; pc != nil {
		b.res = pc
		b.codegen = codegen.NewBuilder(b.res, b.forTesting)
		b.cuegen = cuegen.NewGenerator(b.res)
		return nil
	}

	cfg := &parser.Config{
		AppRoot:                  b.appRoot,
		AppRevision:              b.cfg.Revision,
		AppHasUncommittedChanges: b.cfg.UncommittedChanges,
		ModulePath:               b.modfile.Module.Mod.Path,
		WorkingDir:               b.cfg.WorkingDir,
		ParseTests:               b.forTesting,
	}
	b.res, err = parser.Parse(cfg)

	if err == nil {
		b.codegen = codegen.NewBuilder(b.res, b.forTesting)
		b.cuegen = cuegen.NewGenerator(b.res)
	}

	return err
}

// checkApp checks the parsed app against the metadata.
func (b *builder) checkApp() error {
	dbs := make(map[string]bool)
	for _, svc := range b.res.Meta.Svcs {
		if len(svc.Migrations) > 0 {
			dbs[svc.Name] = true
		}
	}

	serviceConfigsChecked := make(map[*est.Service]struct{})

	for _, pkg := range b.res.App.Packages {
		for _, res := range pkg.Resources {
			switch res := res.(type) {
			case *est.SQLDB:
				if !dbs[res.DBName] {
					panic(bailout{srcerrors.DatabaseNotFound(b.res.FileSet, res.Ident(), res.DBName)})
				}

			case *est.Config:
				if _, found := serviceConfigsChecked[res.Svc]; !found {
					serviceConfigsChecked[res.Svc] = struct{}{}

					if err := b.computeConfigForService(res.Svc); err != nil {
						if list := errlist.Convert(err); list != nil {
							err = list

							errinsrc.AddHintFromGo(err, b.res.FileSet, res.FuncCall, "config loaded from here")
						} else {
							err = srcerrors.UnknownErrorCompilingConfig(
								b.res.FileSet, res.FuncCall, err,
							)
						}
						panic(bailout{err})
					}
				}
			}
		}
	}

	return nil
}

func (b *builder) writeModFile() error {
	newPath := b.cfg.EncoreRuntimePath
	oldPath := "encore.dev"
	if err := b.modfile.AddRequire("encore.dev", "v0.0.0"); err != nil {
		return fmt.Errorf("could not add require encore.dev path: %v", err)
	}
	if err := b.modfile.AddReplace(oldPath, "", newPath, ""); err != nil {
		return fmt.Errorf("could not replace encore.dev path: %v", err)
	}
	// We require Go 1.18+ now that we use generics in code gen.
	if !isGo118Plus(b.modfile) {
		b.modfile.AddGoStmt("1.18")
	}

	b.modfile.Cleanup()

	runtimeModData, err := os.ReadFile(filepath.Join(newPath, "go.mod"))
	if err != nil {
		return err
	}
	runtimeModfile, err := modfile.Parse("encore-runtime/go.mod", runtimeModData, nil)
	if err != nil {
		return err
	}
	mergeModfiles(b.modfile, runtimeModfile)

	modBytes := modfile.Format(b.modfile.Syntax)
	dstGomod := filepath.Join(b.workdir, "go.mod")
	return ioutil.WriteFile(dstGomod, modBytes, 0644)
}

func (b *builder) writeSumFile() error {
	appSum, err := ioutil.ReadFile(filepath.Join(b.appRoot, "go.sum"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	runtimeSum, err := ioutil.ReadFile(filepath.Join(b.cfg.EncoreRuntimePath, "go.sum"))
	if err != nil {
		return err
	}
	if !bytes.HasSuffix(appSum, []byte{'\n'}) {
		appSum = append(appSum, '\n')
	}
	data := append(appSum, runtimeSum...)
	dstGosum := filepath.Join(b.workdir, "go.sum")
	return ioutil.WriteFile(dstGosum, data, 0644)
}

func (b *builder) writePackages() error {
	// Copy all the packages into the workdir
	for _, pkg := range b.res.App.Packages {
		targetDir := filepath.Join(b.workdir, filepath.FromSlash(pkg.RelPath))
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		} else if err := b.rewritePkg(pkg, targetDir); err != nil {
			return err
		}
	}

	for _, svc := range b.res.App.Services {
		if err := b.generateServiceSetup(svc); err != nil {
			return err
		}

		if err := b.generateCueFiles(svc); err != nil {
			return err
		}
	}

	return nil
}

func (b *builder) buildMain() error {
	compileAppOpId := b.cfg.OpTracker.Add("Compiling application source code", time.Now())
	b.lastOpID = compileAppOpId

	overlayData, _ := json.Marshal(map[string]interface{}{"Replace": b.overlay})
	overlayPath := filepath.Join(b.workdir, "overlay.json")
	if err := ioutil.WriteFile(overlayPath, overlayData, 0644); err != nil {
		return err
	}

	tags := append([]string{"encore", "encore_internal", "encore_app"}, b.cfg.BuildTags...)
	args := []string{
		"build",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + overlayPath,
		"-modfile=" + filepath.Join(b.workdir, "go.mod"),
		"-mod=mod",
		"-o=" + filepath.Join(b.workdir, "out"+b.exe()),
	}
	if b.cfg.StaticLink {
		var ldflags string
	
		// Enable external linking if we use cgo.
		if b.cfg.CgoEnabled {
			ldflags = "-linkmode external "
		}

		ldflags += `-extldflags "-static"`
		args = append(args, "-ldflags", ldflags)
	}

	args = append(args, fmt.Sprintf("./%s/%s", encorePkgDir, mainPkgName))
	cmd := exec.Command(filepath.Join(b.cfg.EncoreGoRoot, "bin", "go"+b.exe()), args...)
	env := []string{
		"GO111MODULE=on",
		"GOROOT=" + b.cfg.EncoreGoRoot,
	}
	if goos := b.cfg.GOOS; goos != "" {
		env = append(env, "GOOS="+goos)
	}
	if goarch := b.cfg.GOARCH; goarch != "" {
		env = append(env, "GOARCH="+goarch)
	}
	if !b.cfg.CgoEnabled {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = b.appRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) == 0 {
			out = []byte(err.Error())
		}
		out = convertCompileErrors(out, b.workdir, b.appRoot, b.cfg.WorkingDir)
		return &Error{Output: out}
	}

	b.cfg.OpTracker.Done(compileAppOpId, 300*time.Millisecond)

	return nil
}

func (b *builder) addOverlay(src, dst string) {
	if b.overlay == nil {
		b.overlay = make(map[string]string)
	}
	b.overlay[src] = dst
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

type Error struct {
	Output []byte
}

func (err *Error) Error() string {
	return string(err.Output)
}

type bailout struct {
	err error
}

const binaryName = "out"

func (b *builder) exe() string {
	goos := b.cfg.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

// convertCompileErrors goes through the errors and converts basic compiler errors into
// ErrInSrc errors, which are more useful for the user.
func convertCompileErrors(out []byte, workdir, appRoot, relwd string) []byte {
	wdroot := filepath.Join(appRoot, relwd)
	lines := bytes.Split(out, []byte{'\n'})
	prefix := append([]byte(workdir), '/')
	modified := false

	errList := errlist.New(nil)

	output := make([][]byte, 0)

	for _, line := range lines {
		if !bytes.HasPrefix(line, prefix) {
			output = append(output, line)
			continue
		}
		idx := bytes.IndexByte(line, ':')
		if idx == -1 || idx < len(prefix) {
			output = append(output, line)
			continue
		}

		filename := line[:idx]
		appPath := filepath.Join(appRoot, string(filename[len(prefix):]))
		if _, err := filepath.Rel(wdroot, appPath); err == nil {
			parts := strings.SplitN(string(line), ":", 4)
			if len(parts) != 4 {
				output = append(output, line)
				continue
			}

			lineNumber, err := strconv.Atoi(parts[1])
			if err != nil {
				output = append(output, line)
				continue
			}

			colNumber, err := strconv.Atoi(parts[2])
			if err != nil {
				output = append(output, line)
				continue
			}

			modified = true
			errList.Report(srcerrors.GenericGoCompilerError(changeToAppRootFile(parts[0], workdir, appRoot), lineNumber, colNumber, parts[3]))
		} else {
			output = append(output, line)
		}
	}

	if !modified {
		return out
	}

	// Append the err list for both the workDir and the appRoot
	// as files might be coming from either of them
	errList.MakeRelative(workdir, relwd)
	errList.MakeRelative(appRoot, relwd)
	output = append(output, []byte(errList.Error()))

	return bytes.Join(output, []byte{'\n'})
}

// changeToAppRootFile will return the compiledFile path inside the appRoot directory
// if that file exists within the app root. Otherwise it will return the original
// compiledFile path.
//
// This means when we display compiler errors to the user, we will show them their
// original rewritten code, where line numbers and column numbers will align with
// their own code.
//
// However for generated files which don't exist in their own folders, we will
// still be able to render the source causing the issue
func changeToAppRootFile(compiledFile string, workDirectory, appRoot string) string {
	if strings.HasPrefix(compiledFile, workDirectory) {
		fileInOriginalSrc := strings.TrimPrefix(compiledFile, workDirectory)
		fileInOriginalSrc = path.Join(appRoot, fileInOriginalSrc)

		if _, err := os.Stat(fileInOriginalSrc); err == nil {
			return fileInOriginalSrc
		}
	}

	return compiledFile
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
