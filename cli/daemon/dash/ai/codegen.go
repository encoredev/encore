package ai

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"strconv"
	"strings"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/fns"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type endpointFile struct {
	buffer   strings.Builder
	svc      string
	endpoint string
	relPath  string
	pkg      string
}

func valueTypeToGoType(t *SegmentValueType) string {
	switch *t {
	case SegmentValueTypeString:
		return "string"
	case SegmentValueTypeInt:
		return "int"
	case SegmentValueTypeBool:
		return "bool"
	default:
		panic(fmt.Sprintf("unknown segment value type: %s", *t))
	}

}

func formatPath(segs []PathSegment) (string, []string) {
	var params []string
	return "/" + path.Join(fns.Map(segs, func(s PathSegment) string {
		switch s.Type {
		case SegmentTypeLiteral:
			return *s.Value
		case SegmentTypeParam:
			params = append(params, fmt.Sprintf("%s %s", *s.Value, valueTypeToGoType(s.ValueType)))
			return fmt.Sprintf(":%s", *s.Value)
		case SegmentTypeWildcard:
			return "*"
		case SegmentTypeFallback:
			return "!fallback"
		default:
			panic(fmt.Sprintf("unknown path segment type: %s", s.Type))
		}
	})...), params
}

func (f *endpointFile) Comment(comment string) {
	f.buffer.WriteString(fmt.Sprintf("// %s\n", comment))
}

func (f *endpointFile) Func(id string, params, rtnParams []string, body ...string) {
	paramsStr := strings.Join(params, ", ")
	rtnParamsStr := strings.Join(rtnParams, ", ")
	if len(rtnParams) > 1 {
		rtnParamsStr = fmt.Sprintf("(%s)", rtnParamsStr)
	}
	f.buffer.WriteString(fmt.Sprintf("func %s(%s) %s {\n%s\n}\n", id, paramsStr, rtnParamsStr, strings.Join(body, "\n")))
}

func (f *endpointFile) Pkg() {
	f.buffer.WriteString(fmt.Sprintf("package %s\n\n", f.pkg))
}

func (f *endpointFile) Imports(imports ...string) {
	f.buffer.WriteString("import (\n")
	for _, i := range imports {
		f.buffer.WriteString(fmt.Sprintf("\"%s\"\n", i))
	}
	f.buffer.WriteString(")\n\n")
}

func (f *endpointFile) Code(code string) {
	f.buffer.WriteString(code + "\n")
}

func (f *endpointFile) Handler(e EndpointInput) {
	if e.Doc != "" {
		f.Comment(e.Doc)
	}
	for i, err := range e.Errors {
		if i == 0 {
			f.Comment("Errors:")
		}
		f.Comment(fmt.Sprintf("  %s: %s", err.Code, err.Doc))
	}
	params := []string{"ctx context.Context"}
	path, pathParams := formatPath(e.Path)
	params = append(params, pathParams...)
	if e.RequestType != "" {
		params = append(params, "req "+e.RequestType)
	}
	var rtnParams []string
	if e.ResponseType != "" {
		rtnParams = append(rtnParams, e.ResponseType)
	}
	rtnParams = append(rtnParams, "error")
	f.Comment(fmt.Sprintf("encore:api %s method=%s path=%s", e.Method, e.Method, path))
	f.Func(e.Name, params, rtnParams, `panic("not implemented")`)
}

func (f *endpointFile) validate() (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", f.buffer.String(), parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	// Try to import undefined packages (for now we'll just test for stdlib packages)
	undefined := map[string]struct{}{}
	imp := importer.ForCompiler(fset, "gc", nil)
	var unexpectedErr error
	conf := types.Config{
		Importer: imp,
		Error: func(err error) {
			if terr, ok := err.(types.Error); ok {
				before, after, found := strings.Cut(terr.Msg, ": ")
				if found && before == "undefined" {
					if _, err := imp.Import(after); err == nil {
						undefined[after] = struct{}{}
					}
					return
				}
			}
		},
	}
	_, _ = conf.Check("", fset, []*ast.File{file}, nil)
	if unexpectedErr != nil {
		return nil, nil, unexpectedErr
	}
	if len(undefined) > 0 {
		for i := 0; i < len(file.Decls); i++ {
			decl, ok := file.Decls[i].(*ast.GenDecl)
			if !ok || decl.Tok != token.IMPORT {
				continue
			}
			for missing, _ := range undefined {
				spec := &ast.ImportSpec{Path: &ast.BasicLit{Value: strconv.Quote(missing)}}
				decl.Specs = append(decl.Specs, spec)
			}
			break
		}
		ast.SortImports(fset, file)
		_, _ = conf.Check("", fset, []*ast.File{file}, nil)
		if unexpectedErr != nil {
			return nil, nil, unexpectedErr
		}
	}
	return fset, file, nil
}

func (f *endpointFile) write(root string) error {
	fset, file, err := f.validate()
	if err != nil {
		return err
	}
	pkgPath := path.Join(root, f.relPath)
	fileName := f.endpoint + ".go"
	var i int
	for {
		if _, err := os.Stat(path.Join(pkgPath, fileName)); os.IsNotExist(err) {
			break
		} else if err != nil {
			return err
		}
		i++
		fileName = fmt.Sprintf("%s_%d.go", f.endpoint, i)
	}

	err = os.MkdirAll(pkgPath, 0755)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(path.Join(pkgPath, fileName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer fns.CloseIgnore(out)
	return format.Node(out, fset, file)
}

func generateCode(services []ServiceInput, app *apps.Instance) ([]*endpointFile, error) {
	md, err := app.CachedMetadata()
	if err != nil {
		return nil, err
	}
	svcByName := make(map[string]*meta.Service)
	for _, s := range md.Svcs {
		svcByName[strings.ToLower(s.Name)] = s
	}
	pkgByRelpath := make(map[string]*meta.Package)
	for _, p := range md.Pkgs {
		pkgByRelpath[p.RelPath] = p
	}
	var endpointFiles []*endpointFile
	for _, s := range services {
		relpath := strings.ToLower(s.Name)
		pkg := relpath
		svc, ok := svcByName[relpath]
		if ok {
			relpath = svc.RelPath
			pkg = pkgByRelpath[relpath].Name
		}
		for _, e := range s.Endpoints {
			f := &endpointFile{
				svc:      s.Name,
				endpoint: e.Name,
				relPath:  relpath,
				pkg:      pkg,
			}
			endpointFiles = append(endpointFiles, f)
			f.Pkg()
			f.Imports("context")
			f.Code(e.Structs)
			f.Handler(e)
		}
	}
	return endpointFiles, nil
}

func GenerateCode(services []ServiceInput, app *apps.Instance) error {
	endpointFiles, err := generateCode(services, app)
	if err != nil {
		return err
	}
	for _, f := range endpointFiles {
		if err := f.write(app.Root()); err != nil {
			return err
		}
	}
	return nil
}

func ValidateCode(services []ServiceInput, app *apps.Instance) error {
	endpointFiles, err := generateCode(services, app)
	if err != nil {
		return err
	}
	for _, f := range endpointFiles {
		if _, _, err := f.validate(); err != nil {
			return err
		}
	}
	return nil
}
