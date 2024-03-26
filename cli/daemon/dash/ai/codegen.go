package ai

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/env"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/codegen/rewrite"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/resource/resourceparser"
)

const HEADER_DIVIDER = "// __import-start\n"

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

func writeToFile(absPath string, svc string, ep *EndpointInput) error {
	err := os.MkdirAll(filepath.Dir(absPath), 0755)
	if err != nil {
		return err
	}
	content := toSrcFile(svc, ep.EndpointSource, ep.TypeSource)
	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return err
	}
	// Try resolving imports using goimports
	err = exec.Command("goimports", "-w", absPath).Run()
	if err != nil {
		// If that fails, fallback on gopls
		err = exec.Command("gopls", "imports", "-w", absPath).Run()
		if err != nil {
			// If that fails, log the error
			log.Err(err).Msg("failed to resolve imports")
		}
	}
	return nil

}

type servicePaths struct {
	relPaths map[string]string
	root     string
}

func (s *servicePaths) Add(key, val string) *servicePaths {
	s.relPaths[key] = val
	return s
}

func (s *servicePaths) Get(name string) string {
	pkgName, ok := s.relPaths[name]
	if !ok {
		pkgName = strings.ToLower(name)
	}
	return filepath.Join(s.root, pkgName)
}

func (s *servicePaths) NewFileName(svc, endpoint string) (string, error) {
	pkgPath := s.Get(svc)
	endpoint = strings.ToLower(endpoint)
	fileName := endpoint + ".go"
	var i int
	for {
		if _, err := os.Stat(filepath.Join(pkgPath, fileName)); os.IsNotExist(err) {
			return filepath.Join(pkgPath, fileName), nil
		} else if err != nil {
			return "", err
		}
		i++
		fileName = fmt.Sprintf("%s_%d.go", endpoint, i)
	}
}

func newServicePaths(app *apps.Instance, abs bool) (*servicePaths, error) {
	md, err := app.CachedMetadata()
	if err != nil {
		return nil, err
	}
	pkgRelPath := fns.ToMap(md.Pkgs, func(p *meta.Package) string { return p.RelPath })
	svcPaths := &servicePaths{relPaths: map[string]string{}}
	if abs {
		svcPaths.root = app.Root()
	}
	for _, svc := range md.Svcs {
		if pkgRelPath[svc.RelPath] != nil {
			svcPaths.Add(svc.Name, pkgRelPath[svc.RelPath].RelPath)
		}
	}
	return svcPaths, nil
}

func GenerateCode(services []ServiceInput, app *apps.Instance) ([]ServiceInput, error) {
	svcPaths, err := newServicePaths(app, true)
	if err != nil {
		return nil, err
	}
	for _, s := range services {
		for _, e := range s.Endpoints {
			fileName, err := svcPaths.NewFileName(s.Name, e.Name)
			if err != nil {
				return nil, err
			}
			err = writeToFile(fileName, s.Name, e)
			if err != nil {
				return nil, err
			}
		}
	}
	return services, nil
}

type ValidationError struct {
	Message string `json:"message"`
	Line    *int   `json:"line"`
	Column  *int   `json:"column"`
}

type ValidationResult struct {
	Service  string            `json:"service"`
	Endpoint string            `json:"endpoint"`
	File     string            `json:"file"`
	Code     string            `json:"code"`
	Errors   []ValidationError `json:"errors"`
}

type SyncResult struct {
	ServiceInputs []ServiceInput    `json:"services"`
	Errors        []ValidationError `json:"errors"`
}

func (e ValidationResult) IsError() bool {
	return len(e.Errors) > 0
}

func toSrcFile(svc string, srcs ...string) []byte {
	src := []byte(fmt.Sprintf("package %s\n\n%s", strings.ToLower(svc), strings.Join(srcs, "\n")))
	importedSrc, err := imports.Process("", src, nil)
	if err == nil {
		return importedSrc
	}
	return src
}

func SyncEndpoints(ctx context.Context, services []ServiceInput, app *apps.Instance, fromSrc bool) (*SyncResult, error) {
	if fromSrc {
		services, err := srcToStruct(ctx, app, services)
		return &SyncResult{
			ServiceInputs: services,
		}, err
	} else {
		services, err := structToSrc(services, app)
		return &SyncResult{
			ServiceInputs: services,
		}, err
	}
}

func toPathSegments(p *resourcepaths.Path) []PathSegment {
	rtn := make([]PathSegment, 0, len(p.Segments))
	for _, s := range p.Segments {
		switch s.Type {
		case resourcepaths.Literal:
			rtn = append(rtn, PathSegment{Type: SegmentTypeLiteral, Value: ptr(s.Value)})
		case resourcepaths.Param:
			rtn = append(rtn, PathSegment{Type: SegmentTypeParam, Value: ptr(s.Value), ValueType: ptr(SegmentValueType(s.ValueType.String()))})
		case resourcepaths.Wildcard:
			rtn = append(rtn, PathSegment{Type: SegmentTypeWildcard})
		case resourcepaths.Fallback:
			rtn = append(rtn, PathSegment{Type: SegmentTypeFallback})
		}
	}
	return rtn
}

