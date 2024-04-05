package ai

import (
	"context"
	"go/ast"
	"go/token"
	"runtime"
	"slices"
	"strings"

	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/env"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apienc"
	"encr.dev/v2/parser/resource/resourceparser"
)

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

type DocEntry struct {
	Key string
	Doc string
}

func parseErrorDoc(doc string) (string, []*Error) {
	doc, errs := parseDocSection(doc, ErrDocPrefix)
	return doc, fns.Map(errs, func(e DocEntry) *Error {
		return &Error{
			Code: e.Key,
			Doc:  e.Doc,
		}
	})
}

func parsePathDoc(doc string) (string, map[string]string) {
	doc, docs := parseDocSection(doc, PathDocPrefix)
	rtn := map[string]string{}
	for _, d := range docs {
		rtn[d.Key] = d.Doc
	}
	return doc, rtn
}

func parseDocSection(doc, section string) (string, []DocEntry) {
	var errs []DocEntry
	lines := strings.Split(doc, "\n")
	start := -1
	end := -1
	for i, line := range lines {
		end = i
		if strings.HasPrefix(strings.TrimSpace(line), section+":") {
			start = i

		} else if start == -1 {
			continue
		} else if len(line) > 2 {
			switch strings.TrimSpace(line[:2]) {
			case "-", "":
			default:
				end = i - 1
				break
			}
		}
		lines[i] = strings.TrimSpace(line)
		if line == "" && lines[i-1] == "" {
			break
		}
	}
	if start == -1 {
		return doc, errs
	}

	for _, line := range lines[start+1 : end+1] {
		key, doc, ok := strings.Cut(line, ":")
		key = strings.TrimPrefix(key, "-")
		key = strings.TrimSpace(key)
		if ok {
			errs = append(errs, DocEntry{
				Key: key,
				Doc: strings.TrimSpace(doc),
			})
		} else if len(errs) > 0 && line != "" {
			errs[len(errs)-1].Doc += "\n" + line
		}
	}
	return strings.Join(lines[:start], "\n"), errs
}

func deref(p schema.Type) schema.Type {
	for {
		if pt, ok := p.(schema.PointerType); ok {
			p = pt.Elem
		} else {
			return p
		}
	}
}

func parseCode(ctx context.Context, app *apps.Instance, services []Service) (rtn *SyncResult, err error) {
	overlays, err := newOverlays(app, false, services...)
	if err != nil {
		return nil, err
	}
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs, overlays.ReadFile)
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
		Overlay:       overlays,
	}
	defer func() {
		perr.CatchBailout(recover())
		if rtn == nil {
			rtn = &SyncResult{
				Services: services,
			}
		}
		rtn.Errors = overlays.validationErrors(errs)
	}()

	loader := pkginfo.New(pc)

	pkgs := map[paths.Pkg]*pkginfo.Package{}
	for _, pkg := range overlays.pkgPaths() {
		pkgs[pkg], _ = loader.LoadPkg(token.NoPos, pkg)
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
				overlay, ok := overlays.get(r.File.FSPath)
				if !ok {
					continue
				}
				e := overlay.endpoint
				pathDocs := map[string]string{}
				e.Doc, e.Errors = parseErrorDoc(r.Doc)
				e.Doc, pathDocs = parsePathDoc(e.Doc)
				e.Name = r.Name
				e.Method = r.HTTPMethods[0]
				e.Visibility = VisibilityType(r.Access)
				e.Language = "GO"
				e.Path = toPathSegments(r.Path, pathDocs)
				e.Types = []*Type{}
				if nr, ok := deref(r.Request).(schema.NamedType); ok {
					e.RequestType = nr.String()
					e := overlays.endpoint(nr.DeclInfo.File.FSPath)
					if len(r.RequestEncoding()) > 0 && e != nil {
						e.Types = append(e.Types, &Type{
							Name: nr.String(),
							Doc:  strings.TrimSpace(nr.DeclInfo.Doc),
							Fields: fns.Map(r.RequestEncoding()[0].AllParameters(), func(f *apienc.ParameterEncoding) *TypeField {
								return &TypeField{
									Name:     f.SrcName,
									WireName: f.WireName,
									Location: f.Location,
									Type:     f.Type.String(),
									Doc:      strings.TrimSpace(f.Doc),
								}
							}),
						})
					}
				}
				if nr, ok := deref(r.Response).(schema.NamedType); ok {
					e.ResponseType = nr.String()
					e := overlays.endpoint(nr.DeclInfo.File.FSPath)
					if r.ResponseEncoding() != nil && e != nil {
						e.Types = append(e.Types, &Type{
							Name: nr.String(),
							Doc:  strings.TrimSpace(nr.DeclInfo.Doc),
							Fields: fns.Map(r.ResponseEncoding().AllParameters(), func(f *apienc.ParameterEncoding) *TypeField {
								return &TypeField{
									Name:     f.SrcName,
									WireName: f.WireName,
									Location: f.Location,
									Type:     f.Type.String(),
									Doc:      strings.TrimSpace(f.Doc),
								}
							}),
						})
					}
				}
			}
		}
		for _, file := range pkg.Files {
			ast.Inspect(file.AST(), func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.GenDecl:
					if node.Tok != token.TYPE {
						return true
					}
					for _, spec := range node.Specs {
						d := spec.(*ast.TypeSpec)
						overlay, ok := overlays.get(file.FSPath)
						if !ok {
							continue
						}
						s, ok := schemaParser.ParseType(file, d.Type).(schema.StructType)
						if !ok {
							continue
						}
						e := overlay.endpoint
						if slices.ContainsFunc(e.Types, func(t *Type) bool { return t.Name == d.Name.Name }) {
							continue
						}
						e.Types = append(e.Types, &Type{
							Name:   d.Name.Name,
							Doc:    docText(node.Doc),
							Fields: fns.MapAndFilter(s.Fields, parseTypeField),
						})
					}
				}
				return true
			})
		}
	}
	return &SyncResult{
		Services: services,
	}, nil
}

func parseTypeField(f schema.StructField) (*TypeField, bool) {
	name, ok := f.Name.Get()
	if !ok {
		return nil, false
	}
	wirename := ""
	if tag, err := f.Tag.Get("json"); err == nil {
		wirename = tag.Name
	}
	return &TypeField{
		Name:     name,
		Type:     f.Type.String(),
		Doc:      f.Doc,
		WireName: wirename,
	}, true
}

func docText(c *ast.CommentGroup) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Text())
}
