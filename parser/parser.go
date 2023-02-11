// Package parser parses Encore applications into an Encore Syntax Tree (EST).
package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/ast/astutil"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/paths"
	"encr.dev/parser/selector"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/experiments"
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
	fset                *token.FileSet
	errors              *errlist.List
	pkgs                []*est.Package
	pkgMap              map[string]*est.Package // import path -> pkg
	svcs                []*est.Service
	jobs                []*est.CronJob
	svcMap              map[string]*est.Service // name -> svc
	svcPkgPaths         map[string]*est.Service // pkg path -> svc
	jobsMap             map[string]*est.CronJob // ID -> job
	pubSubTopics        []*est.PubSubTopic
	cacheClusters       []*est.CacheCluster
	names               names.Application
	authHandler         *est.AuthHandler
	middleware          []*est.Middleware
	metrics             []*est.Metric
	declMap             map[string]*schema.Decl // pkg/path.Name -> decl
	decls               []*schema.Decl
	paths               paths.Set                          // RPC paths
	resourceMap         map[string]map[string]est.Resource // pkg/path -> name -> resource
	hasUnexportedFields map[*schema.Struct]*ast.Field      // A struct will be in this map if it has unexported fields

	// validRPCReferences is a set of ast nodes that are allowed to
	// reference RPCs without calling them.
	validRPCReferences map[ast.Node]bool

	// schema types -> ast.Node mappings (used for errors)
	schemaToAST map[any]ast.Node
}

// Config represents the configuration options for parsing.
// TODO(domblack): Remove AppRevision and AppHasUncommittedChanges from here as it's compiler concern not a parser concern
type Config struct {
	AppRoot                  string
	Experiments              *experiments.Set
	AppRevision              string
	AppHasUncommittedChanges bool
	ModulePath               string
	WorkingDir               string
	ParseTests               bool

	// ScriptMainPkg specifies the relative path to the main package,
	// when running in script mode. It's used to mark that package
	// as a synthetic "main" service.
	ScriptMainPkg string
}

func Parse(cfg *Config) (*Result, error) {
	p := &parser{
		cfg:                 cfg,
		declMap:             make(map[string]*schema.Decl),
		validRPCReferences:  make(map[ast.Node]bool),
		jobsMap:             make(map[string]*est.CronJob),
		resourceMap:         make(map[string]map[string]est.Resource),
		hasUnexportedFields: make(map[*schema.Struct]*ast.Field),
		schemaToAST:         make(map[any]ast.Node),
	}
	return p.Parse()
}

