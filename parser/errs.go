package parser

import (
	"go/token"
)

func (p *parser) err(pos token.Pos, msg string) {
	p.errors.Add(pos, msg)
}

func (p *parser) errf(pos token.Pos, format string, args ...interface{}) {
	p.errors.Addf(pos, format, args...)
}

func (p *parser) abort() {
	p.errors.Abort()
}
