package compiler

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/mod/modfile"
)

func TestMergeModfiles(t *testing.T) {
	c := qt.New(t)
	foo := `module foo

require (
	a v1.2.0
	b v1.2.0
	c v1.2.0
)
`
	bar := `module bar

require (
	a v1.3.0
	b v1.1.0
	d v1.5.0
)
`
	modFoo, err := modfile.Parse("foo", []byte(foo), nil)
	c.Assert(err, qt.IsNil)
	modBar, err := modfile.Parse("bar", []byte(bar), nil)
	c.Assert(err, qt.IsNil)

	mergeModfiles(modFoo, modBar)
	out := modfile.Format(modFoo.Syntax)

	c.Assert(string(out), qt.Equals, `module foo

require (
	a v1.3.0
	b v1.2.0
	c v1.2.0
	d v1.5.0
)
`)
}
