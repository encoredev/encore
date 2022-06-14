// Package parser parses Encore applications into an Encore Syntax Tree (EST).
package parser

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	cronparser "github.com/robfig/cron/v3"
	"golang.org/x/tools/go/ast/astutil"

	"encr.dev/parser/dnsname"
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
	jobsMap      map[string]*est.CronJob // ID -> job
	pubSubTopics []*est.PubSubTopic
	names        map[*est.Package]*names.Resolution
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
	sqldbImportPath: "sqldb",
	rlogImportPath:  "rlog",
	uuidImportPath:  "uuid",
	authImportPath:  "auth",
	cronImportPath:  "cron",

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
	p.parseReferences()
	p.parseCronJobs()
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
	p.names = make(map[*est.Package]*names.Resolution)

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
						file.References[node] = &est.Node{
							Type: est.SQLDBNode,
							Res:  res,
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
						case "Tx", "Row", "Rows":
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

func (p *parser) parseCronJobs() {
	p.jobsMap = make(map[string]*est.CronJob)
	cp := cronparser.NewParser(cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow)
	for _, pkg := range p.pkgs {
		for _, file := range pkg.Files {

			// seenCalls tracks the calls we've seen and processed.
			// Any calls to cron.NewJob not in this map is done at
			// an invalid call site.
			seenCalls := make(map[*ast.CallExpr]bool)
			info := p.names[pkg].Files[file]
			for _, decl := range file.AST.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					for _, x := range vs.Values {
						if ce, ok := x.(*ast.CallExpr); ok {
							seenCalls[ce] = true
							if cronJob := p.parseCronJobStruct(cp, ce, file, info); cronJob != nil {
								cronJob.Doc = gd.Doc.Text()
								if cronJob2 := p.jobsMap[cronJob.ID]; cronJob2 != nil {
									p.errf(pkg.AST.Pos(), "cron job %s defined twice", cronJob.ID)
									continue
								}
								p.jobs = append(p.jobs, cronJob)
								p.jobsMap[cronJob.ID] = cronJob
							}
						}
					}
				}
			}

			// Now walk the whole file and catch any calls not already processed; they must be invalid.
			ast.Inspect(file.AST, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok && !seenCalls[call] {
					if imp, obj := pkgObj(info, call.Fun); imp == cronImportPath && obj == "NewJob" {
						p.errf(call.Pos(), "cron job must be defined as a package global variable")
					}
				}
				return true
			})
		}
	}
}

const (
	minute int64 = 60
	hour   int64 = 60 * minute
)

