package build

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"hash"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"

	"encr.dev/internal/version"
	"encr.dev/pkg/xos"
)

// tidyFingerprintName is the file in the workdir that records the inputs
// and outputs of the last successful 'go mod tidy'.
const tidyFingerprintName = "tidy-fingerprint.json"

// tidyFingerprintVersion is hashed first so a change to what the fingerprint
// covers never trusts a file written by an older Encore.
const tidyFingerprintVersion = "encore-tidy-fingerprint-v1"

type tidyFingerprint struct {
	Inputs  string `json:"inputs"`
	Outputs string `json:"outputs"`
}

// itemHasher hashes a sequence of length-prefixed byte strings,
// so that joining items is unambiguous.
type itemHasher struct {
	buf [8]byte
	h   hash.Hash
}

func (h *itemHasher) add(data []byte) {
	binary.BigEndian.PutUint64(h.buf[:], uint64(len(data)))
	_, _ = h.h.Write(h.buf[:])
	_, _ = h.h.Write(data)
}

func (h *itemHasher) addString(s string) {
	h.add([]byte(s))
}

// addList adds a count followed by each item, so adjacent lists cannot shift into each other.
func (h *itemHasher) addList(items []string) {
	h.addString(strconv.Itoa(len(items)))
	for _, item := range items {
		h.addString(item)
	}
}

// addFile adds the file's contents, distinguishing a missing file from an empty one.
func (h *itemHasher) addFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		h.addString("missing")
		return
	}
	h.addString("present")
	h.add(data)
}

// tidyInputsHash hashes everything 'go mod tidy' reads when run by writeModFile.
// modData is the pre-tidy go.mod written to the workdir.
// It reports false when tidy must always run: a temporary workdir, a local
// directory replace, or an input that cannot be read.
//
// Known limits, each of which can leave a stale result: imports inside symlinked
// package directories are not seen, and neither are imports in the runtime's
// own Go sources (only its go.mod and go.sum are hashed, plus the Encore
// version). Checkouts of one app share a workdir, so a build racing another
// one can record a fingerprint over the other's tidy output. A failed build
// or test run removes the fingerprint, so the next one runs tidy again.
// Tidy always runs when the workdir has no go.sum.
func (b *builder) tidyInputsHash(mainMod *modfile.File, modData []byte) (string, bool) {
	// Temporary workdirs never have a fingerprint, so don't spend time computing one.
	if _, ok := b.cfg.Ctx.AppID.Get(); !ok {
		return "", false
	}

	// A replace to a local directory pulls that module's imports into tidy, which we don't track.
	for _, r := range mainMod.Replace {
		isLocalDir := r.New.Version == "" && r.Old.Path != "encore.dev"
		if isLocalDir {
			return "", false
		}
	}

	sum := sha256.New()
	h := &itemHasher{h: sum}
	h.addString(tidyFingerprintVersion)
	h.addString(version.Version)
	h.add(modData)

	// The Go toolchain, using the size and mtime of the go binary as a cheap stand-in for its version.
	build := b.cfg.Ctx.Build
	goroot := build.GOROOT.ToIO()
	goBin, err := os.Stat(filepath.Join(goroot, "bin", "go"+b.exe()))
	if err != nil {
		return "", false
	}
	h.addString(goroot)
	h.addString(strconv.FormatInt(goBin.Size(), 10))
	h.addString(strconv.FormatInt(goBin.ModTime().UnixNano(), 10))
	h.addFile(filepath.Join(goroot, "go.env"))

	// The environment, resolved the way exec resolves duplicates (last one wins).
	// The variables Encore adds for the tidy command derive from the build info hashed below.
	env := make(map[string]string)
	for _, kv := range slices.Concat(os.Environ(), b.cfg.Env) {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	var goEnv []string
	for k, v := range env {
		if strings.HasPrefix(k, "GO") || strings.HasPrefix(k, "CGO_") {
			goEnv = append(goEnv, k+"="+v)
		}
	}
	slices.Sort(goEnv)
	h.addList(goEnv)
	h.addList([]string{build.GOOS, build.GOARCH, strconv.FormatBool(build.CgoEnabled)})

	// Settings from 'go env -w', which the go command reads from the GOENV file.
	goenvFile := env["GOENV"]
	if goenvFile == "" {
		if cfgDir, err := os.UserConfigDir(); err == nil {
			goenvFile = filepath.Join(cfgDir, "go", "env")
		}
	}
	if goenvFile != "" && goenvFile != "off" {
		h.addFile(goenvFile)
	}

	// A go.work file in the app root or above it changes how tidy behaves.
	mainDir := b.cfg.Ctx.MainModuleDir.ToIO()
	h.addString(mainDir)
	for dir := mainDir; ; dir = filepath.Dir(dir) {
		workFile := filepath.Join(dir, "go.work")
		if _, err := os.Stat(workFile); err == nil {
			h.addString(workFile)
			h.addFile(workFile)
			break
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}

	// The Encore runtime module the pre-tidy go.mod replaces encore.dev with.
	runtimeDir := build.EncoreRuntime.ToIO()
	h.addFile(filepath.Join(runtimeDir, "go.mod"))
	h.addFile(filepath.Join(runtimeDir, "go.sum"))

	// The Go files tidy sees: overlays, plus every file on disk no overlay replaces.
	overlaid := make(map[string]bool, len(b.cfg.Overlays))
	var goSources []goSource
	var otherOverlays []string
	for _, o := range b.cfg.Overlays {
		src := o.Source.ToIO()
		overlaid[src] = true

		contents := o.Contents
		if o.Dest != "" {
			if contents, err = os.ReadFile(o.Dest.ToIO()); err != nil {
				return "", false
			}
		}
		if strings.HasSuffix(src, ".go") {
			goSources = append(goSources, goSource{path: src, contents: contents})
			continue
		}
		otherOverlays = append(otherOverlays, src, string(contents))
	}
	h.addList(otherOverlays)

	pkgDirs := make(map[string]bool)
	var nestedModules []string
	for _, o := range b.cfg.Overlays {
		if src := o.Source.ToIO(); strings.HasSuffix(src, ".go") {
			pkgDirs[filepath.Dir(src)] = true
		}
	}
	err = filepath.WalkDir(mainDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == mainDir {
				return nil
			}
			// Skip the directories the go command skips when matching packages.
			name := d.Name()
			isIgnored := strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata"
			if isIgnored {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				nestedModules = append(nestedModules, path)
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || overlaid[path] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		goSources = append(goSources, goSource{path: path, contents: data})
		pkgDirs[filepath.Dir(path)] = true
		return nil
	})
	if err != nil {
		return "", false
	}
	h.addList(nestedModules)
	h.addList(slices.Sorted(maps.Keys(pkgDirs)))

	// Tidy skips some files by name or build constraint, so renaming a file or
	// editing its constraint can change the result without changing any import.
	// The imports themselves ignore build constraints, since tidy considers every build tag.
	imports := make(map[string]bool)
	var fileKeys []string
	fset := token.NewFileSet()
	for _, src := range goSources {
		f, err := parser.ParseFile(fset, "", src.contents, parser.ParseComments|parser.ImportsOnly)
		if err != nil {
			return "", false
		}
		fileKeys = append(fileKeys, src.path+"\x00"+strings.Join(buildConstraints(f), "\n"))
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return "", false
			}
			imports[path] = true
		}
	}
	slices.Sort(fileKeys)
	h.addList(fileKeys)
	h.addList(slices.Sorted(maps.Keys(imports)))

	return hex.EncodeToString(sum.Sum(nil)), true
}

