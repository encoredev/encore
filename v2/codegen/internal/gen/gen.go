package gen

import (
	"encr.dev/v2/codegen/internal/genutil"
	"encr.dev/v2/codegen/internal/rewrite"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
)

type Generator struct {
	*parsectx.Context
	Util     *genutil.Generator
	rewrites map[*pkginfo.File]*rewrite.Rewriter
	files    map[fileKey]*File
}

func New(c *parsectx.Context) *Generator {
	return &Generator{
		Context:  c,
		Util:     genutil.NewGenerator(c.Errs),
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
