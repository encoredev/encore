package paths

import (
	"path"
	"path/filepath"
	"strings"

	"encr.dev/pkg/fns"
)

// RootedFSPath returns a new FS path.
// It should typically not be used except for at parser initialization.
// Use FS.Join, FS.New, or FS.Resolve instead to preserve the working dir.
func RootedFSPath(wd, p string) FS {
	if wd == "" {
		panic("paths: empty wd")
	} else if !filepath.IsAbs(wd) {
		panic("paths: wd is relative")
	}

	if filepath.IsAbs(p) {
		return FS(filepath.Clean(p))
	} else {
		return FS(filepath.Join(wd, p))
	}
}

// FS represents a filesystem path.
//
// It is an absolute path, and is always in the OS-specific format.
type FS string

// ToIO returns the path for use in IO operations.
func (fs FS) ToIO() string {
	fs.checkValid()
	return string(fs)
}

// ToDisplay returns the path in a form suitable for displaying
// to the user.
func (fs FS) ToDisplay() string {
	fs.checkValid()
	return string(fs)
}

// Resolve returns a new FS path to the given path.
// If p is absolute it returns p directly,
// otherwise it returns the path joined with the current path.
func (fs FS) Resolve(p string) FS {
	fs.checkValid()
	if filepath.IsAbs(p) {
		return FS(filepath.Clean(p))
	}
	return FS(filepath.Join(string(fs), p))
}

// Join joins the path with the given elems, according to filepath.Join.
func (fs FS) Join(elem ...string) FS {
	fs.checkValid()
	parts := append([]string{string(fs)}, elem...)
	return FS(filepath.Join(parts...))
}

// Base returns the filepath.Base of the path.
func (fs FS) Base() string {
	fs.checkValid()
	return filepath.Base(string(fs))
}

// Dir returns the filepath.Dir of the path.
func (fs FS) Dir() FS {
	fs.checkValid()
	return FS(filepath.Dir(string(fs)))
}

// HasPrefix reports whether fs is a descendant of other
// or is equal to other. (i.e. it is the given path or a subdirectory of it)
func (fs FS) HasPrefix(other FS) bool {
	fs.checkValid()
	other.checkValid()

	// Note: we use filepath.Rel instead of strings.HasPrefix with filepath.Abs
	// because that wouldn't work on case-insensitive filesystems.
	rel, err := filepath.Rel(string(other), string(fs))
	if err != nil {
		return false
	}

	return filepath.IsLocal(rel)
}

func (fs FS) checkValid() {
	if fs == "" {
		panic("empty FS path")
	}
}

// ValidPkgPath reports whether a given module path is valid.
func ValidPkgPath(p string) bool {
	return p != ""
}

// PkgPath returns a new Pkg path for p. If p is not a valid
// package path it reports "", false.
func PkgPath(p string) (Pkg, bool) {
	if !ValidPkgPath(p) {
		return "", false
	}
	return Pkg(p), true
}

func MustPkgPath(p string) Pkg {
	if !ValidPkgPath(p) {
		panic("invalid Package path")
	}
	return Pkg(p)
}

// Pkg represents a package path within a module.
// It is always slash-separated.
type Pkg string

// String returns the string representation of p.
func (p Pkg) String() string {
	return string(p)
}

// JoinSlash joins the path with the given elems, according to path.Join.
// The elems are expected to be slash-separated, not filesystem-separated.
// Use filesystem.ToSlash() to convert filesystem paths to slash-separated paths.
func (p Pkg) JoinSlash(elem ...RelSlash) Pkg {
	p.checkValid()
	strs := make([]string, len(elem)+1)
	strs[0] = string(p)
	copy(strs[1:], fns.Map(elem, RelSlash.String))
	return Pkg(path.Join(strs...)) // Join cleans the result
}

func (p Pkg) checkValid() {
	if p == "" {
		panic("invalid Pkg path")
	}
}

// LexicallyContains reports whether the given package path contains the package path p
// as a "sub-package".
func (p Pkg) LexicallyContains(other Pkg) bool {
	p.checkValid()
	if other == "" {
		return false
	}
	return p == other || strings.HasPrefix(string(other), string(p)+"/")
}

const stdModule = "std"

// Mod represents a module path.
// It is always slash-separated.
type Mod string

// ValidModPath reports whether a given module path is valid.
func ValidModPath(p string) bool {
	return p != ""
}

// MustModPath returns a new Mod path for p.
func MustModPath(p string) Mod {
	if !ValidModPath(p) {
		panic("invalid Module path")
	}
	return Mod(p)
}

// StdlibMod returns the Mod path representing the standard library.
func StdlibMod() Mod {
	return stdModule
}

// LexicallyContains reports whether the given module path contains the package path p.
// It only considers the lexical path and ignores whether there exists
// a nested module that contains p.
func (m Mod) LexicallyContains(p Pkg) bool {
	m.checkValid()
	if p == "" {
		return false
	}

	// From the spec:
	// A module that will never be fetched as a dependency of any other module may use
	// any valid package path for its module path, but must take care not to collide
	// with paths that may be used by the module's dependencies or the Go standard
	// library. The Go standard library uses package paths that do not contain a dot in
	// the first path element, and the `go` command does not attempt to resolve such
	// paths from network servers. The paths `example` and `test` are reserved for
	// users: they will not be used in the standard library and are suitable for use in
	// self-contained modules, such as those defined in tutorials or example code or
	// created and manipulated as part of a test.

	ms, ps := string(m), string(p)
	if m == stdModule {
		// Treat any dotless package path as being contained, as long as
		// it's not one of the reserved paths.
		if first, _, _ := strings.Cut(ps, "/"); strings.Contains(first, ".") {
			return false
		} else if first == "example" || first == "tests" {
			// Reserved; guaranteed not to be part of std
			return false
		}
		return true
	}

	// We can treat the module path as a package path for this purpose.
	return ms == ps || strings.HasPrefix(ps, ms+"/")
}

// RelativePathToPkg returns the relative path from the module to the package.
// If the package is not contained within the module it reports "", false.
func (m Mod) RelativePathToPkg(p Pkg) (relative RelSlash, ok bool) {
	m.checkValid()
	p.checkValid()
	if !m.LexicallyContains(p) {
		return "", false
	}

	// The module path is a prefix of the package path.
	// Remove the module path and the leading slash.
	if m == stdModule {
		return RelSlash(p), true
	}

	// Is the package path the same as the module path?
	if string(p) == string(m) {
		return ".", true
	}

	suffix, ok := strings.CutPrefix(string(p), string(m)+"/")
	if !ok {
		return "", false
	}
	return RelSlash(suffix), true
}

func (m Mod) checkValid() {
	if m == "" {
		panic("invalid Module path")
	}
}

// IsStdlib reports whether m represents the standard library.
func (m Mod) IsStdlib() bool {
	return m == stdModule
}

// RelSlash is a relative path that is always slash-separated.
type RelSlash string

// ToIO converts the slash-separated path to a filesystem path
// using filepath.FromSlash.
func (p RelSlash) ToIO() string {
	return filepath.FromSlash(string(p))
}

func (p RelSlash) String() string {
	return string(p)
}

// MainModuleRelSlash is like RelSlash, but it's always relative to the application's
// main module directory.
type MainModuleRelSlash string

// ToIO converts the slash-separated path to a filesystem path
// using filepath.FromSlash.
func (p MainModuleRelSlash) ToIO(mainModDir FS) string {
	return mainModDir.Join(filepath.FromSlash(string(p))).ToIO()
}

func (p MainModuleRelSlash) String() string {
	return string(p)
}