func (p *parser) parseCronJobStruct(cp cronparser.Parser, ce *ast.CallExpr, file *est.File, info *names.File) *est.CronJob {
	if imp, obj := pkgObj(info, ce.Fun); imp == cronImportPath && obj == "NewJob" {
		if len(ce.Args) != 2 {
			p.errf(ce.Pos(), "cron.NewJob must be called as (id string, cfg cron.JobConfig)")
			return nil
		}

		cj := &est.CronJob{}
		if bl, ok := ce.Args[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
			cronJobID, _ := strconv.Unquote(bl.Value)
			if cronJobID == "" {
				p.errf(ce.Pos(), "cron.NewJob: id argument must be a non-empty string literal")
				return nil
			}
			err := dnsname.DNS1035Label(cronJobID)
			if err != nil {
				p.errf(ce.Pos(), "cron.NewJob: id must consist of lower case alphanumeric characters"+
					" or '-',\n// start with an alphabetic character, and end with an alphanumeric character ")
				return nil
			}
			cj.ID = cronJobID
			cj.Title = cronJobID // Set ID as the default title
		} else {
			p.errf(ce.Pos(), "cron.NewJob must be called with a string literal as its first argument")
			return nil
		}

		if cl, ok := ce.Args[1].(*ast.CompositeLit); ok {
			if imp, obj := pkgObj(info, cl.Type); imp == cronImportPath && obj == "JobConfig" {
				hasSchedule := false
				for _, e := range cl.Elts {
					kv := e.(*ast.KeyValueExpr)
					key, ok := kv.Key.(*ast.Ident)
					if !ok {
						p.errf(kv.Pos(), "field must be an identifier")
						return nil
					}
					switch key.Name {
					case "Title":
						if v, ok := kv.Value.(*ast.BasicLit); ok && v.Kind == token.STRING {
							parsed, _ := strconv.Unquote(v.Value)
							cj.Title = parsed
						} else {
							p.errf(v.Pos(), "Title must be a string literal")
							return nil
						}
					case "Every":
						if hasSchedule {
							p.errf(kv.Pos(), "Every: cron execution schedule was already defined using the Schedule field, at least one must be set but not both")
							return nil
						}
						if dur, ok := p.parseCronLiteral(info, kv.Value); ok {
							// We only support intervals that are a positive integer number of minutes.
							if rem := dur % minute; rem != 0 {
								p.errf(kv.Value.Pos(), "Every: must be an integer number of minutes, got %d", dur)
								return nil
							}

							minutes := dur / minute
							if minutes < 1 {
								p.errf(kv.Value.Pos(), "Every: duration must be one minute or greater, got %d", minutes)
								return nil
							} else if minutes > 24*60 {
								p.errf(kv.Value.Pos(), "Every: duration must not be greater than 24 hours (1440 minutes), got %d", minutes)
								return nil
							} else if suggestion, ok := p.isCronIntervalAllowed(int(minutes)); !ok {
								suggestionStr := p.formatMinutes(suggestion)
								minutesStr := p.formatMinutes(int(minutes))
								p.errf(kv.Value.Pos(), "Every: 24 hour time range (from 00:00 to 23:59) "+
									"needs to be evenly divided by the interval value (%s), try setting it to (%s)", minutesStr, suggestionStr)
								return nil
							}
							cj.Schedule = fmt.Sprintf("every:%d", minutes)
							hasSchedule = true
						} else {
							return nil
						}
					case "Schedule":
						if hasSchedule {
							p.errf(kv.Pos(), "cron execution schedule was already defined using the Every field, at least one must be set but not both")
							return nil
						}
						if v, ok := kv.Value.(*ast.BasicLit); ok && v.Kind == token.STRING {
							parsed, _ := strconv.Unquote(v.Value)
							_, err := cp.Parse(parsed)
							if err != nil {
								p.errf(v.Pos(), "Schedule must be a valid cron expression: %s", err)
								return nil
							}
							cj.Schedule = fmt.Sprintf("schedule:%s", parsed)
							hasSchedule = true
						} else {
							p.errf(v.Pos(), "Schedule must be a string literal")
							return nil
						}
					case "Endpoint":
						// This is one of the places where it's fine to reference an RPC endpoint.
						p.validRPCReferences[kv.Value] = true

						if ref, ok := file.References[kv.Value]; ok && ref.Type == est.RPCRefNode {
							cj.RPC = ref.RPC
						} else {
							p.errf(kv.Value.Pos(), "Endpoint does not reference an Encore API")
							return nil
						}
					default:
						p.errf(key.Pos(), "cron.JobConfig has unknown key %s", key.Name)
						return nil
					}
				}

				if _, err := cj.IsValid(); err != nil {
					p.errf(cl.Pos(), "cron.NewJob: %s", err)
				}
				return cj
			}
		}
	}
	return nil
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

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (p *parser) formatMinutes(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d * cron.Minute", minutes)
	} else if minutes%60 == 0 {
		return fmt.Sprintf("%d * cron.Hour", minutes/60)
	}
	return fmt.Sprintf("%d * cron.Hour + %d * cron.Minute", minutes/60, minutes%60)
}

func (p *parser) isCronIntervalAllowed(val int) (suggestion int, ok bool) {
	allowed := []int{
		1, 2, 3, 4, 5, 6, 8, 9, 10, 12, 15, 16, 18, 20, 24, 30, 32, 36, 40, 45,
		48, 60, 72, 80, 90, 96, 120, 144, 160, 180, 240, 288, 360, 480, 720, 1440,
	}
	idx := sort.SearchInts(allowed, val)

	if idx == len(allowed) {
		return allowed[len(allowed)-1], false
	} else if allowed[idx] == val {
		return val, true
	} else if idx == 0 {
		return allowed[0], false
	} else if abs(val-allowed[idx-1]) < abs(val-allowed[idx]) {
		return allowed[idx-1], false
	}

	return allowed[idx], false
}

