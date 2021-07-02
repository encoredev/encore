package compiler

import (
	_ "embed" // for go:embed
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"text/template"

	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

var (
	//go:embed tmpl/main.go.tmpl
	mainTmpl string
	//go:embed tmpl/pkg.go.tmpl
	pkgTmpl string
	//go:embed tmpl/testmain.go.tmpl
	testMainTmpl string
)

const mainPkgName = "__encore_main"

func (b *builder) writeMainPkg() error {
	imp := new(importMap)

	type parseFn struct {
		Name string
		Arg  string
		Cast string
	}

	funcs := template.FuncMap{
		"pkgName": func(path string) string {
			return imp.Name(path)
		},
		"traceExpr": func(obj interface{}) int32 {
			switch obj := obj.(type) {
			case *est.RPC:
				return b.res.Nodes[obj.Svc.Root][obj.Func].Id
			case *est.AuthHandler:
				return b.res.Nodes[obj.Svc.Root][obj.Func].Id
			default:
				panic(fmt.Sprintf("unexpected obj %T in traceExpr", obj))
			}
		},
		"typeName": func(param *est.Param) string {
			return b.typeName(param, imp)
		},
		"usesSQLDB": func(svc *est.Service) bool {
			for _, s := range b.res.Meta.Svcs {
				if s.Name == svc.Name {
					return len(s.Migrations) > 0
				}
			}
			return false
		},
		"requiresAuth": func(rpc *est.RPC) bool {
			return rpc.Access == est.Auth
		},
		"quote": func(s string) string {
			return strconv.Quote(s)
		},
		"pathParams": func(rpc *est.RPC) []paths.Segment {
			segs := make([]paths.Segment, 0, len(rpc.Path.Segments))
			for _, s := range rpc.Path.Segments {
				if s.Type != paths.Literal {
					segs = append(segs, s)
				}
			}
			return segs
		},
		"parseFunc": func(s paths.Segment) parseFn {
			if s.Type == paths.Wildcard {
				return parseFn{"parseWildcard", "", ""}
			}
			switch s.ValueType {
			case schema.Builtin_STRING:
				return parseFn{"", "", ""}
			case schema.Builtin_INT8:
				return parseFn{"strconv.ParseInt", "10, 8", "int8"}
			case schema.Builtin_INT16:
				return parseFn{"strconv.ParseInt", "10, 16", "int16"}
			case schema.Builtin_INT32:
				return parseFn{"strconv.ParseInt", "10, 32", "int32"}
			case schema.Builtin_INT64:
				return parseFn{"strconv.ParseInt", "10, 64", ""}
			case schema.Builtin_INT:
				return parseFn{"strconv.ParseInt", "10, 64", "int"}
			case schema.Builtin_UINT8:
				return parseFn{"strconv.ParseUint", "10, 8", "uint8"}
			case schema.Builtin_UINT16:
				return parseFn{"strconv.ParseUint", "10, 16", "uint16"}
			case schema.Builtin_UINT32:
				return parseFn{"strconv.ParseUint", "10, 32", "uint32"}
			case schema.Builtin_UINT64:
				return parseFn{"strconv.ParseUint", "10, 64", ""}
			case schema.Builtin_UINT:
				return parseFn{"strconv.ParseUint", "10, 64", "uint"}
			case schema.Builtin_BOOL:
				return parseFn{"strconv.ParseBool", "", ""}
			case schema.Builtin_UUID:
				return parseFn{"uuid.FromString", "", ""}
			default:
				panic(fmt.Sprintf("unknown paths type %v", s.ValueType))
			}
		},
		// wildcardSegmentIdx reports the path param index of the wildcard segment, or -1 if it doesn't exist.
		"wildcardSegmentIdx": func(rpc *est.RPC) int {
			idx := 0
			for _, s := range rpc.Path.Segments {
				if s.Type == paths.Wildcard {
					return idx
				} else if s.Type == paths.Param {
					idx++
				}
			}
			return -1
		},
	}
	tmpl := template.Must(template.New("mainPkg").Funcs(funcs).Parse(mainTmpl))
	// Write the file to disk
	dir := filepath.Join(b.workdir, mainPkgName)
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	mainPath := filepath.Join(dir, "main.go")
	file, err := os.Create(mainPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	for _, svc := range b.res.App.Services {
		imp.Add(svc.Name, svc.Root.ImportPath)
		for _, rpc := range svc.RPCs {
			if r := rpc.Request; r != nil {
				imp.Add(r.Decl.Loc.PkgName, r.Decl.Loc.PkgPath)
			}
			if r := rpc.Response; r != nil {
				imp.Add(r.Decl.Loc.PkgName, r.Decl.Loc.PkgPath)
			}
		}
	}
	if h := b.res.App.AuthHandler; h != nil {
		imp.Add(h.Svc.Name, h.Svc.Root.ImportPath)
	}

	tmplParams := struct {
		Imports     []importName
		Svcs        []*est.Service
		AppVersion  string
		AppRoot     string
		AuthHandler *est.AuthHandler
	}{
		Svcs:        b.res.App.Services,
		AppVersion:  b.cfg.Version,
		AppRoot:     b.appRoot,
		AuthHandler: b.res.App.AuthHandler,
		Imports:     imp.Imports(),
	}

	b.addOverlay(filepath.Join(b.appRoot, mainPkgName, "main.go"), mainPath)
	return tmpl.Execute(file, tmplParams)
}

func (b *builder) generateWrappers(pkg *est.Package, rpcs []*est.RPC, wrapperPath string) (err error) {
	type rpcDesc struct {
		Name   string
		Svc    string
		Params []string
		Resp   string
		Func   string
	}

	tmpl := template.Must(template.New("pkg").Parse(pkgTmpl))

	file, err := os.Create(wrapperPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	var rpcDescs []*rpcDesc
	imp := &importMap{from: pkg.ImportPath}
	for _, rpc := range rpcs {
		rpcPkg := rpc.Svc.Root
		req := b.typeName(rpc.Request, imp)
		resp := b.typeName(rpc.Response, imp)
		fn := rpc.Name
		if n := imp.Add(rpcPkg.Name, rpcPkg.ImportPath); n.Name != "" {
			fn = n.Name + "." + fn
		}

		desc := &rpcDesc{
			Name: rpc.Name,
			Svc:  rpc.Svc.Name,
			Resp: resp,
			Func: fn,
		}
		for _, p := range rpc.Path.Segments {
			if p.Type != paths.Literal {
				desc.Params = append(desc.Params, b.pathParamName(p.ValueType, imp))
			}
		}
		if req != "" {
			desc.Params = append(desc.Params, req)
		}
		rpcDescs = append(rpcDescs, desc)
	}

	tmplParams := struct {
		Pkg     *est.Package
		RPCs    []*rpcDesc
		Imports []importName
	}{
		Pkg:     pkg,
		RPCs:    rpcDescs,
		Imports: imp.Imports(),
	}

	return tmpl.Execute(file, tmplParams)
}

func (b *builder) generateTestMain(pkg *est.Package) (err error) {
	funcs := template.FuncMap{
		"usesSQLDB": func(svc *est.Service) bool {
			for _, s := range b.res.Meta.Svcs {
				if s.Name == svc.Name {
					return len(s.Migrations) > 0
				}
			}
			return false
		},
	}

	tmpl := template.Must(template.New("testmain").Funcs(funcs).Parse(testMainTmpl))

	// Write the file to disk
	testMainPath := filepath.Join(b.workdir, filepath.FromSlash(pkg.RelPath), "encore_testmain_test.go")
	file, err := os.Create(testMainPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	tmplParams := struct {
		Pkg  *est.Package
		Svcs []*est.Service
	}{
		Pkg:  pkg,
		Svcs: b.res.App.Services,
	}
	b.addOverlay(filepath.Join(pkg.Dir, "encore_testmain_test.go"), testMainPath)
	return tmpl.Execute(file, tmplParams)
}

// importMap manages imports for a given file, and ensures each import
// is given a unique name even in the presence of name collisions.
// The zero value is ready to be used.
type importMap struct {
	from  string // from is the import path the code is running in
	names map[string]importName
	paths map[string]importName
}

type importName struct {
	Name  string
	Path  string
	Named bool
}

func (i *importMap) Add(name, path string) importName {
	if path == i.from {
		return importName{}
	}

	if i.names == nil {
		i.names = make(map[string]importName)
		i.paths = make(map[string]importName)
	}

	named := false
	if p, ok := i.paths[path]; ok {
		// Already imported
		return p
	} else if _, ok := i.names[name]; ok {
		// Name collision; generate a unique name
		for j := 2; ; j++ {
			candidate := name + strconv.Itoa(j)
			if _, ok := i.names[candidate]; !ok {
				name = candidate
				named = true
				break
			}
		}
	}

	n := importName{
		Name:  name,
		Path:  path,
		Named: named,
	}
	i.names[name] = n
	i.paths[path] = n
	return n
}

func (i *importMap) Imports() []importName {
	var names []importName
	for _, n := range i.paths {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i].Path < names[j].Path
	})
	return names
}

func (i *importMap) Name(path string) string {
	name, ok := i.paths[path]
	if !ok {
		panic(fmt.Sprintf("internal error: no import found for %q", path))
	}
	return name.Name
}

// typeName computes the type name for a given param
// from the perspective of from and if necessary
// adds the import to the import map.
//
// If param is nil, it returns "".
func (b *builder) typeName(param *est.Param, imp *importMap) string {
	if param == nil {
		return ""
	}

	decl := param.Decl
	typName := decl.Name
	n := imp.Add(decl.Loc.PkgName, decl.Loc.PkgPath)
	if n.Name != "" {
		typName = n.Name + "." + typName
	}
	if param.IsPtr {
		return "*" + typName
	}
	return typName
}

func (b *builder) pathParamName(t schema.Builtin, imp *importMap) string {
	switch t {
	case schema.Builtin_STRING:
		return "string"
	case schema.Builtin_BOOL:
		return "bool"
	case schema.Builtin_INT8:
		return "int8"
	case schema.Builtin_INT16:
		return "int16"
	case schema.Builtin_INT32:
		return "int32"
	case schema.Builtin_INT64:
		return "int64"
	case schema.Builtin_INT:
		return "int"
	case schema.Builtin_UINT8:
		return "uint8"
	case schema.Builtin_UINT16:
		return "uint16"
	case schema.Builtin_UINT32:
		return "uint32"
	case schema.Builtin_UINT64:
		return "uint64"
	case schema.Builtin_UINT:
		return "uint"
	case schema.Builtin_UUID:
		n := imp.Add("uuid", "encore.dev/types/uuid")
		if n.Name != "" {
			return n.Name + ".UUID"
		}
		return "UUID"
	default:
		panic(fmt.Sprintf("unexpected path param type %v", t))
	}
}
