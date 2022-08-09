package parser

import (
	"fmt"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rogpeppe/go-internal/txtar"
	"golang.org/x/mod/modfile"

	"encr.dev/parser/est"
)

func TestCollectPackages(t *testing.T) {
	const modulePath = "test.path"
	tests := []struct {
		Archive string
		Pkgs    []*est.Package
		Err     string
	}{
		{
			Archive: `
-- a/a.go --
package foo
-- a/b/b.go --
package bar
`,
			Pkgs: []*est.Package{
				{
					Name:       "foo",
					ImportPath: modulePath + "/a",
					RelPath:    "a",
					Dir:        "./a",
				},
				{
					Name:       "bar",
					ImportPath: modulePath + "/a/b",
					RelPath:    "a/b",
					Dir:        "./a/b",
				},
			},
		},
		{
			Archive: `
-- a/a.go --
package fo/;
`,
			Err: ".*a.go:.*expected ';', found '/'",
		},
		{
			Archive: `
-- a/a.go --
package a;
-- a/b.go --
package b;
`,
			Err: "got multiple package names in directory: a and b",
		},
		{
			Archive: `
-- a/a.txt --
`,
			Pkgs: []*est.Package{},
		},
	}

	c := qt.New(t)
	for i, test := range tests {
		a := txtar.Parse([]byte(test.Archive))
		base := t.TempDir()
		err := txtar.Write(a, base)
		c.Assert(err, qt.IsNil, qt.Commentf("test #%d", i))

		fs := token.NewFileSet()
		pkgs, err := collectPackages(fs, base, modulePath, goparser.ParseComments, true)
		if test.Err != "" {
			c.Assert(err, qt.ErrorMatches, test.Err, qt.Commentf("test #%d", i))
			continue
		}
		c.Assert(err, qt.IsNil)
		c.Assert(pkgs, qt.HasLen, len(test.Pkgs), qt.Commentf("test #%d", i))
		for i, got := range pkgs {
			want := test.Pkgs[i]
			c.Assert(got.Name, qt.Equals, want.Name)
			c.Assert(got.ImportPath, qt.Equals, want.ImportPath)
			c.Assert(got.RelPath, qt.Equals, want.RelPath)
			c.Assert(got.Dir, qt.Equals, filepath.Join(base, filepath.FromSlash(want.Dir)))
		}
	}
}

func TestCompile(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(e *testscript.Env) error {
			gomod := []byte("module test\n\nrequire encore.dev v0.0.6")
			return ioutil.WriteFile(filepath.Join(e.WorkDir, "go.mod"), gomod, 0755)
		},
	})
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"parse": func() int {
			wd, err := os.Getwd()
			if err != nil {
				os.Stderr.WriteString(err.Error())
				return 1
			}
			modPath := filepath.Join(wd, "go.mod")
			modData, err := ioutil.ReadFile(modPath)
			if err != nil {
				os.Stderr.WriteString(err.Error())
				return 1
			}
			modFile, err := modfile.Parse(modPath, modData, nil)
			if err != nil {
				os.Stderr.WriteString(err.Error())
				return 1
			}

			cfg := &Config{
				AppRoot:    wd,
				WorkingDir: ".",
				ModulePath: modFile.Module.Mod.Path,
			}
			res, err := Parse(cfg)
			if err != nil {
				if list, ok := err.(scanner.ErrorList); ok {
					for _, e := range list {
						os.Stderr.WriteString(e.Error())
					}
					return 1
				}
				os.Stderr.WriteString(err.Error())
				return 1
			}

			for _, svc := range res.Meta.Svcs {
				fmt.Fprintf(os.Stdout, "svc %s dbs=%s\n", svc.Name, strings.Join(svc.Databases, ","))
			}
			for _, svc := range res.App.Services {
				for _, rpc := range svc.RPCs {
					var recvName string
					if rpc.SvcStruct != nil {
						recvName = "*" + rpc.SvcStruct.Name
					}
					fmt.Fprintf(os.Stdout, "rpc %s.%s access=%v raw=%v path=%v recv=%v\n",
						svc.Name, rpc.Name, rpc.Access, rpc.Raw, rpc.Path, recvName)
				}
			}
			for _, job := range res.App.CronJobs {
				fmt.Fprintf(os.Stdout, "cronJob %s title=%q\n", job.ID, job.Title)
			}
			for _, topic := range res.App.PubSubTopics {
				fmt.Fprintf(os.Stdout, "pubsubTopic %s\n", topic.Name)

				for _, pub := range topic.Publishers {
					fmt.Fprintf(os.Stdout, "pubsubPublisher %s %s\n", topic.Name, pub.DeclFile.Pkg.Service.Name)
				}

				for _, sub := range topic.Subscribers {
					fmt.Fprintf(os.Stdout, "pubsubSubscriber %s %s %s %d %d %d %d %d\n", topic.Name, sub.Name, sub.DeclFile.Pkg.Service.Name, sub.AckDeadline, sub.MessageRetention, sub.MaxRetries, sub.MinRetryBackoff, sub.MaxRetryBackoff)
				}
			}
			for _, pkg := range res.App.Packages {
				for _, res := range pkg.Resources {
					switch res := res.(type) {
					case *est.SQLDB:
						fmt.Fprintf(os.Stdout, "resource %s %s.%s db=%s", res.Type(), pkg.Name, res.Ident().Name, res.DBName)
					default:
						fmt.Fprintf(os.Stdout, "resource %s %s.%s\n", res.Type(), pkg.Name, res.Ident().Name)
					}
				}
			}
			return 0
		},
	}))
}
