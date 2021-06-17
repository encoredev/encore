// Package parser parses Encore applications into an Encore Syntax Tree (EST).
package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
	"golang.org/x/tools/go/ast/astutil"
)

type Result struct {
	FileSet *token.FileSet
	App     *est.Application
	Meta    *meta.Data
	Nodes   map[*est.Package]TraceNodes
}

type parser struct {
	// inputs
	cfg *Config

	// accumulated results
	fset        *token.FileSet
	errors      scanner.ErrorList
	pkgs        []*est.Package
	pkgMap      map[string]*est.Package // import path -> pkg
	svcs        []*est.Service
	svcMap      map[string]*est.Service // name -> svc
	names       map[*est.Package]*names.Resolution
	authHandler *est.AuthHandler
	declMap     map[string]*schema.Decl // pkg/path.Name -> decl
	decls       []*schema.Decl
	paths       paths.Set // RPC paths
}

// Config represents the configuration options for parsing.
type Config struct {
	AppRoot    string
	Version    string
	ModulePath string
	WorkingDir string
	ParseTests bool
}

func Parse(cfg *Config) (*Result, error) {
	p := &parser{
		cfg:     cfg,
		declMap: make(map[string]*schema.Decl),
	}
	return p.Parse()
}

const (
	sqldbImportPath = "encore.dev/storage/sqldb"
	rlogImportPath  = "encore.dev/rlog"
	uuidImportPath  = "encore.dev/types/uuid"
	authImportPath  = "encore.dev/beta/auth"
)

func (p *parser) Parse() (res *Result, err error) {
	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(bailout); !ok {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				err = fmt.Errorf("parser panicked:\n%s", buf)
			}
		}
		if err == nil {
			p.errors.Sort()
			makeErrsRelative(p.errors, p.cfg.AppRoot, p.cfg.WorkingDir)
			err = p.errors.Err()
		}
	}()
	p.fset = token.NewFileSet()

	p.pkgs, err = collectPackages(p.fset, p.cfg.AppRoot, p.cfg.ModulePath, goparser.ParseComments, p.cfg.ParseTests)
	if err != nil {
		return nil, err
	}
	p.pkgMap = make(map[string]*est.Package)
	for _, pkg := range p.pkgs {
		p.pkgMap[pkg.ImportPath] = pkg
	}

	track := names.TrackedPackages{
		sqldbImportPath: "sqldb",
		rlogImportPath:  "rlog",
		uuidImportPath:  "uuid",
		authImportPath:  "auth",

		"net/http":      "http",
		"context":       "context",
		"encoding/json": "json",
		"time":          "time",
	}
	p.resolveNames(track)
	p.parseServices()
	p.parseReferences()
	p.parseSecrets()
	p.validateApp()

	sort.Slice(p.pkgs, func(i, j int) bool {
		return p.pkgs[i].RelPath < p.pkgs[j].RelPath
	})
	sort.Slice(p.svcs, func(i, j int) bool {
		return p.svcs[i].Name < p.svcs[j].Name
	})
	for _, svc := range p.svcs {
		sort.Slice(svc.RPCs, func(i, j int) bool {
			return svc.RPCs[i].Name < svc.RPCs[j].Name
		})
	}
	app := &est.Application{
		ModulePath:  p.cfg.ModulePath,
		Packages:    p.pkgs,
		Services:    p.svcs,
		Decls:       p.decls,
		AuthHandler: p.authHandler,
	}
	md, nodes, err := ParseMeta(p.cfg.Version, p.cfg.AppRoot, app)
	if err != nil {
		return nil, err
	}
	return &Result{
		FileSet: p.fset,
		App:     app,
		Meta:    md,
		Nodes:   nodes,
	}, nil
}

