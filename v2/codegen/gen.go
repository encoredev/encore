package codegen

import (
	"bytes"
	"fmt"
	"go/token"

	"golang.org/x/exp/slices"

	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/codegen/internal/rewrite"
	"encr.dev/v2/internal/overlay"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
)

type Generator struct {
	*parsectx.Context
	Util     *genutil.Helper
	rewrites map[*pkginfo.File]*rewrite.Rewriter
	files    map[fileKey]*File
}

func New(c *parsectx.Context) *Generator {
	return &Generator{
		Context:  c,
		Util:     genutil.NewHelper(c.Errs),
		rewrites: make(map[*pkginfo.File]*rewrite.Rewriter),
		files:    make(map[fileKey]*File),
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
	pkgPath paths.Pkg
	suffix  string
}

func (g *Generator) File(pkg *pkginfo.Package, suffix string) *File {
	key := fileKey{pkg.ImportPath, suffix}
	if f, ok := g.files[key]; ok {
		return f
	}
	f := newFile(pkg, suffix)
	g.files[key] = f
	return f
}

func (g *Generator) InjectFile(pkgPath paths.Pkg, pkgName string, pkgDir paths.FS, suffix string) *File {
	key := fileKey{pkgPath, suffix}
	if f, ok := g.files[key]; ok {
		return f
	}
	f := newFileForPath(pkgPath, pkgName, pkgDir, suffix)
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
			pp := token.Position{Filename: source.ToIO()}
			g.Errs.AddPosition(pp, fmt.Sprintf("failed to render codegen: %v", err))
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