func formatErrors(errs ...error) []ValidationError {
	return fns.FlatMap(errs, func(e error) []ValidationError {
		return formatError(e)
	})
}

func formatError(err error) []ValidationError {
	if err == nil {
		return nil
	}
	var list scanner.ErrorList
	if errors.As(err, &list) {
		return fns.Map(list, func(e *scanner.Error) ValidationError {
			return ValidationError{
				Message: e.Msg,
				Line:    ptr(e.Pos.Line),
				Column:  ptr(e.Pos.Column),
			}
		})
	} else {
		return []ValidationError{{Message: err.Error()}}
	}
}

var errIDToCode = map[string]int{
	"OK":                 200,
	"Canceled":           499,
	"Unknown":            500,
	"InvalidArgument":    400,
	"DeadlineExceeded":   504,
	"NotFound":           404,
	"AlreadyExists":      409,
	"PermissionDenied":   403,
	"ResourceExhausted":  429,
	"FailedPrecondition": 400,
	"Aborted":            409,
	"OutOfRange":         400,
	"Unimplemented":      501,
	"Internal":           500,
	"Unavailable":        503,
	"DataLoss":           500,
	"Unauthenticated":    401,
}

func parseErrorDoc(doc string) (string, []ErrorInput) {
	var errors []ErrorInput
	lines := strings.Split(doc, "\n")
	errStart := -1
	errEnd := -1
	for i, line := range lines {
		errEnd = i
		if strings.HasPrefix(strings.TrimSpace(line), "Errors:") {
			errStart = i
		} else if errStart == -1 {
			continue
		}
		if strings.TrimSpace(line) == "" && strings.TrimSpace(lines[i-1]) == "" {
			break
		}
	}
	if errStart == -1 {
		return doc, errors
	}

	for _, line := range lines[errStart+1 : errEnd+1] {
		line = strings.TrimSpace(line)
		errID, doc, ok := strings.Cut(line, ":")
		if ok && errIDToCode[errID] != 0 {
			errors = append(errors, ErrorInput{
				Code: errID,
				Doc:  strings.TrimSpace(doc),
			})
		} else if len(errors) > 0 {
			errors[len(errors)-1].Doc += "\n" + line
		}
	}
	return strings.Join(lines[:errStart], "\n"), errors
}

func srcToStruct(ctx context.Context, app *apps.Instance, services []ServiceInput) (rtn []ServiceInput, err error) {
	defer func() {
		if l, ok := perr.CatchBailout(recover()); ok {
			l.Len()
			err = l.AsError()
		}
	}()

	pathToSvc := map[string]*EndpointInput{}
	files := map[string][]byte{}
	svcPaths, err := newServicePaths(app, false)
	if err != nil {
		return nil, err
	}
	for _, s := range services {
		pkgPath := svcPaths.Get(s.Name)
		for _, e := range s.Endpoints {
			relPath := filepath.Join(pkgPath, e.Name+"_tmp.go")
			files[relPath] = toSrcFile(s.Name, e.EndpointSource, e.TypeSource)
			pathToSvc[relPath] = e
		}
	}

	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	rootDir := paths.RootedFSPath(app.Root(), ".")
	pc := &parsectx.Context{
		Ctx: ctx,
		Log: zerolog.Logger{},
		Build: parsectx.BuildInfo{
			Experiments: nil,
			GOROOT:      paths.RootedFSPath(env.EncoreGoRoot(), "."),
			GOARCH:      runtime.GOARCH,
			GOOS:        runtime.GOOS,
		},
		MainModuleDir: rootDir,
		FS:            fs,
		ParseTests:    false,
		Errs:          errs,
		Overlay:       files,
	}
	loader := pkginfo.New(pc)

	pkgs := map[string]*pkginfo.Package{}
	for p, _ := range files {
		pkg, _ := filepath.Split(p)
		if _, ok := pkgs[pkg]; !ok {
			pkgs[pkg], ok = loader.LoadPkg(token.NoPos, paths.MustPkgPath(path.Join(string(loader.MainModule().Path), pkg)))
			if !ok {
				return nil, errors.New("failed to load package")
			}
		}
	}
	schemaParser := schema.NewParser(pc, loader)
	for _, pkg := range pkgs {
		pass := &resourceparser.Pass{
			Context:      pc,
			SchemaParser: schemaParser,
			Pkg:          pkg,
		}
		apis.Parser.Run(pass)
		for _, r := range pass.Resources() {
			switch r := r.(type) {
			case *api.Endpoint:
				relPath, err := filepath.Rel(app.Root(), r.File.FSPath.ToIO())
				if err != nil {
					continue
				}
				e, ok := pathToSvc[relPath]
				if !ok {
					continue
				}

				e.Doc, e.Errors = parseErrorDoc(r.Doc)
				e.Name = r.Name
				e.Method = r.HTTPMethods[0]
				e.Path = toPathSegments(r.Path)
				e.Visibility = VisibilityType(r.Access)
				e.Language = "GO"
				e.Path = toPathSegments(r.Path)
				e.Types = []TypeInput{}
				if nr, ok := r.Request.(schema.NamedType); ok {
					e.RequestType = r.Request.String()
					e.Types = append(e.Types, TypeInput{
						Name: r.Request.String(),
						Doc:  nr.DeclInfo.Doc,
						Fields: fns.Map(r.RequestEncoding()[0].AllParameters(), func(f *apienc.ParameterEncoding) *TypeFieldInput {
							return &TypeFieldInput{
								Name:     f.SrcName,
								WireName: f.WireName,
								Location: f.Location,
								Type:     f.Type.String(),
								Doc:      f.Doc,
							}
						}),
					})
				}
				if nr, ok := r.Response.(schema.NamedType); ok {
					e.ResponseType = r.Response.String()
					e.Types = append(e.Types, TypeInput{
						Name: r.Response.String(),
						Doc:  nr.DeclInfo.Doc,
						Fields: fns.Map(r.ResponseEncoding().AllParameters(), func(f *apienc.ParameterEncoding) *TypeFieldInput {
							return &TypeFieldInput{
								Name:     f.SrcName,
								WireName: f.WireName,
								Location: f.Location,
								Type:     f.Type.String(),
								Doc:      f.Doc,
							}
						}),
					})
				}
			}

		}
		for _, d := range schemaParser.ParsedDecls() {
			switch d := d.(type) {
			case *schema.TypeDecl:
				schemaType, ok := d.Type.(schema.StructType)
				if !ok {
					continue
				}
				relPath, err := filepath.Rel(app.Root(), d.File.FSPath.ToIO())
				if err != nil {
					continue
				}
				e, ok := pathToSvc[relPath]
				if !ok {
					continue
				}
				if slices.ContainsFunc(e.Types, func(t TypeInput) bool { return t.Name == d.Name }) {
					continue
				}
				e.Types = append(e.Types, TypeInput{
					Name: d.Name,
					Doc:  d.Info.Doc,
					Fields: fns.Map(schemaType.Fields, func(f schema.StructField) *TypeFieldInput {
						return &TypeFieldInput{
							Name: f.Name.String(),
							Type: f.Type.String(),
							Doc:  f.Doc,
						}
					}),
				})
			}
		}
	}
	return services, nil
}

