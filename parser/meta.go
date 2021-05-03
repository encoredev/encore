package parser

import (
	"fmt"
	"go/ast"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"encr.dev/parser/est"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

type TraceNodes map[ast.Node]*meta.TraceNode

// ParseMeta parses app metadata.
func ParseMeta(version, appRoot string, app *est.Application) (*meta.Data, map[*est.Package]TraceNodes, error) {
	data := &meta.Data{
		ModulePath: app.ModulePath,
		AppVersion: version,
		Decls:      app.Decls,
	}
	pkgMap := make(map[string]*meta.Package)
	nodes := parceTraceNodes(app)
	for _, pkg := range app.Packages {
		p := &meta.Package{
			RelPath: pkg.RelPath,
			Name:    pkg.Name,
			Doc:     pkg.Doc,
			Secrets: pkg.Secrets,
		}
		if pkg.Service != nil {
			p.ServiceName = pkg.Service.Name
		}
		tx := nodes[pkg]
		for _, n := range tx {
			p.TraceNodes = append(p.TraceNodes, n)
		}
		sort.Slice(p.TraceNodes, func(i, j int) bool { return p.TraceNodes[i].Id < p.TraceNodes[j].Id })
		data.Pkgs = append(data.Pkgs, p)
		pkgMap[p.RelPath] = p
	}

	for _, svc := range app.Services {
		s, err := parseSvc(appRoot, svc)
		if err != nil {
			return nil, nil, err
		}
		data.Svcs = append(data.Svcs, s)
	}

	// Populate rpc calls based on rewrite information
	for _, pkg := range app.Packages {
		type key struct {
			pkg string
			rpc string
		}
		seen := make(map[key]bool)
		for _, f := range pkg.Files {
			for _, r := range sortedRefs(f.References) {
				if r.Node.Type == est.RPCCallNode {
					k := key{pkg: r.Node.RPC.Svc.Root.RelPath, rpc: r.Node.RPC.Name}
					if !seen[k] {
						p := pkgMap[pkg.RelPath]
						p.RpcCalls = append(p.RpcCalls, &meta.QualifiedName{
							Pkg:  k.pkg,
							Name: k.rpc,
						})
						seen[k] = true
					}
				}
			}
		}
	}

	if app.AuthHandler != nil {
		data.AuthHandler = parseAuthHandler(app.AuthHandler)
	}

	return data, nodes, nil
}

var migrationRe = regexp.MustCompile(`^([0-9]+)_([^.]+)\.up.sql$`)

func parseSvc(appRoot string, svc *est.Service) (*meta.Service, error) {
	s := &meta.Service{
		Name:    svc.Name,
		RelPath: svc.Root.RelPath,
	}
	for _, rpc := range svc.RPCs {
		proto := meta.RPC_REGULAR
		if rpc.Raw {
			proto = meta.RPC_RAW
		}
		var accessType meta.RPC_AccessType
		switch rpc.Access {
		case est.Public:
			accessType = meta.RPC_PUBLIC
		case est.Private:
			accessType = meta.RPC_PRIVATE
		case est.Auth:
			accessType = meta.RPC_AUTH
		default:
			return nil, fmt.Errorf("unhandled access type %v", rpc.Access)
		}

		var req, resp *schema.Decl
		if rpc.Request != nil {
			req = rpc.Request.Decl
		}
		if rpc.Response != nil {
			resp = rpc.Response.Decl
		}
		r := &meta.RPC{
			Name:           rpc.Name,
			ServiceName:    rpc.Svc.Name,
			Doc:            rpc.Doc,
			AccessType:     accessType,
			RequestSchema:  req,
			ResponseSchema: resp,
			Proto:          proto,
			Loc:            parseLoc(rpc.File, rpc.Func),
		}
		s.Rpcs = append(s.Rpcs, r)
	}

	relPath := filepath.Join(svc.Root.RelPath, "migrations")
	migs, err := parseMigrations(appRoot, relPath)
	if err != nil {
		return nil, fmt.Errorf("%s: could not parse sqldb migrations: %v", svc.Root.RelPath, err)
	}
	s.Migrations = migs
	return s, nil
}

func parseMigrations(appRoot, relPath string) ([]*meta.DBMigration, error) {
	absPath := filepath.Join(appRoot, relPath)
	fi, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", relPath)
	}

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("could not read migrations: %v", err)
	}
	migrations := make([]*meta.DBMigration, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		match := migrationRe.FindStringSubmatch(f.Name())
		if match == nil {
			return nil, fmt.Errorf("migration %s/%s has an invalid name (must be of the format '123_description_here.up.sql')",
				relPath, f.Name())
		}
		num, _ := strconv.Atoi(match[1])
		migrations = append(migrations, &meta.DBMigration{
			Filename:    f.Name(),
			Number:      int32(num),
			Description: match[2],
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Number < migrations[j].Number
	})
	for i := int32(0); i < int32(len(migrations)); i++ {
		fn := migrations[i].Filename
		num := migrations[i].Number
		if num <= 0 {
			return nil, fmt.Errorf("%s/%s: invalid migration number %d", relPath, fn, num)
		} else if num < (i + 1) {
			return nil, fmt.Errorf("%s/%s: duplicate migration with number %d", relPath, fn, num)
		} else if num > (i + 1) {
			return nil, fmt.Errorf("%s/%s: missing migration with number %d", relPath, fn, i+1)
		}
	}
	return migrations, nil
}

func parseAuthHandler(h *est.AuthHandler) *meta.AuthHandler {
	pb := &meta.AuthHandler{
		Name:    h.Name,
		Doc:     h.Name,
		PkgPath: h.Svc.Root.ImportPath,
		PkgName: h.Svc.Root.Name,
		Loc:     parseLoc(h.File, h.Func),
	}
	if h.AuthData != nil {
		pb.AuthData = h.AuthData.Decl
	}
	return pb
}

