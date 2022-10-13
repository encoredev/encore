package parser

import (
	"fmt"
	"go/ast"
	"go/constant"
	goparser "go/parser"
	"go/token"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/pkg/errlist"
)

func Test_parser_parseConstantValue(t *testing.T) {
	c := qt.New(t)
	var tests = []struct {
		Expr string
		Want any
		Err  string
	}{
		{
			Expr: "true",
			Want: true,
		},
		{
			Expr: "false",
			Want: false,
		},
		{
			Expr: "\"hello world\"",
			Want: "hello world",
		},
		{
			Expr: "\"hello world\" == \"hello world\"",
			Want: true,
		},
		{
			Expr: "\"hello world\" != \"hello world\"",
			Want: false,
		},
		{
			Expr: "1*cron.Minute",
			Want: 1 * minute,
		},
		{
			Expr: "(4/2)*cron.Minute",
			Want: 2 * minute,
		},
		{
			Expr: "(4-2)*cron.Minute + cron.Hour",
			Want: 2*minute + hour,
		},
		{
			Expr: "5 / 2",
			Want: 2.5,
		},
		{
			Expr: "(5 - 4) - 1",
			Want: int64(0),
		},
		// Note the "(?s)" allows for "." to match newlines
		// This is needed when running tests with the tag `dev_build` which includes
		// stack traces from the parser in the error message.
		{
			Expr: "2.3 / 0",
			Err:  `(?s).+cannot divide by zero.*`,
		},
	}

	for i, test := range tests {
		c.Run(fmt.Sprintf("test[%d]", i), func(c *qt.C) {
			fset := token.NewFileSet()
			x, err := goparser.ParseExprFrom(fset, c.Name()+".go", test.Expr, goparser.AllErrors)
			c.Assert(err, qt.IsNil)

			pkg := &est.Package{Name: "test"}
			file := &est.File{
				Name: c.Name() + ".go",
				Pkg:  pkg,
				Path: "test/" + c.Name() + ".go",
			}
			pkg.Files = append(pkg.Files, file)

			info := &names.File{
				Idents: make(map[*ast.Ident]*names.Name),
			}
			appNames := make(names.Application)
			appNames[pkg] = &names.Resolution{
				Files: map[*est.File]*names.File{file: info},
			}

			ast.Inspect(x, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok {
						if id.Name == "cron" {
							info.Idents[id] = &names.Name{
								Package:    true,
								ImportPath: "encore.dev/cron",
							}
						}
					}
				}
				return true
			})

			p := &parser{fset: fset, errors: errlist.New(fset), names: appNames}
			value := p.parseConstantValue(file, x)
			if test.Err != "" {
				c.Check(value.Kind(), qt.Equals, constant.Unknown, qt.Commentf("Wanted an Unknown value, got: %s", value.Kind().String()))
				c.Check(p.errors.Err(), qt.IsNotNil)
				c.Check(p.errors.Err(), qt.ErrorMatches, test.Err)
			} else {
				c.Check(p.errors.Err(), qt.IsNil)
				c.Check(value.Kind(), qt.Not(qt.Equals), constant.Unknown, qt.Commentf("Result was unknown: %s", p.errors.Error()))

				switch w := test.Want.(type) {
				case bool:
					c.Check(constant.BoolVal(value), qt.Equals, test.Want)
				case string:
					c.Check(constant.StringVal(value), qt.Equals, test.Want)
				case float64:
					got, _ := constant.Float64Val(constant.ToFloat(value))
					c.Check(got, qt.Equals, test.Want)
				case int:
					got, _ := constant.Int64Val(constant.ToInt(value))
					c.Check(got, qt.Equals, int64(w))
				case int64:
					got, _ := constant.Int64Val(constant.ToInt(value))
					c.Check(got, qt.Equals, test.Want)
				}
			}
		})
	}
}
