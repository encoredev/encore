package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// parseFeatures parses the application packages looking for Encore features
// such as RPCs and auth handlers, and computes the set of services.
func (p *parser) parseServices() {
	p.svcPkgPaths = make(map[string]*est.Service) // import path -> *Service

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

		p.registerService(svc)
	}
}

// createService creates a new service registered as the given package
func (p *parser) createService(pkg *est.Package) {
	if pkg.Service != nil {
		p.errf(pkg.Files[0].AST.Pos(), "tried to turn package %s into a service, but it is already part of service %s", pkg.Name, pkg.Service.Name)
		return
	}

	svc := &est.Service{
		Name: pkg.Name,
		Root: pkg,
		Pkgs: []*est.Package{pkg},
	}
	p.registerService(svc)
}

// registerService adds a service to the list of services.
func (p *parser) registerService(svc *est.Service) {
	svc.Root.Service = svc
	p.svcPkgPaths[svc.Root.ImportPath] = svc
	if svc2 := p.svcMap[svc.Name]; svc2 != nil {
		p.errf(svc.Root.AST.Pos(), "service %s defined twice (previous definition at %s)",
			svc.Name, p.fset.Position(svc2.Root.Files[0].AST.Pos()))
	}
	p.svcs = append(p.svcs, svc)
	p.svcMap[svc.Name] = svc

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
			if svc := p.svcPkgPaths[path]; svc != nil {
				if p.svcPkgPaths[pkg.ImportPath] != nil {
					// This pkg is a service, but it's nested within another service
					p.errf(pkg.Files[0].AST.Pos(), "cannot nest service %s within service %s", pkg.Name, svc.Name)
					continue PkgLoop
				}

				if pkg.Service == nil {
					pkg.Service = svc
					svc.Pkgs = append(svc.Pkgs, pkg)
				} else if pkg.Service != svc {
					p.errf(pkg.Files[0].AST.Pos(), "package %s is part of service %s, but is also part of service %s", pkg.Name, svc.Name, pkg.Service.Name)
				}
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
				path := dir.Path
				if path == nil {
					path = &paths.Path{
						Pos: dir.TokenPos,
						Segments: []paths.Segment{{
							Type:  paths.Literal,
							Value: svc.Name + "." + fd.Name.Name,
						}},
					}
				}
				rpc := &est.RPC{
					Svc:         svc,
					Name:        fd.Name.Name,
					Doc:         doc,
					Access:      dir.Access,
					Raw:         dir.Raw,
					Func:        fd,
					File:        f,
					Path:        path,
					HTTPMethods: dir.Method,
				}
				p.initRPC(rpc)

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
				p.parseAuthHandler(authHandler)
				p.authHandler = authHandler
				isService = true

			default:
				p.errf(dir.Pos(), "unexpected directive type %T", dir)
				p.abort()
			}
		}
	}
	return isService
}

func (p *parser) initRPC(rpc *est.RPC) {
	if rpc.Raw {
		p.initRawRPC(rpc)
	} else {
		p.initTypedRPC(rpc)
	}

	for _, m := range rpc.HTTPMethods {
		if err := p.paths.Add(m, rpc.Path); err != nil {
			if e, ok := err.(*paths.ConflictError); ok {
				p.errf(e.Path.Pos, "invalid API path: "+e.Context+" (other declaration at %s)",
					p.fset.Position(e.Other.Pos))
			} else {
				p.errf(e.Path.Pos, "invalid API path: %v", e)
			}
		}
	}
}

func (p *parser) initTypedRPC(rpc *est.RPC) {
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

	pkgNames := p.names[rpc.Svc.Root]
	info := pkgNames.Files[rpc.File]

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

	// For each path parameter, expect a parameter to match it
	var pathParams []*paths.Segment
	for i := 0; i < len(rpc.Path.Segments); i++ {
		if s := &rpc.Path.Segments[i]; s.Type != paths.Literal {
			pathParams = append(pathParams, s)
		}
	}

	seenParams := 0
	for i := 0; i < numParams-1; i++ {
		param, name := getField(params, i+1)

		// Is it a path parameter?
		if i < len(pathParams) {
			pp := pathParams[i]
			if name != pp.Value {
				p.errf(param.Pos(), "unexpected parameter name '%s', expected '%s' (to match path parameter '%s')",
					name, pp.Value, pp.String())
				continue
			}
			typ := p.resolveType(rpc.Svc.Root, rpc.File, param.Type, nil)
			if !p.validatePathParamType(param, name, typ, pp.Type) {
				continue
			}
			pathParams[seenParams].ValueType = typ.GetBuiltin()
			seenParams++
		} else {
			// Otherwise it must be a payload parameter
			payloadIdx := i - len(pathParams)
			if payloadIdx > 0 {
				p.err(param.Pos(), "APIs cannot have multiple payload parameters")
				continue
			}

			rpc.Request = p.resolveParameter("payload parameter", rpc.Svc.Root, rpc.File, param.Type)
		}
	}
	if seenParams < len(pathParams) {
		var missing []string
		for i := seenParams; i < len(pathParams); i++ {
			missing = append(missing, pathParams[i].Value)
		}
		p.errf(req.Pos(), "invalid API signature: expected function parameters named '%s' to match API path params", strings.Join(missing, "', '"))
	}

	// First return value must be *T or *pkg.T
	if numResults >= 2 {
		result := results.List[0]
		rpc.Response = p.resolveParameter("response", rpc.Svc.Root, rpc.File, result.Type)
	}

	if numResults > 2 {
		result, _ := getField(results, 2)
		p.err(result.Pos(), "API signature cannot contain more than two results"+sigHint)
		return
	}

	err, _ := getField(results, numResults-1)
	if id, ok := err.Type.(*ast.Ident); !ok || id.Name != "error" {
		p.err(err.Pos(), "last result is not of type error"+sigHint)
		return
	} else if pkgNames.Decls["error"] != nil {
		p.err(err.Pos(), "last result is not of type error (local name shadows builtin)"+sigHint)
		return
	}

	if len(rpc.HTTPMethods) == 0 {
		if rpc.Request != nil {
			rpc.HTTPMethods = []string{"POST"}
		} else {
			rpc.HTTPMethods = []string{"GET", "POST"}
		}
	}
}

