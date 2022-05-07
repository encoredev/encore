package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/errlist"
)

func TestParseStructTag(t *testing.T) {
	tests := []struct {
		Tag  string
		Want structFieldOptions
	}{
		{
			Tag:  `json:"foo" qs:"bar"`,
			Want: structFieldOptions{JSONName: "foo", QueryStringName: "bar"},
		},
		{
			Tag:  `json:"foo,omitempty"`,
			Want: structFieldOptions{JSONName: "foo"},
		},
		{
			Tag:  `json:"foo,omitempty" qs:"-" encore:"optional"`,
			Want: structFieldOptions{JSONName: "foo", QueryStringName: "-", Optional: true},
		},
		{
			Tag:  `header:"X-Forwarded-For"`,
			Want: structFieldOptions{HTTPHeaderName: "X-Forwarded-For"},
		},
	}

	fset := token.NewFileSet()
	p := &parser{
		fset:   fset,
		errors: errlist.New(fset),
	}
	c := qt.New(t)
	for _, test := range tests {
		x, err := goparser.ParseExpr("`" + test.Tag + "`")
		c.Assert(err, qt.IsNil)
		lit := x.(*ast.BasicLit)
		got := p.parseStructTag(lit)
		c.Assert(p.errors.Err(), qt.IsNil)
		c.Assert(got, qt.DeepEquals, test.Want)
	}
}
