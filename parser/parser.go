// Package parser parses Encore applications into an Encore Syntax Tree (EST).
package parser

import (
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"

	"encr.dev/pkg/errlist"
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
	fset         *token.FileSet
	errors       *errlist.List
	pkgs         []*est.Package
	pkgMap       map[string]*est.Package // import path -> pkg
	svcs         []*est.Service
	jobs         []*est.CronJob
	svcMap       map[string]*est.Service // name -> svc
	svcPkgPaths  map[string]*est.Service // pkg path -> svc
	jobsMap      map[string]*est.CronJob // ID -> job
	pubSubTopics []*est.PubSubTopic
	names        names.Application
	authHandler  *est.AuthHandler
	declMap      map[string]*schema.Decl // pkg/path.Name -> decl
	decls        []*schema.Decl
	paths        paths.Set // RPC paths

	// validRPCReferences is a set of ast nodes that are allowed to
	// reference RPCs without calling them.
	validRPCReferences map[ast.Node]bool
}

// Config represents the configuration options for parsing.
// TODO(domblack): Remove AppRevision and AppHasUncommittedChanges from here as it's compiler concern not a parser concern
type Config struct {
	AppRoot                  string
	AppRevision              string
	AppHasUncommittedChanges bool
	ModulePath               string
	WorkingDir               string
	ParseTests               bool
}

func Parse(cfg *Config) (*Result, error) {
	p := &parser{
		cfg:                cfg,
		declMap:            make(map[string]*schema.Decl),
		validRPCReferences: make(map[ast.Node]bool),
		jobsMap:            make(map[string]*est.CronJob),
	}
	return p.Parse()
}

const (
	sqldbImportPath = "encore.dev/storage/sqldb"
	rlogImportPath  = "encore.dev/rlog"
	uuidImportPath  = "encore.dev/types/uuid"
	authImportPath  = "encore.dev/beta/auth"
	cronImportPath  = "encore.dev/cron"
)

var defaultTrackedPackages = names.TrackedPackages{
	rlogImportPath: "rlog",
	uuidImportPath: "uuid",
	authImportPath: "auth",
	cronImportPath: "cron",

	"net/http":      "http",
	"context":       "context",
	"encoding/json": "json",
	"time":          "time",
}

func (p *parser) Parse() (res *Result, err error) {
	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(errlist.Bailout); !ok {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				err = fmt.Errorf("parser panicked: %+v\n%s", e, buf)
			}
		}
		if err == nil {
			p.errors.Sort()
			p.errors.MakeRelative(p.cfg.AppRoot, p.cfg.WorkingDir)
			err = p.errors.Err()
		}
	}()
	p.fset = token.NewFileSet()
	p.errors = errlist.New(p.fset)

	p.pkgs, err = collectPackages(p.fset, p.cfg.AppRoot, p.cfg.ModulePath, goparser.ParseComments, p.cfg.ParseTests)
	if err != nil {
		return nil, err
	}
	p.pkgMap = make(map[string]*est.Package)
	for _, pkg := range p.pkgs {
		p.pkgMap[pkg.ImportPath] = pkg
	}

	track := make(names.TrackedPackages, len(defaultTrackedPackages))
	for pkgPath, name := range defaultTrackedPackages {
		track[pkgPath] = name
	}
	p.resolveNames(track)
	p.parseServices()
	p.parseResources()
	p.parseResourceUsage()
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
		ModulePath:   p.cfg.ModulePath,
		Packages:     p.pkgs,
		Services:     p.svcs,
		CronJobs:     p.jobs,
		PubSubTopics: p.pubSubTopics,
		Decls:        p.decls,
		AuthHandler:  p.authHandler,
	}
	md, nodes, err := ParseMeta(p.cfg.AppRevision, p.cfg.AppHasUncommittedChanges, p.cfg.AppRoot, app)
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