const (
	sqldbImportPath = "encore.dev/storage/sqldb"
	rlogImportPath  = "encore.dev/rlog"
	uuidImportPath  = "encore.dev/types/uuid"
	authImportPath  = "encore.dev/beta/auth"
	cronImportPath  = "encore.dev/cron"
	testImportPath  = "encore.dev/et"
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
			p.errors.Report(srcerrors.UnhandledPanic(e))
		}

		if err == nil {
			p.errors.MakeRelative(p.cfg.AppRoot, p.cfg.WorkingDir)
			err = p.errors.Err()
		}
	}()
	p.fset = token.NewFileSet()
	p.errors = errlist.New(p.fset)

	p.pkgs, err = collectPackages(p.fset, p.cfg.AppRoot, p.cfg.ModulePath, p.cfg.ScriptMainPkg, goparser.ParseComments, p.cfg.ParseTests, experiments.NoAPI.Enabled(p.cfg.Experiments))
	if err != nil {
		if errList, ok := err.(scanner.ErrorList); ok {
			p.errors.Report(errList)
			return nil, p.errors
		}
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
	p.validateMiddleware()
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
		ModulePath:    p.cfg.ModulePath,
		Packages:      p.pkgs,
		Services:      p.svcs,
		CronJobs:      p.jobs,
		PubSubTopics:  p.pubSubTopics,
		CacheClusters: p.cacheClusters,
		Decls:         p.decls,
		AuthHandler:   p.authHandler,
		Middleware:    p.middleware,
		Metrics:       p.metrics,
	}

	md, nodes, err := ParseMeta(p.cfg.AppRevision, p.cfg.AppHasUncommittedChanges, p.cfg.AppRoot, app, p.fset, p.cfg.Experiments)
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
//
// Main packages are ignored by default, except for mainPkgRelPath if set.
func collectPackages(fset *token.FileSet, rootDir, rootImportPath, mainPkgRelPath string, mode goparser.Mode, parseTests bool, allowMainPkg bool) ([]*est.Package, error) {
	var pkgs []*est.Package
	var errors scanner.ErrorList
	filter := func(f fs.DirEntry) bool {
		// Don't parse encore.gen.go files, since they're not intended to be checked in.
		// We've had several issues where things work locally but not in CI/CD because
		// the encore.gen.go file was parsed for local development which papered over issues.
		if strings.Contains(f.Name(), "encore.gen.go") {
			return false
		}

		return parseTests || !strings.HasSuffix(f.Name(), "_test.go")
	}

	buildContext := encoreBuildContext()

	parsePkg := func(dir, relPath string, files []fs.DirEntry) (*est.Package, error) {
		ps, pkgFiles, err := parseDir(buildContext, fset, dir, files, filter, mode)
		if err != nil {
			return nil, err
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
				errors.Add(fset.Position(first.Pos()), "got multiple package names in directory: "+namestr)
				return nil, nil
			}
		} else if n == 0 {
			// No Go files; ignore directory
			return nil, nil
		}

		p := ps[pkgNames[0]]
		var doc string
		for _, astFile := range p.Files {
			// HACK: getting package comments is not at all easy
			// because of the quirks of go/ast. This seems to work.
			cm := ast.NewCommentMap(fset, astFile, astFile.Comments)
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
			Imports:    make(map[string]bool),
		}

		// Ignore main packages (they're scripts) unless we're executing that very main package
		// as an exec script.
		if pkg.Name == "main" && pkg.RelPath != mainPkgRelPath && !allowMainPkg {
			return nil, nil
		}

		for _, f := range pkgFiles {
			f.Pkg = pkg

			for importPath := range f.Imports {
				pkg.Imports[importPath] = true
			}
		}
		return pkg, nil
	}

	type dirToParse struct {
		dir     string
		relPath string
		files   []fs.DirEntry
	}

	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers < 4 {
		numWorkers = 4
	}

	work := make(chan dirToParse, 100)
	pkgCh := make(chan *est.Package, numWorkers)
	errCh := make(chan error, numWorkers)
	quit := make(chan struct{})
	workerDone := make(chan struct{}, numWorkers)

	worker := func() {
		defer func() { workerDone <- struct{}{} }()
		for d := range work {
			pkg, err := parsePkg(d.dir, d.relPath, d.files)
			if err != nil {
				errCh <- err
				return
			}
			if pkg != nil {
				pkgCh <- pkg
			}
		}
	}

	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	go func() {
		err := walkDirs(rootDir, func(dir, relPath string, files []fs.DirEntry) error {
			work <- dirToParse{dir: dir, relPath: relPath, files: files}
			return nil
		})
		close(work) // no more work
		if err != nil {
			errCh <- err
		}
	}()

	defer close(quit)
	numWorkersDone := 0
	for numWorkersDone < numWorkers {
		select {
		case pkg := <-pkgCh:
			pkgs = append(pkgs, pkg)
		case err := <-errCh:
			// If the error is an error list, it means we have a parsing error.
			// Keep going in that case.
			if el, ok := err.(scanner.ErrorList); ok {
				errors = append(errors, el...)
			} else {
				return nil, err
			}

		case <-workerDone:
			numWorkersDone++
		}
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].RelPath < pkgs[j].RelPath
	})
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
			if id != nil {
				resources[id.Name] = res
			}
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

					// Are we referencing a service struct from another service?
					if pkg2.Service != nil && pkg.Service != nil && pkg2.Service != pkg.Service &&
						pkg2.Service.Struct != nil && obj == pkg2.Service.Struct.Name {
						if !strings.HasSuffix(file.Name, "_test.go") {
							p.errf(node.Pos(), "cannot reference encore:service struct type %s.%s from another service",
								pkg2.Name, obj)
							return false
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
					case est.CacheClusterDefNode:
						// cache cluster definitions are allowed outside of services
					case est.PubSubPublisherNode:
						// we verify this inside the pubsub publisher parser
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

	// Validate types which get marshaled outside the app (RPC or PubSub)
	// don't use the config Value types
	for _, svc := range p.svcs {
		for _, rpc := range svc.RPCs {
			if rpc.Request != nil {
				p.validateTypeDoesntUseConfigTypes(rpc.Func.Pos(), rpc.Request)
			}
			if rpc.Response != nil {
				p.validateTypeDoesntUseConfigTypes(rpc.Func.Pos(), rpc.Response)
			}
		}
	}
	for _, topic := range p.pubSubTopics {
		p.validateTypeDoesntUseConfigTypes(topic.Ident().Pos(), topic.MessageType)
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

				pos := token.NoPos
				if id := res.Ident(); id != nil {
					pos = id.Pos()
				} else {
					pos = res.DefNode().Pos()
				}
				p.errf(pos, "cannot define %s resource in non-service package", resType)
			}
		}

		for _, f := range pkg.Files {
			if !strings.HasSuffix(f.Name, "_test.go") {
				for _, imp := range f.AST.Imports {
					if strings.Contains(imp.Path.Value, testImportPath) {
						p.err(imp.Pos(), "Encore's test packages can only be used inside tests and cannot otherwise be imported.")
					}
				}
			}

			for node, ref := range f.References {
				if res := ref.Res; ref.Res != nil &&
					res.Type() != est.PubSubTopicResource { // PubSub topics are allow to be published to in global middleware, so it's ok to reference a topic outside a service
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
				if ref, ok := f.References[node]; ok && ref.Type == est.RPCRefNode {
					rpc := ref.RPC
					if !p.validRPCReferences[node] {
						if call, isCall := c.Parent().(*ast.CallExpr); !isCall || call.Fun != node {
							p.errf(node.Pos(), "cannot reference API endpoint %s.%s without calling it", rpc.Svc.Name, rpc.Name)
						}
					}
					if rpc.Raw {
						p.errf(node.Pos(), "calling raw API endpoint %s.%s from another endpoint is not yet supported",
							rpc.Svc.Name, rpc.Name)
					}
				}
				return true
			}, nil)
		}
	}

	p.validateCacheKeyspacePathConflicts()
	p.validateConfigTypes()
}

