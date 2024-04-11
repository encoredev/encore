package codegen

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"

	"encr.dev/pkg/paths"
	"encr.dev/v2/app/legacymeta"
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

func (g *Generator) Overlays() []overlay.File {
	var of []overlay.File

	var buf bytes.Buffer
	for _, f := range g.files {
		source := f.dir.Join(f.name())

		buf.Reset()
		if err := f.Render(&buf); err != nil {
			g.Errs.Add(errRender.InFile(source.ToIO()).Wrapping(err))
			continue
		}

		// Get a copy of the buffer since we reuse it across files.
		contents := slices.Clone(buf.Bytes())

		of = append(of, overlay.File{
			Source:   source,
			Contents: contents,
		})
	}

	for f, rw := range g.rewrites {
		source := f.Pkg.FSPath.Join(f.Name)
		of = append(of, overlay.File{
			Source:   source,
			Contents: rw.Data(),
		})
	}

	return of
}

// InsertTestSupport inserts an import of the testsupport package in the given package.
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
	rw.Insert(insertPos, []byte(fmt.Sprintf("\nimport _ %s;/*line :%d:%d*/",
		strconv.Quote("encore.dev/appruntime/shared/testsupport"), ln.Line, ln.Column)))
}
