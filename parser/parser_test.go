package parser

import (
	"bytes"
	"fmt"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/pkg/regexp"
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
		pkgs, err := collectPackages(fs, base, modulePath, "", goparser.ParseComments, true)
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
			e.Values["wd"] = e.WorkDir
			e.Values["output"] = &bytes.Buffer{}
			e.Values["errs"] = &bytes.Buffer{}
			return os.WriteFile(filepath.Join(e.WorkDir, "go.mod"), gomod, 0755)
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"parse": func(ts *testscript.TestScript, neg bool, args []string) {
				stdout := ts.Value("output").(*bytes.Buffer)
				stderr := ts.Value("errs").(*bytes.Buffer)
				defer func() {
					ts.Logf("stdout: %s", stdout.String())
					ts.Logf("stderr: %s", stderr.String())
				}()

				wd := ts.Value("wd").(string)
				modPath := filepath.Join(wd, "go.mod")
				modData, err := os.ReadFile(modPath)
				if err != nil {
					ts.Fatalf("cannot read go.mod: %v", err)
				}
				modFile, err := modfile.Parse(modPath, modData, nil)
				if err != nil {
					ts.Fatalf("cannot parse go.mod: %v", err)
				}

				cfg := &Config{
					AppRoot:    wd,
					WorkingDir: ".",
					ModulePath: modFile.Module.Mod.Path,
				}
				res, err := Parse(cfg)
				if err != nil {
					scanner.PrintError(stderr, err)
				}
				if err != nil && !neg {
					ts.Fatalf("parse failure")
				} else if err == nil && neg {
					ts.Fatalf("wanted failure, got unexpected success")
				}

				for _, svc := range res.Meta.Svcs {
					fmt.Fprintf(stdout, "svc %s dbs=%s\n", svc.Name, strings.Join(svc.Databases, ","))
				}
				for _, svc := range res.App.Services {
					for _, rpc := range svc.RPCs {
						var recvName string
						if rpc.SvcStruct != nil {
							recvName = "*" + rpc.SvcStruct.Name
						}
						fmt.Fprintf(stdout, "rpc %s.%s access=%v raw=%v path=%v recv=%v\n",
							svc.Name, rpc.Name, rpc.Access, rpc.Raw, rpc.Path, recvName)
					}

					for _, config := range svc.ConfigLoads {
						fmt.Fprintf(stdout, "config %s %s\n", svc.Name, config.ConfigStruct.Type)
					}
				}
				for _, job := range res.App.CronJobs {
					fmt.Fprintf(stdout, "cronJob %s title=%q\n", job.ID, job.Title)
				}
				for _, topic := range res.App.PubSubTopics {
					fmt.Fprintf(stdout, "pubsubTopic %s\n", topic.Name)

					for _, pub := range topic.Publishers {
						if pub.Service != nil {
							fmt.Fprintf(stdout, "pubsubPublisher %s %s\n", topic.Name, pub.Service.Name)
						}
						if pub.GlobalMiddleware != nil {
							fmt.Fprintf(stdout, "pubsubPublisher middlware %s %s\n", topic.Name, pub.GlobalMiddleware.Name)
						}
					}

					for _, sub := range topic.Subscribers {
						fmt.Fprintf(stdout, "pubsubSubscriber %s %s %s %d %d %d %d %d\n", topic.Name, sub.Name, sub.DeclFile.Pkg.Service.Name, sub.AckDeadline, sub.MessageRetention, sub.MaxRetries, sub.MinRetryBackoff, sub.MaxRetryBackoff)
					}
				}
				for _, pkg := range res.App.Packages {
					for _, res := range pkg.Resources {
						switch res := res.(type) {
						case *est.SQLDB:
							fmt.Fprintf(stdout, "resource %s %s.%s db=%s", res.Type(), pkg.Name, res.Ident().Name, res.DBName)
						default:
							fmt.Fprintf(stdout, "resource %s %s.%s\n", res.Type(), pkg.Name, res.Ident().Name)
						}
					}
				}
			},
			"output": func(ts *testscript.TestScript, neg bool, args []string) {
				stdout := ts.Value("output").(*bytes.Buffer)
				m, err := regexp.Match(args[0], stdout.String())
				if err != nil {
					ts.Fatalf("invalid pattern: %v", err)
				}
				if !m && !neg {
					ts.Fatalf("output does not match %q", args[0])
				} else if m && neg {
					ts.Fatalf("output unexpectedly matches %q", args[0])
				}
			},
			"err": func(ts *testscript.TestScript, neg bool, args []string) {
				stderr := ts.Value("errs").(*bytes.Buffer)
				m, err := regexp.Match(args[0], stderr.String())
				if err != nil {
					ts.Fatalf("invalid pattern: %v", err)
				}
				if !m && !neg {
					ts.Fatalf("stderr does not match %q", args[0])
				} else if m && neg {
					ts.Fatalf("stderr unexpectedly matches %q", args[0])
				}
			},
		},
	})
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, nil))
}
