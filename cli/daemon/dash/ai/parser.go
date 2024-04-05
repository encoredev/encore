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

// parseErrorList parses a list of errors docs from a doc string.
func parseErrorList(doc string) (string, []*Error) {
	doc, errs := parseDocList(doc, ErrDocPrefix)
	return doc, fns.Map(errs, func(e docListItem) *Error {
		return &Error{
			Code: e.Key,
			Doc:  e.Doc,
		}
	})
}

// parsePathList parses a list of path docs from a doc string.
func parsePathList(doc string) (string, map[string]string) {
	doc, docs := parseDocList(doc, PathDocPrefix)
	rtn := map[string]string{}
	for _, d := range docs {
		rtn[d.Key] = d.Doc
	}
	return doc, rtn
}

// parseDocList parses a list of key-value pairs from a doc string.
// e.g.
//
// Errors:
//   - NotFound: The requested resource was not found.
//   - InvalidArgument: The request had invalid arguments.
func parseDocList(doc, section string) (string, []docListItem) {
	var errs []docListItem
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
			errs = append(errs, docListItem{
				Key: key,
				Doc: strings.TrimSpace(doc),
			})
		} else if len(errs) > 0 && line != "" {
			errs[len(errs)-1].Doc += "\n" + line
		}
	}
	return strings.Join(lines[:start], "\n"), errs
}

// docListItem represents a key-value pair in a doc list.
type docListItem struct {
	Key string
	Doc string
}

// deref returns the underlying type of a pointer type.
func deref(p schema.Type) schema.Type {
	for {
		if pt, ok := p.(schema.PointerType); ok {
			p = pt.Elem
		} else {
			return p
		}
	}
}

// parseCode
func parseCode(ctx context.Context, app *apps.Instance, services []ServiceInput) (rtn *SyncResult, err error) {
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
				e.Doc, e.Errors = parseErrorList(r.Doc)
				e.Doc, pathDocs = parsePathList(e.Doc)
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
	wireName := ""
	if tag, err := f.Tag.Get("json"); err == nil {
		wireName = tag.Name
	}
	return &TypeField{
		Name:     name,
		Type:     f.Type.String(),
		Doc:      f.Doc,
		WireName: wireName,
	}, true
}

func docText(c *ast.CommentGroup) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Text())
}