func structToSrc(services []ServiceInput, app *apps.Instance) ([]ServiceInput, error) {
	files := map[string][]byte{}
	pathToEp := map[string]*EndpointInput{}
	for _, s := range services {
		for _, e := range s.Endpoints {
			prefix := filepath.Join(app.Root(), s.Name, e.Name)
			p := prefix + "_ep_tmp.go"
			files[p] = toSrcFile(s.Name, HEADER_DIVIDER, e.EndpointSource)
			pathToEp[p] = e
			p = prefix + "_types_tmp.go"
			files[p] = toSrcFile(s.Name, HEADER_DIVIDER, e.TypeSource)
			pathToEp[p] = e
		}
	}
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Mode:    packages.NeedTypes | packages.NeedSyntax,
		Dir:     app.Root(),
		Fset:    fset,
		Overlay: files,
	}, "encore.app/birds")
	if err != nil {
		return nil, err
	}
	pathToAst := map[string]*ast.File{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			fPos := pkg.Fset.Position(f.Pos())
			pathToAst[fPos.Filename] = f
		}
	}

	for p, ep := range pathToEp {
		astFile := pathToAst[p]
		rewriter := rewrite.New(files[p], int(astFile.FileStart))
		typeByName := map[string]*ast.GenDecl{}
		funcByName := map[string]*ast.FuncDecl{}
		for _, decl := range astFile.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					typeByName[typeSpec.Name.Name] = decl
				}
			case *ast.FuncDecl:
				funcByName[decl.Name.Name] = decl
			}
		}
		if strings.HasSuffix(p, "_ep_tmp.go") {
			funcDecl := funcByName[ep.Name]
			sig := ep.Render()
			if funcDecl != nil {
				start := funcDecl.Pos()
				if funcDecl.Doc != nil {
					start = funcDecl.Doc.Pos()
				}
				rewriter.Replace(start, funcDecl.Body.Lbrace, []byte(sig))
			} else {
				sig = sig + ` {\n  panic("not implemented"\n}\n`
				rewriter.Append([]byte(sig))
			}
			content := string(rewriter.Data())
			_, content, ok := strings.Cut(content, HEADER_DIVIDER)
			if !ok {
				return nil, errors.New("no header divider")
			}
			ep.EndpointSource = strings.TrimSpace(content)
		} else {
			for _, typ := range ep.Types {
				typeSpec := typeByName[typ.Name]
				code := typ.Render()
				if typeSpec != nil {
					start := typeSpec.Pos()
					if typeSpec.Doc != nil {
						start = typeSpec.Doc.Pos()
					}
					rewriter.Replace(start, typeSpec.End(), []byte(code))
				} else {
					rewriter.Append([]byte(code))
				}
			}
			content := string(rewriter.Data())
			_, content, ok := strings.Cut(content, HEADER_DIVIDER)
			if !ok {
				return nil, errors.New("no header divider")
			}
			ep.TypeSource = strings.TrimSpace(content)
		}
	}

	return services, nil
}