// collectPackages collects and parses the regular Go AST
// for all subdirectories in the root.
func collectPackages(fs *token.FileSet, rootDir, rootImportPath string, mode goparser.Mode, parseTests bool) ([]*est.Package, error) {
	var pkgs []*est.Package
	var errors scanner.ErrorList
	filter := func(f os.FileInfo) bool {
		return parseTests || !strings.HasSuffix(f.Name(), "_test.go")
	}
	err := walkDirs(rootDir, func(dir, relPath string, files []os.FileInfo) error {
		ps, pkgFiles, err := parseDir(fs, dir, relPath, filter, mode)
		if err != nil {
			// If the error is an error list, it means we have a parsing error.
			// Keep going with other directories in that case.
			if el, ok := err.(scanner.ErrorList); ok {
				errors = append(errors, el...)
				return nil
			}
			return err
		}

		var pkgNames []string
		for name := range ps {
			pkgNames = append(pkgNames, name)
		}
		if n := len(ps); n > 1 {
			// We only support a single package for now
			sort.Strings(pkgNames)
			first := ps[pkgNames[0]]
			namestr := strings.Join(pkgNames[:n-1], ", ") + " and " + pkgNames[n-1]
			errors.Add(fs.Position(first.Pos()), "got multiple package names in directory: "+namestr)
			return nil
		} else if n == 0 {
			// No Go files; ignore directory
			return nil
		}

		p := ps[pkgNames[0]]

		var doc string
		for _, astFile := range p.Files {
			// HACK: getting package comments is not at all easy
			// because of the quirks of go/ast. This seems to work.
			cm := ast.NewCommentMap(fs, astFile, astFile.Comments)
			for _, cg := range cm[astFile] {
				if text := strings.TrimSpace(cg.Text()); text != "" {
					doc = text
				}
				break
			}
			if doc != "" {
				break
			}
		}

		pkg := &est.Package{
			AST:        p,
			Name:       p.Name,
			Doc:        doc,
			ImportPath: path.Clean(path.Join(rootImportPath, relPath)),
			RelPath:    path.Clean(relPath),
			Dir:        dir,
			Files:      pkgFiles,
		}
		for _, f := range pkgFiles {
			f.Pkg = pkg
		}
		pkgs = append(pkgs, pkg)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pkgs, errors.Err()
}

// resolveNames resolves identifiers for the application's packages.
// track defines the non-application packages to track usage for.
func (p *parser) resolveNames(track names.TrackedPackages) {
	p.names = make(map[*est.Package]*names.Resolution)

	for _, pkg := range p.pkgs {
		track[pkg.ImportPath] = pkg.Name
	}
	for _, pkg := range p.pkgs {
		res, err := names.Resolve(p.fset, track, pkg)
		if err != nil {
			if el, ok := err.(scanner.ErrorList); ok {
				p.errors = append(p.errors, el...)
			} else {
				p.err(pkg.Files[0].AST.Pos(), err.Error())
			}
			continue
		}
		p.names[pkg] = res
	}
	if len(p.errors) > 0 {
		panic(bailout{})
	}
}

func (p *parser) parseReferences() {
	// For all RPCs defined, store them in a map per package for faster lookup
	rpcMap := make(map[string]map[string]*est.RPC, len(p.pkgs)) // path -> name -> RPC
	for _, svc := range p.svcs {
		rpcs := make(map[string]*est.RPC, len(svc.RPCs))
		rpcMap[svc.Root.ImportPath] = rpcs
		for _, rpc := range svc.RPCs {
			rpcs[rpc.Func.Name.Name] = rpc
			rpc.File.References[rpc.Func] = &est.Node{
				Type: est.RPCDefNode,
				RPC:  rpc,
			}
		}
	}

	for _, pkg := range p.pkgs {
		for _, file := range pkg.Files {
			info := p.names[pkg].Files[file]

			// For every call, determine if it should be rewritten
		CallLoop:
			for _, call := range info.Calls {
				switch x := call.Fun.(type) {
				case *ast.SelectorExpr:
					if id, ok := x.X.(*ast.Ident); ok {
						ri := info.Idents[id]
						if ri == nil || ri.ImportPath == "" {
							continue
						}
						path := ri.ImportPath

						// Is it an RPC?
						if rpc := rpcMap[path][x.Sel.Name]; rpc != nil {
							file.References[call] = &est.Node{
								Type: est.RPCCallNode,
								RPC:  rpc,
							}
							continue CallLoop
						}
						switch path {
						case sqldbImportPath:
							file.References[call] = &est.Node{
								Type: est.SQLDBNode,
								Func: x.Sel.Name,
							}
						case rlogImportPath:
							file.References[call] = &est.Node{
								Type: est.RLogNode,
								Func: x.Sel.Name,
							}
						}
					}

				case *ast.Ident:
					ri := info.Idents[x]
					// Is it a package-local rpc?
					if ri != nil && ri.Package {
						if rpc := rpcMap[pkg.ImportPath][x.Name]; rpc != nil {
							file.References[call] = &est.Node{
								Type: est.RPCCallNode,
								RPC:  rpc,
							}
						}
					}
				}
			}

			// Find all references to RPCs and objects to rewrite that are not
			// part of rewritten calls.
			astutil.Apply(file.AST, func(c *astutil.Cursor) bool {
				if sel, ok := c.Node().(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok {
						// Have we rewritten this already?
						if file.References[c.Parent()] != nil {
							// We can't recurse into children because that will lead to
							// spurious errors since we'll look up the child Ident in the case below
							// and fail because it looks like we're referencing an RPC without calling it.
							return false
						}
						ri := info.Idents[id]
						if ri == nil || ri.ImportPath == "" {
							return true
						}
						path := ri.ImportPath
						pkg2, ok := p.pkgMap[path]
						if !ok {
							return true
						}

						// Is it an RPC?
						if rpc := rpcMap[path][sel.Sel.Name]; rpc != nil {
							p.errf(sel.Pos(), "cannot reference API %s.%s without calling it", rpc.Svc.Name, sel.Sel.Name)
							return false
						} else if h := p.authHandler; h != nil && path == h.Svc.Root.ImportPath && sel.Sel.Name == h.Name {
							p.errf(sel.Pos(), "cannot reference auth handler %s.%s from another package", h.Svc.Root.RelPath, sel.Sel.Name)
							return false
						}

						switch path {
						case sqldbImportPath:
							// Allow types to be references
							switch sel.Sel.Name {
							case "Tx", "Row", "Rows":
								return true
							default:
								p.errf(sel.Pos(), "cannot reference func %s.%s without calling it", path, sel.Sel.Name)
								return false
							}
						case rlogImportPath:
							// Allow types to be whitelisted
							switch sel.Sel.Name {
							case "Ctx":
								return true
							default:
								p.errf(sel.Pos(), "cannot reference func %s.%s without calling it", path, sel.Sel.Name)
								return false
							}
						}

						// Is it a type in a different service?
						if pkg2.Service != nil && !(pkg.Service != nil && pkg.Service.Name == pkg2.Service.Name) {
							if decl, ok := p.names[pkg2].Decls[sel.Sel.Name]; ok {
								switch decl.Type {
								case token.CONST:
									// all good
								case token.TYPE:
									// TODO check if there are methods on the type
								case token.VAR:
									p.errf(sel.Pos(), "cannot reference variable %s.%s in another Encore service", pkg2.RelPath, sel.Sel.Name)
									return false
								}
							}
						}
					}
				} else if id, ok := c.Node().(*ast.Ident); ok {
					// Have we rewritten this already?
					if file.References[c.Parent()] != nil {
						return true
					} else if _, isSel := c.Parent().(*ast.SelectorExpr); isSel {
						// We've already processed this above
						return true
					}

					ri := info.Idents[id]
					// Is it a package-local rpc?
					if ri != nil && ri.Package {
						if rpc := rpcMap[pkg.ImportPath][id.Name]; rpc != nil {
							p.errf(id.Pos(), "cannot reference API %s without calling it", rpc.Name)
							return false
						}
					}
				}
				return true
			}, nil)

			if pkg.Service == nil && len(file.References) > 0 {
				for astNode, node := range file.References {
					switch node.Type {
					case est.RPCCallNode:
						rpc := node.RPC
						p.errf(astNode.Pos(), "cannot reference API %s.%s outside of a service\n\tpackage %s is not considered a service (it has no APIs defined)", rpc.Svc.Name, rpc.Name, pkg.Name)
					case est.SQLDBNode:
						p.errf(astNode.Pos(), "cannot use package %s outside of a service\n\tpackage %s is not considered a service (it has no APIs defined)", sqldbImportPath, pkg.Name)
					case est.RLogNode:
						// rlog calls are allowed outside of services
					default:
						p.errf(astNode.Pos(), "invalid reference outside of a service\n\tpackage %s is not considered a service (it has no APIs defined)", pkg.Name)
					}
					break
				}
			}
		}
	}
}

func (p *parser) parseSecrets() {
	for _, pkg := range p.pkgs {
		p.parsePackageSecrets(pkg)
	}
}

func (p *parser) parsePackageSecrets(pkg *est.Package) {
	var secretsDecl *ast.StructType
SpecLoop:
	for _, f := range pkg.Files {
		for _, decl := range f.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				spec := spec.(*ast.ValueSpec)
				for _, name := range spec.Names {
					if name.Name == "secrets" {
						typ, ok := spec.Type.(*ast.StructType)
						if !ok {
							p.err(spec.Pos(), "secrets var must be a struct")
							return
						} else if len(spec.Names) != 1 {
							p.err(spec.Pos(), "secrets var must be declared separately")
							return
						} else if len(spec.Values) != 0 {
							p.err(spec.Pos(), "secrets var must not be given a value")
							return
						}
						secretsDecl = typ
						f.References[spec] = &est.Node{Type: est.SecretsNode}
						break SpecLoop
					}
				}
			}
		}
	}

	if secretsDecl == nil {
		// No secrets
		return
	}

	names := p.names[pkg]
	var secretNames []string
	for _, field := range secretsDecl.Fields.List {
		if typ, ok := field.Type.(*ast.Ident); !ok || typ.Name != "string" {
			p.errf(typ.Pos(), "field %s is not of type string", field.Names[0].Name)
			return
		} else if decl := names.Decls["string"]; decl != nil {
			pp := p.fset.Position(decl.Pos)
			p.errf(typ.Pos(), "package shadows built-in type string:\n\tshadowing declaration at %s:%d",
				filepath.Base(pp.Filename), pp.Line)
			return
		}
		for _, name := range field.Names {
			secretNames = append(secretNames, name.Name)
		}
	}
	sort.Strings(secretNames)
	pkg.Secrets = secretNames
}

// validateApp performs full-app validation after everything has been parsed.
func (p *parser) validateApp() {
	// Error if we have auth endpoints without an auth handlers
	if p.authHandler == nil {
	AuthLoop:
		for _, svc := range p.svcs {
			for _, rpc := range svc.RPCs {
				if rpc.Access == est.Auth {
					p.errf(rpc.Func.Pos(), "cannot use \"auth\" access type, no auth handler is defined in the app")
					break AuthLoop
				}
			}
		}
	}
}