// encoreBuildContext creates a build context that mirrors what we pass onto the go compiler once the we trigger a build
// of the application. This allows us to ignore `go` files which would be exlcuded during the build.
//
// For instance if a file has the directive `//go:build !encore` in it
func encoreBuildContext() build.Context {
	buildContext := build.Default
	buildContext.ToolTags = append(buildContext.ToolTags, "encore")

	return buildContext
}

// collectPackages collects and parses the regular Go AST
// for all subdirectories in the root.
func collectPackages(fs *token.FileSet, rootDir, rootImportPath string, mode goparser.Mode, parseTests bool) ([]*est.Package, error) {
	var pkgs []*est.Package
	var errors scanner.ErrorList
	filter := func(f os.FileInfo) bool {
		return parseTests || !strings.HasSuffix(f.Name(), "_test.go")
	}

	buildContext := encoreBuildContext()

	err := walkDirs(rootDir, func(dir, relPath string, files []os.FileInfo) error {
		ps, pkgFiles, err := parseDir(buildContext, fs, dir, relPath, filter, mode)
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
			if n == 2 && pkgNames[1] == (pkgNames[0]+"_test") {
				// It's just a "_test" package; we're good.
			} else {
				namestr := strings.Join(pkgNames[:n-1], ", ") + " and " + pkgNames[n-1]
				errors.Add(fs.Position(first.Pos()), "got multiple package names in directory: "+namestr)
				return nil
			}
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
	p.names = make(names.Application)

	for _, pkg := range p.pkgs {
		track[pkg.ImportPath] = pkg.Name
	}
	for _, pkg := range p.pkgs {
		res, err := names.Resolve(p.fset, track, pkg)
		if err != nil {
			if el, ok := err.(*errlist.List); ok {
				p.errors.Merge(el)
			} else {
				p.err(pkg.Files[0].AST.Pos(), err.Error())
			}
			continue
		}
		p.names[pkg] = res
	}
	if p.errors.Len() > 0 {
		p.errors.Abort()
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

	// For all resources defined, store them in a map per package for faster lookup
	resourceMap := make(map[string]map[string]est.Resource, len(p.pkgs)) // path -> name -> resource
	for _, pkg := range p.pkgs {
		resources := make(map[string]est.Resource, len(pkg.Resources))
		resourceMap[pkg.ImportPath] = resources
		for _, res := range pkg.Resources {
			id := res.Ident()
			resources[id.Name] = res
		}
	}

	for _, pkg := range p.pkgs {
		for _, file := range pkg.Files {
			info := p.names[pkg].Files[file]
			// Find all references to RPCs and objects to rewrite that are not
			// part of rewritten calls.
			astutil.Apply(file.AST, func(c *astutil.Cursor) bool {
				node := c.Node()
				if path, obj := pkgObj(info, node); path != "" {
					pkg2, ok := p.pkgMap[path]
					if !ok {
						return true
					}

					// Is it an RPC?
					if rpc := rpcMap[path][obj]; rpc != nil {
						file.References[node] = &est.Node{
							Type: est.RPCRefNode,
							RPC:  rpc,
						}
						return true
					} else if res := resourceMap[path][obj]; res != nil {
						switch res.Type() {
						case est.SQLDBResource:
							file.References[node] = &est.Node{
								Type: est.SQLDBNode,
								Res:  res,
							}
						}
					}

					if h := p.authHandler; h != nil && path == h.Svc.Root.ImportPath && obj == h.Name {
						p.errf(node.Pos(), "cannot reference auth handler %s.%s from another package", h.Svc.Root.RelPath, obj)
						return false
					}

					switch path {
					case sqldbImportPath:
						// Allow types to be references
						switch obj {
						case "ExecResult", "Tx", "Row", "Rows":
							return true
						default:
							p.errf(node.Pos(), "cannot reference func %s.%s without calling it", path, obj)
							return false
						}
					case rlogImportPath:
						// Allow types to be whitelisted
						switch obj {
						case "Ctx":
							return true
						default:
							p.errf(node.Pos(), "cannot reference func %s.%s without calling it", path, obj)
							return false
						}
					}

					// Is it a type in a different service?
					if pkg2.Service != nil && !(pkg.Service != nil && pkg.Service.Name == pkg2.Service.Name) {
						if decl, ok := p.names[pkg2].Decls[obj]; ok {
							switch decl.Type {
							case token.CONST:
								// all good
							case token.TYPE:
								// TODO check if there are methods on the type
							case token.VAR:
								// p.errf(node.Pos(), "cannot reference variable %s.%s from outside the service", pkg2.RelPath, obj)
								// return false
							}
						}
					}
				} else if id, ok := node.(*ast.Ident); ok {
					// Have we rewritten this already?
					if file.References[c.Parent()] != nil {
						return true
					}
					ri := info.Idents[id]
					// Is it a package-local rpc?
					if ri != nil && ri.Package {
						if rpc := rpcMap[pkg.ImportPath][id.Name]; rpc != nil {
							file.References[id] = &est.Node{
								Type: est.RPCRefNode,
								RPC:  rpc,
							}
						}
					}

				}
				return true
			}, nil)

			if pkg.Service == nil && len(file.References) > 0 {
				for astNode, node := range file.References {
					switch node.Type {
					case est.RPCRefNode:
						rpc := node.RPC
						p.errf(astNode.Pos(), "cannot reference API %s.%s outside of a service\n\tpackage %s is not considered a service (it has no APIs or pubsub subscribers defined)", rpc.Svc.Name, rpc.Name, pkg.Name)
					case est.SQLDBNode:
						// sqldb calls are allowed outside of services
					case est.RLogNode:
						// rlog calls are allowed outside of services
					case est.PubSubTopicDefNode:
						// pubsub topic definitions are allowed outside of services
					default:
						p.errf(astNode.Pos(), "invalid reference outside of a service\n\tpackage %s is not considered a service (it has no APIs or pubsub subscribers defined)", pkg.Name)
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

	// Error if resources are defined in non-services
	for _, pkg := range p.pkgs {
		if pkg.Service == nil {
			for _, res := range pkg.Resources {
				resType := ""
				switch res.Type() {
				case est.SQLDBResource:
					resType = "SQL Database"
				case est.PubSubTopicResource:
					// we do allow pubsub to be declared outside of a service package
					continue
				default:
					panic(fmt.Sprintf("unsupported resource type %v", res.Type()))
				}
				p.errf(res.Ident().Pos(), "cannot define %s resource in non-service package", resType)
			}
		}
		for _, f := range pkg.Files {
			for node, ref := range f.References {
				if res := ref.Res; ref.Res != nil {
					if ff := res.File(); ff.Pkg.Service != nil && (pkg.Service == nil || pkg.Service.Name != ff.Pkg.Service.Name) {
						p.errf(node.Pos(), "cannot reference resource %s.%s outside the service", ff.Pkg.Name, res.Ident().Name)
					}
				}
			}
		}
	}

	// Error if APIs are referenced but not called in non-permissible locations.
	for _, pkg := range p.pkgs {
		for _, f := range pkg.Files {
			astutil.Apply(f.AST, func(c *astutil.Cursor) bool {
				node := c.Node()
				if ref, ok := f.References[node]; ok && ref.Type == est.RPCRefNode && !p.validRPCReferences[node] {
					if _, isCall := c.Parent().(*ast.CallExpr); !isCall {
						rpc := ref.RPC
						p.errf(node.Pos(), "cannot reference API endpoint %s.%s without calling it", rpc.Svc.Name, rpc.Name)
					}
				}
				return true
			}, nil)
		}
	}
}

// pkgObj attempts to unpack a node as a reference to a package obj, returning the
// package path and object name if resolvable.
// If the node is not an *ast.SelectorExpr or it doesn't reference a tracked package,
// it reports "", "".
func pkgObj(info *names.File, node ast.Node) (pkgPath, objName string) {
	if sel, ok := node.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			ri := info.Idents[id]
			if ri != nil && ri.ImportPath != "" {
				return ri.ImportPath, sel.Sel.Name
			}
		}
	}
	return "", ""
}