// goSource is a Go file that go mod tidy would read.
type goSource struct {
	path     string
	contents []byte
}

// buildConstraints returns the build constraint lines above the package clause.
func buildConstraints(f *ast.File) []string {
	var lines []string
	for _, group := range f.Comments {
		if group.Pos() > f.Package {
			break
		}
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				lines = append(lines, c.Text)
			}
		}
	}
	return lines
}

// tidyOutputsHash hashes the workdir's go.mod then go.sum, as tidy last wrote them.
// It reports false if either is missing.
func (b *builder) tidyOutputsHash() (string, bool) {
	sum := sha256.New()
	h := &itemHasher{h: sum}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(b.workdir.Join(name).ToIO())
		if err != nil {
			return "", false
		}
		h.add(data)
	}
	return hex.EncodeToString(sum.Sum(nil)), true
}

// tidyUpToDate reports whether the saved fingerprint records the given inputs
// and the go.mod and go.sum currently in the workdir.
func (b *builder) tidyUpToDate(inputs string) bool {
	data, err := os.ReadFile(b.workdir.Join(tidyFingerprintName).ToIO())
	if err != nil {
		return false
	}
	var saved tidyFingerprint
	if err := json.Unmarshal(data, &saved); err != nil {
		return false
	}
	if saved.Inputs != inputs {
		return false
	}
	outputs, ok := b.tidyOutputsHash()
	if !ok {
		return false
	}
	return saved.Outputs == outputs
}

func (b *builder) removeTidyFingerprint() {
	_ = os.Remove(b.workdir.Join(tidyFingerprintName).ToIO())
}

// writeTidyFingerprint records inputs with the workdir's current go.mod and go.sum.
// A failed write is ignored: the next build runs tidy.
func (b *builder) writeTidyFingerprint(inputs string) {
	outputs, ok := b.tidyOutputsHash()
	if !ok {
		return
	}
	data, err := json.Marshal(tidyFingerprint{
		Inputs:  inputs,
		Outputs: outputs,
	})
	if err != nil {
		return
	}

	// xos.WriteFile writes to a temporary name and renames, so a crash never leaves a partial file.
	_ = xos.WriteFile(b.workdir.Join(tidyFingerprintName).ToIO(), data, 0o644)
}