func (p *parser) validateTypeDoesntUseConfigTypes(pos token.Pos, param *est.Param) {
	err := schema.Walk(p.decls, param.Type, func(n any) error {
		if _, ok := n.(*schema.ConfigValue); ok {
			return errors.New("config.Value")
		}
		return nil
	})
	if err != nil {
		p.errf(pos, "type %s can only be used in data types used by config.Load and cannot be used for API response/response parameters or PubSub message types", err.Error())
	}
}

func (p *parser) rpcForName(pkgPath, objName string) (*est.RPC, bool) {
	if svc, found := p.svcPkgPaths[pkgPath]; found {
		for _, rpc := range svc.RPCs {
			if rpc.Name == objName {
				return rpc, true
			}
		}
	}

	return nil, false
}

// resolveRPCRef resolves an expression as a reference to an RPC.
// It must either be in the form "svc.RPC" (if different service)
// or "RPC", "Service.RPC" or "(*Service).RPC" if within a service.
func (p *parser) resolveRPCRef(file *est.File, expr ast.Expr) (*est.RPC, bool) {
	// Simple case: just "RPC"
	if ident, ok := expr.(*ast.Ident); ok {
		pkgPath, objName, _ := p.names.PackageLevelRef(file, expr)
		if pkgPath != "" {
			return p.rpcForName(pkgPath, objName)
		} else {
			// On first compiles of an Encore app using service structs the `encore.gen.go` file has not been created yet.
			// which means during the parsers name resolution phase, a reference of `Blah()` on `(*Service).Blah` would
			// not be resolved to any RPC. However after the generation of `encore.gen.go` the reference would be resolved.
			//
			// As such if `pkgPath` is blank, it means name resolution failed, so we'll check if there is an RPC on the
			// service struct with that name, if there is we'll return that one.
			//
			// Note we can't first generate `encore.gen.go` as it needs a successful parse output to generate, which we
			// can't do without either having `encore.gen.go` or putting in this work around.
			if file.Pkg.Service != nil && file.Pkg.Service.Struct != nil {
				for _, rpc := range file.Pkg.Service.RPCs {
					if rpc.Name == ident.Name {
						return rpc, true
					}
				}
			}
		}
		return nil, false
	}

	// See if it's "othersvc.RPC"
	{
		pkgPath, objName, _ := p.names.PackageLevelRef(file, expr)
		if pkgPath != "" {
			return p.rpcForName(pkgPath, objName)
		}
	}

	// Finally it might be "Service.RPC" where "Service" is an API Group.
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	endpointName := sel.Sel.Name

	// Unwrap "(*Service)", if necessary
	groupExpr := sel.X
	if paren, ok := groupExpr.(*ast.ParenExpr); ok {
		groupExpr = paren.X
	}
	if ptr, ok := groupExpr.(*ast.StarExpr); ok {
		groupExpr = ptr.X
	}

	pkgPath, objName, _ := p.names.PackageLevelRef(file, groupExpr)
	if pkgPath != file.Pkg.ImportPath {
		// Refers to a different package; API group references must be within
		// the same package.
		return nil, false
	}

	// Resolve the "Service" in "Service.RPC" to an API Group.
	svc, ok := p.svcPkgPaths[pkgPath]
	if !ok {
		// Not a service, so doesn't contain a service struct
		return nil, false
	}
	var ss *est.ServiceStruct
	if svc.Struct != nil && svc.Struct.Name == objName {
		ss = svc.Struct
	} else {
		return nil, false
	}

	for _, rpc := range ss.RPCs {
		if rpc.Name == endpointName {
			return rpc, true
		}
	}
	return nil, false
}

