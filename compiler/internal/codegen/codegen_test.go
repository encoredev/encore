package codegen

import (
	"bytes"
	"fmt"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser"
	"encr.dev/parser/est"
	"encr.dev/pkg/golden"
)

func TestMain(m *testing.M) {
	golden.TestMain(m)
}

func TestCodeGenMain(t *testing.T) {
	c := qt.New(t)
	tests, err := filepath.Glob("./testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	c.Assert(err, qt.IsNil)

	for i, test := range tests {
		path := test
		name := strings.TrimSuffix(filepath.Base(test), ".txt")
		c.Run(name, func(c *qt.C) {
			archiveData, err := os.ReadFile(path)
			c.Assert(err, qt.IsNil)
			a := txtar.Parse(archiveData)
			base := c.TempDir()
			err = txtar.Write(a, base)
			c.Assert(err, qt.IsNil, qt.Commentf("test #%d", i))

			res, err := parser.Parse(&parser.Config{
				AppRoot:    base,
				ModulePath: "encore.app",
				WorkingDir: ".",
			})
			c.Assert(err, qt.IsNil)

			bld := NewBuilder(res)
			var buf bytes.Buffer
			buf.WriteString("// main code\n")
			f, err := bld.Main()
			c.Assert(err, qt.IsNil)
			err = f.Render(&buf)
			if err != nil {
				c.Fatalf("render failed: %v", err)
			}
			c.Assert(err, qt.IsNil)

			fs := token.NewFileSet()
			code := buf.Bytes()
			_, err = goparser.ParseFile(fs, c.Name()+".go", code, goparser.AllErrors)
			c.Assert(err, qt.IsNil)

			for _, svc := range res.App.Services {
				// Find all RPCs referenced
				refs := make(map[string]bool)
				var rpcs []*est.RPC
				for _, pkg := range svc.Pkgs {
					for _, f := range pkg.Files {
						for _, ref := range f.References {
							if ref.Type == est.RPCCallNode {
								key := ref.RPC.Svc.Name + "." + ref.RPC.Name
								if !refs[key] {
									refs[key] = true
									rpcs = append(rpcs, ref.RPC)
								}
							}
						}
					}
				}
				if len(rpcs) > 0 {
					fmt.Fprintf(&buf, "\n\n// wrappers for service %s\n", svc.Name)
					err = bld.Wrappers(svc.Root, svc.RPCs).Render(&buf)
					if err != nil {
						c.Fatalf("got render error: \n%s", err.Error())
					}
					c.Assert(err, qt.IsNil)
					code = buf.Bytes()[len(code):]
					fs := token.NewFileSet()
					_, err = goparser.ParseFile(fs, c.Name()+".go", code, goparser.AllErrors)
					if err != nil {
						c.Fatalf("got parse error: \n%s\ncode:\n%s", err.Error(), code)
					}
				}
			}

			golden.Test(c.TB, buf.String())
		})
	}
}
func TestCodeGen_TestMain(t *testing.T) {
	c := qt.New(t)
	tests, err := filepath.Glob("./testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	c.Assert(err, qt.IsNil)

	for i, test := range tests {
		path := test
		name := strings.TrimSuffix(filepath.Base(test), ".txt")
		c.Run(name, func(c *qt.C) {
			archiveData, err := os.ReadFile(path)
			c.Assert(err, qt.IsNil)
			a := txtar.Parse(archiveData)
			base := c.TempDir()
			err = txtar.Write(a, base)
			c.Assert(err, qt.IsNil, qt.Commentf("test #%d", i))

			res, err := parser.Parse(&parser.Config{
				AppRoot:    base,
				ModulePath: "encore.app",
				WorkingDir: ".",
			})
			c.Assert(err, qt.IsNil)

			bld := NewBuilder(res)
			var buf bytes.Buffer
			var code []byte

			for _, pkg := range res.App.Packages {
				fmt.Fprintf(&buf, "// pkg %s\n", pkg.RelPath)
				err = bld.TestMain(pkg, res.App.Services).Render(&buf)
				if err != nil {
					c.Fatalf("got render error: \n%s", err.Error())
				}
				c.Assert(err, qt.IsNil)
				code = buf.Bytes()[len(code):]
				buf.WriteString("\n")
				fs := token.NewFileSet()
				_, err = goparser.ParseFile(fs, c.Name()+".go", code, goparser.AllErrors)
				if err != nil {
					c.Fatalf("got parse error: \n%s\ncode:\n%s", err.Error(), code)
				}
			}

			golden.Test(c.TB, buf.String())
		})
	}
}
