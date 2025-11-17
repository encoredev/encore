package pkginfo

import (
	"cmp"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
)

// File pkgparse implements parsing of packages.

// parseResult is the result from attempting to parse a package.
type parseResult struct {
	done    chan struct{} // closed when parsing is completed
	pkg     *Package
	ok      bool
	bailout bool
}

// loadPkgSpec is the specification for how to load a package.
type loadPkgSpec struct {
	// cause is the source position that caused the load.
	// It's used to generate useful error messages.
	cause token.Pos

	// path is the package path.
	path paths.Pkg

	// dir is the directory containing the package.
	dir paths.FS
}

// doParsePkg parses a single package in the given directory.
// It returns (nil, false) if the directory contains no Go files.
func (l *Loader) doParsePkg(s loadPkgSpec) (pkg *Package, ok bool) {
	l.c.Errs.BailoutOnErrors(func() {
		astPkgs, files := l.parseAST(s)
		pkg = l.processPkg(s, astPkgs, files)
	})
	return pkg, pkg != nil
}

// processPkg combines the results of parsing a package into a single *Package.
func (l *Loader) processPkg(s loadPkgSpec, pkgs []*ast.Package, files []*File) *Package {
	if n := len(pkgs); n > 1 {
		// Make sure the extra packages are just "_test" packages.
		// Pull out the package names.

		slices.SortFunc(pkgs, func(a, b *ast.Package) int {
			return cmp.Compare(a.Name, b.Name)
		})
		pkgNames := fns.Map(pkgs, func(pkg *ast.Package) string { return pkg.Name })
		if n == 2 && pkgNames[1] == pkgNames[0]+"_test" {
			// We're good
		} else {
			names := strings.Join(pkgNames[:n-1], ", ") + " and " + pkgNames[n-1]
			l.c.Errs.Addf(s.cause, "%s", fmt.Sprintf("found multiple package names in package %s: %s", s.path, names))
		}
	} else if n == 0 {
		// No Go files; ignore directory
		return nil
	}

	p := pkgs[0]
	pkg := &Package{
		AST:        p,
		Name:       p.Name,
		ImportPath: s.path,
		FSPath:     s.dir,
		Files:      files,
		Imports:    make(map[paths.Pkg]ast.Node),
	}

	for _, f := range files {
		f.Pkg = pkg
		// Fill in imports.
		for importPath, pointer := range f.Imports {
			pkg.Imports[importPath] = pointer
		}

		// Fill in package docs.
		if pkg.Doc == "" && !f.TestFile && f.initialAST.Doc != nil {
			pkg.Doc = f.initialAST.Doc.Text()
		}
	}

	return pkg
}

// parseAST is like go/parser.ParseDir but it constructs *File objects instead.
func (l *Loader) parseAST(s loadPkgSpec) ([]*ast.Package, []*File) {
	dir := s.dir.ToIO()
	entries, err := l.c.ReadDir(dir)
	if err != nil {
		l.c.Errs.Addf(s.cause, "parse package %q: %v", s.path, err)
		return nil, nil
	}
	shouldParseFile := func(info fs.DirEntry) bool {
		name := info.Name()
		switch {
		// Don't parse encore.gen.go files, since they're not intended to be checked in.
		// We've had several issues where things work locally but not in CI/CD because
		// the encore.gen.go file was parsed for local development which papered over issues.
		case strings.Contains(name, "encore.gen.go"):
			return false
		case !l.c.ParseTests && strings.HasSuffix(name, "_test.go"):
			return false
		case !strings.HasSuffix(name, ".go"):
			return false
		default:
			return true
		}
	}

	type fileInfo struct {
		path     paths.FS
		ioPath   string
		baseName string
	}

	infos := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && shouldParseFile(e) {
			baseName := e.Name()
			ioPath := filepath.Join(dir, baseName)
			path := s.dir.Join(baseName)
			infos = append(infos, fileInfo{path: path, ioPath: ioPath, baseName: baseName})
		}
	}

	var pkgs []*ast.Package
	var files []*File
	seenPkgs := make(map[string]*ast.Package) // package name -> pkg

	for _, d := range infos {
		// Check if this file should be part of the build
		matched, err := l.buildCtx.MatchFile(dir, d.baseName)
		if err != nil {
			l.c.Errs.Add(errMatchingFile.InFile(d.ioPath).Wrapping(err))
			continue
		} else if !matched {
			continue
		} else if strings.EqualFold(d.baseName, "encore.gen.go") {
			continue
		}

		reader, err := l.c.OpenFile(d.ioPath)
		if err != nil {
			l.c.Errs.Add(errReadingFile.InFile(d.ioPath).Wrapping(err))
			continue
		}

		// Parse the package and imports only so code can consult that.
		// We parse the full AST on-demand later.
		mode := goparser.ParseComments | goparser.ImportsOnly
		astFile, err := goparser.ParseFile(l.c.FS, d.ioPath, reader, mode)
		_ = reader.Close()
		if err != nil {
			l.c.Errs.AddStd(err)
			continue
		}

		pkgName := astFile.Name.Name
		pkg, found := seenPkgs[pkgName]
		if !found {
			pkg = &ast.Package{
				Name:  pkgName,
				Files: make(map[string]*ast.File),
			}
			seenPkgs[pkgName] = pkg
			pkgs = append(pkgs, pkg)
		}

		pkg.Files[d.ioPath] = astFile

		isTestFile := strings.HasSuffix(d.baseName, "_test.go") || strings.HasSuffix(pkgName, "_test")
		files = append(files, &File{
			l:        l,
			Name:     d.baseName,
			FSPath:   d.path,
			Pkg:      nil, // will be set later
			Imports:  getFileImports(astFile),
			TestFile: isTestFile,

			initialAST: astFile,
		})
	}

	return pkgs, files
}

func getFileImports(f *ast.File) map[paths.Pkg]ast.Node {
	imports := make(map[paths.Pkg]ast.Node)
	for _, s := range f.Imports {
		if importPath, err := strconv.Unquote(s.Path.Value); err == nil {
			if p, ok := paths.PkgPath(importPath); ok {
				imports[p] = s
			}
		}
	}
	return imports
}