// validateMiddleware validates the middleware and sorts them in file and line order,
// to ensure that middleware runs in the order they were defined.
func (p *parser) validateMiddleware() {
	// Find which tags are used so we can error for unknown tags
	type svcTag struct{ svc, tag string }
	globalTags := make(map[string]bool)
	svcTags := make(map[svcTag]bool)
	for _, svc := range p.svcs {
		for _, rpc := range svc.RPCs {
			for _, tag := range rpc.Tags {
				globalTags[tag.Value] = true
				svcTags[svcTag{svc: svc.Name, tag: tag.Value}] = true
			}
		}
	}

	// Ensure global middleware are not defined in service packages, and vice versa.
	for _, mw := range p.middleware {
		// We can't check mw.Svc as during parsing it wasn't yet clear if a package was a service or not,
		// so it's nil iff Global is false.
		if mw.Global && mw.Pkg.Service != nil {
			p.errf(mw.Func.Pos(), "cannot define global middleware within a service package\n"+
				"\tNote: since global middleware applies to all services it must be defined outside\n"+
				"\tof any particular service.")
		} else if !mw.Global && mw.Pkg.Service == nil {
			p.errf(mw.Func.Pos(), "cannot define non-global middleware outside of a service\n"+
				"\tNote: package %s is not a service. To define a global middleware,\n"+
				"\tspecify //encore:middleware global", mw.Pkg.Name)
		}

		// Make sure the middleware is targeting tags that actually exist
		for _, sel := range mw.Target {
			if sel.Type == selector.Tag {
				if mw.Global {
					if !globalTags[sel.Value] {
						p.errf(mw.Func.Pos(), "undefined tag (no API in the application defines this tag): %s",
							sel.String())
					}
				} else {
					if !svcTags[svcTag{svc: mw.Svc.Name, tag: sel.Value}] {
						p.errf(mw.Func.Pos(), "undefined tag (no API in the %s service defines this tag): %s",
							mw.Svc.Name, sel.String())
					}
				}
			}
		}
	}

	// Sort the middleware in file and column order
	sortFn := func(a, b *est.Middleware) bool {
		// Globals come first, then group by package
		if a.Global != b.Global {
			return a.Global
		} else if a.Pkg != b.Pkg {
			return a.Pkg.RelPath < b.Pkg.RelPath
		}

		posA := p.fset.Position(a.Func.Pos())
		posB := p.fset.Position(b.Func.Pos())

		// Sort by filename, then line, then column
		if posA.Filename != posB.Filename {
			return posA.Filename < posB.Filename
		} else if posA.Line != posB.Line {
			return posA.Line < posB.Line
		} else {
			return posA.Column < posB.Column
		}
	}

	// Sort both the overall list and the service-specific lists.
	slices.SortStableFunc(p.middleware, sortFn)
	for _, svc := range p.svcs {
		slices.SortStableFunc(svc.Middleware, sortFn)
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
