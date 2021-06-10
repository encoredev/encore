package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
)

// parseFeatures parses the application packages looking for Encore features
// such as RPCs and auth handlers, and computes the set of services.
func (p *parser) parseServices() {
	svcPaths := make(map[string]*est.Service) // import path -> *Service

	// First determine which packages are considered services based on
	// whether they define RPCs.
	p.svcMap = make(map[string]*est.Service)
	for _, pkg := range p.pkgs {
		// svc is a candidate service; if we don't find any
		// rpcs it is discarded.
		svc := &est.Service{
			Name: pkg.Name,
			Root: pkg,
			Pkgs: []*est.Package{pkg},
		}
		if isSvc := p.parseFuncs(pkg, svc); !isSvc {
			continue
		}
		pkg.Service = svc
		svcPaths[pkg.ImportPath] = svc
		if svc2 := p.svcMap[svc.Name]; svc2 != nil {
			p.errf(pkg.AST.Pos(), "service %s defined twice (previous definition at %s)",
				svc.Name, p.fset.Position(svc2.Root.Files[0].AST.Pos()))
			continue
		}
		p.svcs = append(p.svcs, svc)
		p.svcMap[svc.Name] = svc
	}

PkgLoop:
	for _, pkg := range p.pkgs {
		// Determine which service this pkg belongs to, if any
		path := pkg.ImportPath
		for {
			idx := strings.LastIndexByte(path, '/')
			if idx < 0 {
				break
			}
			path = path[:idx]
			if svc := svcPaths[path]; svc != nil {
				if svcPaths[pkg.ImportPath] != nil {
					// This pkg is a service, but it's nested within another service
					p.errf(pkg.Files[0].AST.Pos(), "cannot nest service %s within service %s", pkg.Name, svc.Name)
					continue PkgLoop
				}
				pkg.Service = svc
				svc.Pkgs = append(svc.Pkgs, pkg)
			}
		}
	}
}

// parseFuncs parses the pkg for any declared RPCs and auth handlers.
func (p *parser) parseFuncs(pkg *est.Package, svc *est.Service) (isService bool) {
	for _, f := range pkg.Files {
		for _, decl := range f.AST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			dir, doc := p.parseDirectives(fd.Doc)
			if dir == nil {
				continue
			}

			switch dir := dir.(type) {
			case *rpcDirective:
				rpc := &est.RPC{
					Svc:    svc,
					Name:   fd.Name.Name,
					Doc:    doc,
					Access: dir.Access,
					Raw:    dir.Raw,
					Func:   fd,
					File:   f,
					Path:   dir.Params[directiveParamPath],
				}
				p.validateRPC(rpc)
				svc.RPCs = append(svc.RPCs, rpc)
				isService = true

			case *authHandlerDirective:
				if h := p.authHandler; h != nil {
					p.errf(fd.Pos(), "cannot declare multiple auth handlers (previous declaration at %s)",
						p.fset.Position(h.Func.Pos()))
					continue
				}
				authHandler := &est.AuthHandler{
					Svc:  svc,
					Name: fd.Name.Name,
					Doc:  doc,
					Func: fd,
					File: f,
				}
				p.validateAuthHandler(authHandler)
				p.authHandler = authHandler
				isService = true

			default:
				p.errf(dir.Pos(), "unexpected directive type %T", dir)
				panic(bailout{})
			}
		}
	}
	return isService
}

func (p *parser) validateRPC(rpc *est.RPC) {
	if rpc.Raw {
		p.validateRawRPC(rpc)
	} else {
		p.validateTypedRPC(rpc)
	}
}

