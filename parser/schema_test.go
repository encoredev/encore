package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"testing"

	"github.com/fatih/structtag"
	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/errlist"
)

func TestParseStructTag(t *testing.T) {
	tests := []struct {
		Tag  string
		Want structFieldOptions
	}{
		{
			Tag: `json:"foo" qs:"bar"`,
			Want: structFieldOptions{
				JSONName:        "foo",
				QueryStringName: "bar",
				RawTag:          `json:"foo" qs:"bar"`,
				Tags: []*structtag.Tag{
					{Key: "json", Name: "foo"},
					{Key: "qs", Name: "bar"},
				},
			},
		},
		{
			Tag: `json:"foo,omitempty"`,
			Want: structFieldOptions{
				JSONName: "foo",
				RawTag:   `json:"foo,omitempty"`,
				Tags: []*structtag.Tag{
					{Key: "json", Name: "foo", Options: []string{"omitempty"}},
				},
			},
		},
		{
			Tag: `json:"foo,omitempty" qs:"-" encore:"optional"`,
			Want: structFieldOptions{
				JSONName:        "foo",
				QueryStringName: "-",
				Optional:        true,
				RawTag:          `json:"foo,omitempty" qs:"-" encore:"optional"`,
				Tags: []*structtag.Tag{
					{Key: "json", Name: "foo", Options: []string{"omitempty"}},
					{Key: "qs", Name: "-"},
					{Key: "encore", Name: "optional"},
				},
			},
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
		got := p.parseStructTag(lit, nil)
		c.Assert(p.errors.Err(), qt.IsNil)
		c.Assert(got, qt.DeepEquals, test.Want)
	}
}
