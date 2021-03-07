package names

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"encr.dev/parser/est"
	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		Name    string
		Track   TrackedPackages
		Archive string

		// Expected error, if any
		Err string
		// Names expected to be found in package scope
		PkgNames []string
		// Expected (import -> name) pairs in *File.NameToPath and *File.PathToName
		PkgMap map[string]string
		// Name information to validate
		Names map[string]*Name
	}{
		{
			Name: "pkg_map",
			Track: TrackedPackages{
				"foo/path": "foo",
			},
			Archive: `
-- pkg.go --
package pkg

import "foo/path"
			`,

			PkgNames: []string{},
			PkgMap:   map[string]string{"foo/path": "foo"},
			Names:    map[string]*Name{},
		},
		{
			Name: "pkg_var",
			Archive: `
-- pkg.go --
package pkg

var foo = true
			`,

			PkgNames: []string{"foo"},
			Names: map[string]*Name{
				"foo": {
					Package: true,
				},
			},
		},
		{
			Name: "func_local",
			Archive: `
-- pkg.go --
package pkg

func fn() {
	bar := "hello"
}
			`,

			PkgNames: []string{"fn"},
			Names: map[string]*Name{
				"bar": {Local: true},
			},
		},
		{
			Name:  "imported_name",
			Track: TrackedPackages{"foo/path": "foo"},
			Archive: `
-- pkg.go --
package pkg

import "foo/path"

func fn() {
	bar := foo.Moo()
}
			`,

			PkgNames: []string{"fn"},
			Names: map[string]*Name{
				"bar": {Local: true},
				"foo": {ImportPath: "foo/path"},
			},
		},
		{
			Name: "import_name_collision",
			Track: TrackedPackages{
				"foo":  "foo",
				"foo2": "foo",
			},
			Archive: `
-- pkg.go --
package pkg

import (
	"foo"
	"foo2"
)
			`,
			Err: `.*pkg.go:5:2: name foo already declared \(import of package foo\)`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			c := qt.New(t)
			dir := t.TempDir()
			err := txtar.Write(txtar.Parse([]byte(test.Archive)), dir)
			c.Assert(err, qt.IsNil)

			fs := token.NewFileSet()
			pkgs, err := parser.ParseDir(fs, dir, nil, parser.ParseComments)
			c.Assert(err, qt.IsNil)
			astPkg := pkgs["pkg"]
			c.Assert(astPkg, qt.Not(qt.IsNil))

			pkg := &est.Package{
				AST:        astPkg,
				Name:       "pkg",
				ImportPath: "foo",
				RelPath:    ".",
				Dir:        dir,
			}
			var firstFile *ast.File
			for _, f := range astPkg.Files {
				firstFile = f
				break
			}
			pkg.Files = []*est.File{{AST: firstFile}}

			res, err := Resolve(fs, test.Track, pkg)
			if test.Err != "" {
				c.Assert(err, qt.ErrorMatches, test.Err)
				return
			}
			c.Assert(err, qt.IsNil)

			// Compare res with our expectations
			c.Assert(res.Decls, qt.HasLen, len(test.PkgNames))
			for _, name := range test.PkgNames {
				c.Assert(res.Decls[name], qt.Not(qt.IsNil))
			}

			var file *est.File
			for _, file = range pkg.Files {
				break
			}
			f := res.Files[file]
			c.Assert(f, qt.Not(qt.IsNil))
			for path, name := range test.PkgMap {
				c.Assert(f.NameToPath[name], qt.Equals, path)
				c.Assert(f.PathToName[path], qt.Equals, name)
			}

			ast.Inspect(file.AST, func(node ast.Node) bool {
				if id, ok := node.(*ast.Ident); ok {
					name := id.Name
					if want, ok := test.Names[name]; ok {
						info := res.Files[file].Idents[id]
						c.Assert(info, qt.Not(qt.IsNil), qt.Commentf("%s: %s", fs.Position(id.Pos()), name))
						c.Assert(info, qt.DeepEquals, want)
					}
				}
				return true
			})
		})
	}
}

func TestKeyValueExpr(t *testing.T) {
	const data = `-- pkg.go --
package pkg

type Foo struct {
	Key string
}

func Key() {
	return &Foo{Key: "test"}
}
`
	c := qt.New(t)
	dir := t.TempDir()
	err := txtar.Write(txtar.Parse([]byte(data)), dir)
	c.Assert(err, qt.IsNil)

	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, dir, nil, parser.ParseComments)
	c.Assert(err, qt.IsNil)
	astPkg := pkgs["pkg"]
	c.Assert(astPkg, qt.Not(qt.IsNil))

	pkg := &est.Package{
		AST:        astPkg,
		Name:       "pkg",
		ImportPath: "foo",
		RelPath:    ".",
		Dir:        dir,
	}
	var firstFile *ast.File
	for _, f := range astPkg.Files {
		firstFile = f
		break
	}
	pkg.Files = []*est.File{{AST: firstFile}}

	res, err := Resolve(fs, nil, pkg)
	c.Assert(err, qt.IsNil)

	// Compare res with our expectations
	var file *est.File
	for _, file = range pkg.Files {
		break
	}
	f := res.Files[file]
	c.Assert(f, qt.Not(qt.IsNil))

	// Walk to our "Key" ident within the &Foo{} struct literal.
	inLiteral := false
	ast.Inspect(file.AST, func(node ast.Node) bool {
		if _, ok := node.(*ast.KeyValueExpr); ok {
			inLiteral = true
		}

		if inLiteral {
			if id, ok := node.(*ast.Ident); ok && id.Name == "Key" {
				// We want this ident to not be tracked.
				_, ok := f.Idents[id]
				c.Assert(ok, qt.IsFalse)
			}
		}
		return true
	})
}