func parceTraceNodes(app *est.Application) map[*est.Package]TraceNodes {
	var id int32
	res := make(map[*est.Package]TraceNodes)
	for _, pkg := range app.Packages {
		nodes := make(TraceNodes)
		res[pkg] = nodes
		for _, file := range pkg.Files {
			for _, r := range sortedRefs(file.References) {
				switch r.Node.Type {
				// Secret nodes are not relevant for tracing
				case est.SecretsNode:
					continue
				}

				tx := newTraceNode(&id, pkg, file, r.AST)
				nodes[r.AST] = tx
				start := file.Token.Offset(r.AST.Pos())
				end := file.Token.Offset(r.AST.End())

				switch r.Node.Type {
				case est.RPCCallNode:
					tx.Context = &meta.TraceNode_RpcCall{
						RpcCall: &meta.RPCCallNode{
							ServiceName: r.Node.RPC.Svc.Name,
							RpcName:     r.Node.RPC.Name,
							Context:     string(file.Contents[start:end]),
						},
					}

				case est.SQLDBNode:
					tx.Context = &meta.TraceNode_StaticCall{
						StaticCall: &meta.StaticCallNode{
							Package: meta.StaticCallNode_SQLDB,
							Func:    r.Node.Func,
							Context: string(file.Contents[start:end]),
						},
					}

				case est.RLogNode:
					tx.Context = &meta.TraceNode_StaticCall{
						StaticCall: &meta.StaticCallNode{
							Package: meta.StaticCallNode_RLOG,
							Func:    r.Node.Func,
							Context: string(file.Contents[start:end]),
						},
					}
				}
			}
		}
	}

	for _, svc := range app.Services {
		for _, rpc := range svc.RPCs {
			fd := rpc.Func
			tx := newTraceNode(&id, rpc.Svc.Root, rpc.File, fd)
			res[svc.Root][fd] = tx
			f := rpc.File
			start := f.Token.Offset(fd.Type.Pos())
			end := f.Token.Offset(fd.Type.End())
			tx.Context = &meta.TraceNode_RpcDef{
				RpcDef: &meta.RPCDefNode{
					ServiceName: svc.Name,
					RpcName:     fd.Name.Name,
					Context:     string(f.Contents[start:end]),
				},
			}
		}
	}

	if h := app.AuthHandler; h != nil {
		fd := h.Func
		tx := newTraceNode(&id, h.Svc.Root, h.File, fd)
		res[h.Svc.Root][fd] = tx
		f := h.File
		start := f.Token.Offset(fd.Type.Pos())
		end := f.Token.Offset(fd.Type.End())
		tx.Context = &meta.TraceNode_AuthHandlerDef{
			AuthHandlerDef: &meta.AuthHandlerDefNode{
				ServiceName: h.Svc.Name,
				Name:        fd.Name.Name,
				Context:     string(f.Contents[start:end]),
			},
		}
	}

	return res
}

func newTraceNode(id *int32, pkg *est.Package, f *est.File, node ast.Node) *meta.TraceNode {
	*id++
	var expr *meta.TraceNode
	file := f.Token
	filename := filepath.Base(f.Path)
	filepath := path.Join(pkg.RelPath, filename)

	switch node := node.(type) {
	case *ast.CallExpr:
		expr = &meta.TraceNode{
			Id:       *id,
			Filepath: filepath,
			StartPos: int32(file.Offset(node.Lparen)),
			EndPos:   int32(file.Offset(node.Rparen)),
		}
	case *ast.FuncDecl:
		expr = &meta.TraceNode{
			Id:       *id,
			Filepath: filepath,
			StartPos: int32(file.Offset(node.Type.Pos())),
			EndPos:   int32(file.Offset(node.Type.End())),
		}
	default:
		panic(fmt.Sprintf("unhandled trace expression node %T", node))
	}

	start := file.Pos(int(expr.StartPos))
	end := file.Pos(int(expr.EndPos))
	sPos, ePos := file.Position(start), file.Position(end)
	expr.SrcLineStart = int32(sPos.Line)
	expr.SrcLineEnd = int32(ePos.Line)
	expr.SrcColStart = int32(sPos.Column)
	expr.SrcColEnd = int32(ePos.Column)
	return expr
}

func qualified(pkg *est.Package, name string) *meta.QualifiedName {
	return &meta.QualifiedName{
		Pkg:  pkg.RelPath,
		Name: name,
	}
}

func parseLoc(f *est.File, node ast.Node) *schema.Loc {
	sPos, ePos := f.Token.Position(node.Pos()), f.Token.Position(node.Pos())
	return &schema.Loc{
		PkgName:      f.Pkg.Name,
		PkgPath:      f.Pkg.ImportPath,
		Filename:     f.Name,
		StartPos:     int32(f.Token.Offset(node.Pos())),
		EndPos:       int32(f.Token.Offset(node.End())),
		SrcLineStart: int32(sPos.Line),
		SrcLineEnd:   int32(ePos.Line),
		SrcColStart:  int32(sPos.Column),
		SrcColEnd:    int32(ePos.Column),
	}
}

type refPair struct {
	AST  ast.Node
	Node *est.Node
}

func sortedRefs(refs map[ast.Node]*est.Node) []refPair {
	p := make([]refPair, 0, len(refs))
	for ast, node := range refs {
		p = append(p, refPair{ast, node})
	}
	sort.Slice(p, func(i, j int) bool {
		return p[i].AST.Pos() < p[j].AST.Pos()
	})
	return p
}