func (p *parser) validateTypedRPC(rpc *est.RPC) {
	const sigHint = `
	hint: valid signatures are:
	- func(context.Context) error
	- func(context.Context) (*ResponseData, error)
	- func(context.Context, *RequestData) error
	- func(context.Context, *RequestType) (*ResponseData, error)`

	params := rpc.Func.Type.Params
	numParams := params.NumFields()
	if numParams == 0 {
		p.errf(rpc.Func.Type.Pos(), "invalid API signature (too few parameters)"+sigHint)
		return
	}

	results := rpc.Func.Type.Results
	numResults := results.NumFields()
	if numResults == 0 {
		p.errf(rpc.Func.Type.Pos(), "invalid API signature (too few results)"+sigHint)
		return
	}

	names := p.names[rpc.Svc.Root]
	info := names.Files[rpc.File]

	// First type should always be context.Context
	req := params.List[0].Type
	if err := validateSel(info, req, "context", "Context"); err != nil {
		if err == errNotFound {
			p.err(req.Pos(), "first parameter must be of type context.Context"+sigHint)
		} else {
			p.err(req.Pos(), err.Error()+sigHint)
		}
		return
	}

	// Second type must be *T or *pkg.T
	if numParams >= 2 {
		param, _ := getField(params, 1)
		decl := p.resolveDecl(rpc.Svc.Root, rpc.File, param.Type)
		if decl.Type.GetStruct() == nil {
			p.err(param.Pos(), "request type must be a struct type")
		}

		_, isPtr := param.Type.(*ast.StarExpr)
		rpc.Request = &est.Param{
			IsPtr: isPtr,
			Decl:  decl,
		}
	}

	// First return value must be *T or *pkg.T
	if numResults >= 2 {
		result := results.List[0]
		decl := p.resolveDecl(rpc.Svc.Root, rpc.File, result.Type)
		if decl.Type.GetStruct() == nil {
			p.err(result.Pos(), "response type must be a struct type")
		}
		_, isPtr := result.Type.(*ast.StarExpr)
		rpc.Response = &est.Param{
			IsPtr: isPtr,
			Decl:  decl,
		}
	}

	if numParams > 2 {
		param, _ := getField(params, 2)
		p.err(param.Pos(), "API signature cannot contain more than two parameters"+sigHint)
		return
	} else if numResults > 2 {
		result, _ := getField(results, 2)
		p.err(result.Pos(), "API signature cannot contain more than two results"+sigHint)
		return
	}

	err, _ := getField(results, numResults-1)
	if id, ok := err.Type.(*ast.Ident); !ok || id.Name != "error" {
		p.err(err.Pos(), "last result is not of type error"+sigHint)
		return
	} else if names.Decls["error"] != nil {
		p.err(err.Pos(), "last result is not of type error (local name shadows builtin)"+sigHint)
		return
	}
}

func (p *parser) validateRawRPC(rpc *est.RPC) {
	const sigHint = `
	hint: signature must be func(http.ResponseWriter, *http.Request)`

	params := rpc.Func.Type.Params
	if params.NumFields() < 2 {
		p.err(params.Pos(), "invalid API signature (too few parameters)"+sigHint)
		return
	} else if params.NumFields() > 2 {
		p.err(params.Pos(), "invalid API signature (too many parameters)"+sigHint)
		return
	} else if results := rpc.Func.Type.Results; results.NumFields() != 0 {
		p.err(params.Pos(), "invalid API signature (too many results)"+sigHint)
		return
	}

	info := p.names[rpc.Svc.Root].Files[rpc.File]

	{
		// First type should always be http.ResponseWriter
		rw := params.List[0].Type
		if err := validateSel(info, rw, "net/http", "ResponseWriter"); err != nil {
			if err == errNotFound {
				p.err(rw.Pos(), "first parameter must be http.ResponseWriter"+sigHint)
			} else {
				p.err(rw.Pos(), err.Error()+sigHint)
			}
			return
		}
	}

	{
		// First type should always be *http.Request
		req := params.List[1].Type
		star, ok := req.(*ast.StarExpr)
		if !ok {
			p.err(req.Pos(), "second parameter must be *http.Request"+sigHint)
			return
		} else if err := validateSel(info, star.X, "net/http", "Request"); err != nil {
			if err == errNotFound {
				p.err(req.Pos(), "second parameter must be *http.Request"+sigHint)
			} else {
				p.err(req.Pos(), err.Error()+sigHint)
			}
			return
		}
	}
}