// parseCronLiteral parses an expression representing a cron duration constant.
// It uses go/constant to perform arbitrary-precision arithmetic according
// to the rules of the Go compiler.
func (p *parser) parseCronLiteral(info *names.File, durationExpr ast.Expr) (dur int64, ok bool) {
	zero := constant.MakeInt64(0)
	var parse func(expr ast.Expr) constant.Value
	parse = func(expr ast.Expr) constant.Value {
		switch x := expr.(type) {
		case *ast.BinaryExpr:
			lhs := parse(x.X)
			rhs := parse(x.Y)
			switch x.Op {
			case token.MUL, token.ADD, token.SUB, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
				return constant.BinaryOp(lhs, x.Op, rhs)
			case token.QUO:
				// constant.BinaryOp panics when dividing by zero
				if constant.Compare(rhs, token.EQL, zero) {
					p.errf(x.Pos(), "cannot divide by zero")
					return constant.MakeUnknown()
				}

				return constant.BinaryOp(lhs, x.Op, rhs)
			default:
				p.errf(x.Pos(), "unsupported operation: %s", x.Op)
				return constant.MakeUnknown()
			}

		case *ast.UnaryExpr:
			val := parse(x.X)
			switch x.Op {
			case token.ADD, token.SUB, token.XOR:
				return constant.UnaryOp(x.Op, val, 0)
			default:
				p.errf(x.Pos(), "unsupported operation: %s", x.Op)
				return constant.MakeUnknown()
			}

		case *ast.BasicLit:
			switch x.Kind {
			case token.INT, token.FLOAT:
				return constant.MakeFromLiteral(x.Value, x.Kind, 0)
			default:
				p.errf(x.Pos(), "unsupported literal in duration expression: %s", x.Kind)
				return constant.MakeUnknown()
			}

		case *ast.CallExpr:
			// We allow "cron.Duration(x)" as a no-op
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Duration" {
				if id, ok := sel.X.(*ast.Ident); ok {
					ri := info.Idents[id]
					if ri != nil && ri.ImportPath == cronImportPath {
						if len(x.Args) == 1 {
							return parse(x.Args[0])
						}
					}
				}
			}
			p.errf(x.Pos(), "unsupported call expression in duration expression")
			return constant.MakeUnknown()

		case *ast.SelectorExpr:
			if pkg, obj := pkgObj(info, x); pkg == cronImportPath {
				var d int64
				switch obj {
				case "Minute":
					d = minute
				case "Hour":
					d = hour
				default:
					p.errf(x.Pos(), "unsupported duration value: %s.%s (expected cron.Minute or cron.Hour)", pkg, obj)
					return constant.MakeUnknown()
				}
				return constant.MakeInt64(d)
			}
			p.errf(x.Pos(), "unexpected value in duration literal")
			return constant.MakeUnknown()

		case *ast.ParenExpr:
			return parse(x.X)

		default:
			p.errf(x.Pos(), "unsupported expression in duration literal: %T", x)
			return constant.MakeUnknown()
		}
	}

	val := constant.Val(parse(durationExpr))
	switch val := val.(type) {
	case int64:
		return val, true
	case *big.Int:
		if !val.IsInt64() {
			p.errf(durationExpr.Pos(), "duration expression out of bounds")
			return 0, false
		}
		return val.Int64(), true
	case *big.Rat:
		num := val.Num()
		if val.IsInt() && num.IsInt64() {
			return num.Int64(), true
		}
		p.errf(durationExpr.Pos(), "floating point numbers are not supported in duration literals")
		return 0, false
	case *big.Float:
		p.errf(durationExpr.Pos(), "floating point numbers are not supported in duration literals")
		return 0, false
	default:
		p.errf(durationExpr.Pos(), "unsupported duration literal")
		return 0, false
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
