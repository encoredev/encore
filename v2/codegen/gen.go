package codegen

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"

	"encr.dev/pkg/paths"
	"encr.dev/v2/app/legacymeta"
	"encr.dev/v2/codegen/apigen/typescrub"
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/codegen/rewrite"
	"encr.dev/v2/internals/overlay"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/pkginfo"
)

type Generator struct {
	*parsectx.Context

	Util             *genutil.Helper
	TraceNodes       *legacymeta.TraceNodes
	TypeScrubber     *typescrub.Computer
	rewrites         map[*pkginfo.File]*rewrite.Rewriter
	files            map[fileKey]*File
	addedAppInit     map[paths.Pkg]bool
	addedTestSupport map[paths.Pkg]bool
}

func New(c *parsectx.Context, traceNodes *legacymeta.TraceNodes) *Generator {
	return &Generator{
		Context:          c,
		Util:             genutil.NewHelper(c.Errs),
		TraceNodes:       traceNodes,
		TypeScrubber:     typescrub.NewComputer(c.Log),
		rewrites:         make(map[*pkginfo.File]*rewrite.Rewriter),
		files:            make(map[fileKey]*File),
		addedAppInit:     make(map[paths.Pkg]bool),
		addedTestSupport: make(map[paths.Pkg]bool),
	}
}

func (g *Generator) Rewrite(file *pkginfo.File) *rewrite.Rewriter {
	if r, ok := g.rewrites[file]; ok {
		return r
	}
	r := rewrite.New(file.Contents(), file.Token().Base())
	g.rewrites[file] = r
	return r
}

type fileKey struct {
	pkgPath  paths.Pkg
	baseName string
}

func (g *Generator) File(pkg *pkginfo.Package, shortName string) *File {
	baseName := "encore_internal__" + shortName + ".go"
	key := fileKey{pkg.ImportPath, baseName}
	if f, ok := g.files[key]; ok {
		return f
	}
	f := newFile(pkg, baseName, shortName)
	g.files[key] = f
	return f
}

func (g *Generator) InjectFile(pkgPath paths.Pkg, pkgName string, pkgDir paths.FS, baseName, shortName string) *File {
	key := fileKey{pkgPath, baseName}
	if f, ok := g.files[key]; ok {
		return f
	}
	f := newFileForPath(pkgPath, pkgName, pkgDir, baseName, shortName)
	g.files[key] = f
	return f
}

// Overlays returns the overlay files for the generated code.
//
// Rendering the files, which formats the generated source, runs on all cores.
// The result is sorted by source path so that overlay.Write, which suffixes
// clashing names in slice order, gives each file the same name every run.
func (g *Generator) Overlays() []overlay.File {
	type pendingFile struct {
		source   paths.FS
		file     *File // nil for a rewrite, whose contents are already computed
		contents []byte
		err      error
	}

	// Add every file's declarations before rendering any of them, since
	// Decl.Code() reaches shared state that is not safe to use concurrently.
	pending := make([]pendingFile, 0, len(g.files)+len(g.rewrites))
	for _, f := range g.files {
		f.AddDecls()
		pending = append(pending, pendingFile{
			source: f.dir.Join(f.name()),
			file:   f,
		})
	}

	// Jennifer panics on some invalid declarations. Callers recover those into build
	// errors, but a panic on a worker goroutine would bypass them.
	render := func(f *File) (contents []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			}
		}()
		var buf bytes.Buffer
		if err := f.Jen.Render(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Render the files, one buffer each, on all cores.
	work := make(chan int, len(pending))
	for i := range pending {
		work <- i
	}
	close(work)

	var wg sync.WaitGroup
	for range min(len(pending), runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				pending[i].contents, pending[i].err = render(pending[i].file)
			}
		}()
	}
	wg.Wait()

	// Rewrites have nothing left to render, so they join once the workers are done.
	for f, rw := range g.rewrites {
		pending = append(pending, pendingFile{
			source:   f.Pkg.FSPath.Join(f.Name),
			contents: rw.Data(),
		})
	}

	slices.SortStableFunc(pending, func(a, b pendingFile) int {
		return strings.Compare(a.source.ToIO(), b.source.ToIO())
	})

	of := make([]overlay.File, 0, len(pending))
	for _, p := range pending {
		if p.err != nil {
			g.Errs.Add(errRender.InFile(p.source.ToIO()).Wrapping(p.err))
			continue
		}
		of = append(of, overlay.File{
			Source:   p.source,
			Contents: p.contents,
		})
	}
	return of
}

// InsertTestSupport inserts an import of the testsupport package in the given package.
//
// The import goes on the package clause's line, like every rewrite of a user's
// file: a rewrite must not add lines, since tools that ignore line directives
// (cmd/cover since Go 1.27) would report everything below it on the wrong line.
// It has no trailing semicolon: the newline ending the line inserts one, and
// another insert at the same position (secrets) brings its own leading one,
// where two in a row would be an empty declaration and fail to parse.
func (g *Generator) InsertTestSupport(pkg *pkginfo.Package) {
	if g.addedTestSupport[pkg.ImportPath] {
		return
	}
	g.addedTestSupport[pkg.ImportPath] = true

	file := pkg.Files[0]
	rw := g.Rewrite(file)
	a := file.AST()

	insertPos := a.Name.End()
	ln := g.FS.Position(insertPos)
	rw.Insert(insertPos, []byte(fmt.Sprintf(";import _ %s/*line :%d:%d*/",
		strconv.Quote("encore.dev/appruntime/shared/testsupport"), ln.Line, ln.Column)))
}