// validateAuthHandler parses and valiidates the function declaration for an auth handler.
func (p *parser) validateAuthHandler(h *est.AuthHandler) {
	const sigHint = `
	hint: valid signatures are:
	- func(ctx context.Context, token string) (auth.UID, error)
	- func(ctx context.Context, token string) (auth.UID, *UserData, error)

	note: *UserData is a custom data type you define`

	typ := h.Func.Type
	params := typ.Params
	numParams := params.NumFields()
	if numParams < 2 {
		p.errf(h.Func.Type.Pos(), "invalid API signature (too few parameters)"+sigHint)
		return
	} else if numParams > 3 {
		p.errf(h.Func.Type.Pos(), "invalid API signature (too many parameters)"+sigHint)
		return
	}

	results := typ.Results
	numResults := results.NumFields()
	if numResults < 2 {
		p.errf(h.Func.Type.Pos(), "invalid API signature (too few results)"+sigHint)
		return
	} else if numResults > 3 {
		p.errf(h.Func.Type.Pos(), "invalid API signature (too many results)"+sigHint)
		return
	}

	names := p.names[h.Svc.Root]
	info := names.Files[h.File]

	// First param must be context.Context
	req, _ := getField(params, 0)
	if err := validateSel(info, req.Type, "context", "Context"); err != nil {
		if err == errNotFound {
			p.err(req.Type.Pos(), "first parameter must be of type context.Context"+sigHint)
		} else {
			p.err(req.Type.Pos(), err.Error()+sigHint)
		}
		return
	}

	// Second param must be string
	tok, _ := getField(params, 1)
	if id, ok := tok.Type.(*ast.Ident); !ok || id.Name != "string" {
		p.err(tok.Type.Pos(), "second parameter must be of type string"+sigHint)
		return
	} else if names.Decls["string"] != nil {
		p.err(tok.Type.Pos(), "second parameter must be of type string (local name shadows builtin)"+sigHint)
		return
	}

	// First result must be auth.UID
	uid, _ := getField(results, 0)
	if err := validateSel(info, uid.Type, "encore.dev/beta/auth", "UID"); err != nil {
		if err == errNotFound {
			p.err(req.Type.Pos(), "first result must be of type auth.UID"+sigHint)
		} else {
			p.err(req.Type.Pos(), err.Error()+sigHint)
		}
		return
	}

	if numResults == 3 {
		// Second result must be *T or *pkg.T
		authData, _ := getField(results, 1)
		decl := p.resolveDecl(h.Svc.Root, h.File, authData.Type)
		if decl.Type.GetStruct() == nil {
			p.err(authData.Pos(), "auth data must be a struct type")
		}
		_, isPtr := authData.Type.(*ast.StarExpr)
		h.AuthData = &est.Param{
			IsPtr: isPtr,
			Decl:  decl,
		}
	}

	// Last result must be error
	err, _ := getField(results, numResults-1)
	if id, ok := err.Type.(*ast.Ident); !ok || id.Name != "error" {
		p.err(err.Pos(), "last result is not of type error"+sigHint)
		return
	} else if names.Decls["error"] != nil {
		p.err(err.Pos(), "last result is not of type error (local name shadows builtin)"+sigHint)
		return
	}
}

var errNotFound = errors.New("not found")

func validateSel(info *names.File, x ast.Node, pkgPath, name string) error {
	if sel, ok := x.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
		if id, ok := sel.X.(*ast.Ident); ok {
			path := info.NameToPath[id.Name]
			if path == "" {
				return fmt.Errorf(`missing import of package "%s"`, pkgPath)
			} else if path != pkgPath {
				return fmt.Errorf(`missing import of package "%s"\n\tidentifier %s" refers to package "%s"`, pkgPath, id.Name, path)
			}
			return nil
		}
	}
	return errNotFound
}
