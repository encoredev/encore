package parser

import (
	"fmt"
	"go/scanner"
	"go/token"
	"path/filepath"
	"strings"

	"encr.dev/parser/internal"
)

type bailout struct{}

func (p *parser) err(pos token.Pos, msg string) {
	pp := p.fset.Position(pos)
	n := len(p.errors)
	if n > 0 && p.errors[n-1].Pos.Line == pp.Line {
		return // spurious
	} else if n > 10 {
		panic(bailout{})
	}
	internal.AddErrToList(&p.errors, pp, msg)
}

func (p *parser) errf(pos token.Pos, format string, args ...interface{}) {
	p.err(pos, fmt.Sprintf(format, args...))
}

// makeErrsRelative goes through the errors and tweaks the filename to be relative
// to the relwd.
func makeErrsRelative(errs scanner.ErrorList, root, relwd string) {
	wdroot := filepath.Join(root, relwd)
	for _, e := range errs {
		fn := e.Pos.Filename
		if strings.HasPrefix(fn, root) {
			if rel, err := filepath.Rel(wdroot, fn); err == nil {
				e.Pos.Filename = rel
			}
		}
	}
}