func (p *parser) validatePathParamType(param *ast.Field, name string, typ *schema.Type, segType paths.SegmentType) bool {
	b := typ.GetBuiltin()

	if segType == paths.Wildcard && b != schema.Builtin_STRING {
		p.errf(param.Pos(), "wildcard path parameter '%s' must be a string", name)
		return false
	}

	switch b {
	case schema.Builtin_STRING,
		schema.Builtin_INT,
		schema.Builtin_INT8,
		schema.Builtin_INT16,
		schema.Builtin_INT32,
		schema.Builtin_INT64,
		schema.Builtin_UINT,
		schema.Builtin_UINT8,
		schema.Builtin_UINT16,
		schema.Builtin_UINT32,
		schema.Builtin_UINT64,
		schema.Builtin_BOOL,
		schema.Builtin_UUID:
		return true
	default:
		p.errf(param.Pos(), "path parameter '%s' must be a string, bool, integer, or encore.dev/types/uuid.UUID", name)
		return false
	}
}

func (p *parser) initRawRPC(rpc *est.RPC) {
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

	if len(rpc.HTTPMethods) == 0 {
		rpc.HTTPMethods = []string{"*"}
	}
}

// parseAuthHandler parses and validates the function declaration for an auth handler.
func (p *parser) parseAuthHandler(h *est.AuthHandler) {
	const sigHint = `
	hint: valid signatures are:
	- func(ctx context.Context, p *Params) (auth.UID, error)
	- func(ctx context.Context, p *Params) (auth.UID, *UserData, error)
	- func(ctx context.Context, token string) (auth.UID, error)
	- func(ctx context.Context, token string) (auth.UID, *UserData, error)

	note: *Params and *UserData are custom data types you define`

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

	pkgNames := p.names[h.Svc.Root]
	info := pkgNames.Files[h.File]

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

	// Second param must be string or named type pointing to a struct
	authInfo, _ := getField(params, 1)
	paramType := p.resolveType(h.Svc.Root, h.File, authInfo.Type, nil)
	switch typ := paramType.Typ.(type) {
	case *schema.Type_Named:
		decl := p.decls[typ.Named.Id]
		st := decl.Type.GetStruct()
		if st == nil {
			p.errf(authInfo.Type.Pos(), "%s must be a struct type", decl.Name)
		} else {
			// Ensure all fields in the struct are headers or query strings
			var invalidFields []string
			for _, f := range st.Fields {
				found := false
				for _, tag := range f.Tags {
					key := tag.Key
					if tag.Name != "-" && (key == "header" || key == "query" || key == "qs") {
						found = true
						break
					}
				}
				if !found {
					invalidFields = append(invalidFields, f.Name)
				}
			}

			if len(invalidFields) > 0 {
				p.errf(authInfo.Type.Pos(), "all struct fields used in auth handler parameter %s "+
					"must originate from HTTP headers or query strings.\n"+
					"\thint: specify `header:\"X-My-Header\"` or `query:\"my-query\"` struct tags\n"+
					"\tfor the field(s): %s", decl.Name, strings.Join(invalidFields, ", "))
			}
		}

	case *schema.Type_Builtin:
		if typ.Builtin != schema.Builtin_STRING {
			p.errf(authInfo.Type.Pos(), "second parameter must be of type string or a named type")
		}
	}
	h.Params = paramType

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

		h.AuthData = p.resolveParameter("auth data", h.Svc.Root, h.File, authData.Type)
	}

	// Last result must be error
	err, _ := getField(results, numResults-1)
	if id, ok := err.Type.(*ast.Ident); !ok || id.Name != "error" {
		p.err(err.Pos(), "last result is not of type error"+sigHint)
		return
	} else if pkgNames.Decls["error"] != nil {
		p.err(err.Pos(), "last result is not of type error (local name shadows builtin)"+sigHint)
		return
	}
}

func (p *parser) resolveParameter(parameterType string, pkg *est.Package, file *est.File, expr ast.Expr) *est.Param {
	typ := p.resolveType(pkg, file, expr, nil)

	// Check it's a supported parameter type (i.e. a named type which is a structure)
	n := typ.GetNamed()
	if n == nil {
		p.errf(expr.Pos(), "%s is not a named type. API Parameters must be a struct type.", types.ExprString(expr))
		p.abort()
	}

	if p.decls[n.Id].Type.GetStruct() == nil {
		p.errf(expr.Pos(), "%s must be a struct type", parameterType)
	}
	_, isPtr := expr.(*ast.StarExpr)

	return &est.Param{
		IsPtr: isPtr,
		Type:  typ,
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

func unwrapSel(sel *ast.SelectorExpr) (x ast.Expr, ids []*ast.Ident) {
	ids = []*ast.Ident{sel.Sel}
	for {
		if sel2, ok := sel.X.(*ast.SelectorExpr); ok {
			ids = append(ids, sel2.Sel)
			sel = sel2
		} else {
			break
		}
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		ids = append(ids, id)
	} else {
		x = sel.X
	}

	// Reverse the ids
	for i, n := 0, len(ids); i < n/2; i++ {
		ids[i], ids[n-i-1] = ids[n-i-1], ids[i]
	}

	return x, ids
}
