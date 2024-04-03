package ai

import (
	"context"
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

func parseErrorDoc(doc string) (string, []*ErrorInput) {
	doc, errs := parseDocSection(doc, "Errors")
	return doc, fns.Map(errs, func(e DocEntry) *ErrorInput {
		return &ErrorInput{
			Code: e.Key,
			Doc:  e.Doc,
		}
	})
}

func parseDocSection(doc, section string) (string, []DocEntry) {
	var errs []DocEntry
	lines := strings.Split(doc, "\n")
	errStart := -1
	errEnd := -1
	for i, line := range lines {
		errEnd = i
		if strings.HasPrefix(strings.TrimSpace(line), section+":") {
			errStart = i
		} else if errStart == -1 {
			continue
		}
		if len(line) > 2 {
			switch strings.TrimSpace(line[:2]) {
			case "-", "":
			default:
				errEnd = i - 1
			}
		}
		lines[i] = strings.TrimSpace(line)
		if line == "" && lines[i-1] == "" {
			break
		}
	}
	if errStart == -1 {
		return doc, errs
	}

	for _, line := range lines[errStart+1 : errEnd+1] {
		errID, doc, ok := strings.Cut(line, ":")
		errID = strings.TrimPrefix(errID, "-")
		errID = strings.TrimSpace(errID)
		if ok && errIDToCode[errID] != 0 {
			errs = append(errs, DocEntry{
				Key: errID,
				Doc: strings.TrimSpace(doc),
			})
		} else if len(errs) > 0 && line != "" {
			errs[len(errs)-1].Doc += "\n" + line
		}
	}
	return strings.Join(lines[:errStart], "\n"), errs
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

func parseCode(ctx context.Context, app *apps.Instance, services []ServiceInput) (rtn *SyncResult, err error) {
	overlays, err := newOverlays(app, false, services...)
	if err != nil {
		return nil, err
	}
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs, overlays.ReadFile).SetIgnoreBailouts(true)
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
		if err != nil {
			return
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
		func() {
			defer func() { perr.CatchBailout(recover()) }()
			apis.Parser.Run(pass)
		}()
		for _, r := range pass.Resources() {
			switch r := r.(type) {
			case *api.Endpoint:
				overlay, ok := overlays.get(r.File.FSPath)
				if !ok {
					continue
				}
				e := overlay.endpoint
				var pathDocs []DocEntry
				e.Doc, e.Errors = parseErrorDoc(r.Doc)
				e.Doc, pathDocs = parseDocSection(e.Doc, "Path Parameters")
				e.Name = r.Name
				e.Method = r.HTTPMethods[0]
				e.Visibility = VisibilityType(r.Access)
				e.Language = "GO"
				e.Path = toPathSegments(r.Path, pathDocs)
				e.Types = []*TypeInput{}
				if nr, ok := deref(r.Request).(schema.NamedType); ok {
					e.RequestType = nr.String()
					if len(r.RequestEncoding()) > 0 {
						e.Types = append(e.Types, &TypeInput{
							Name: nr.String(),
							Doc:  strings.TrimSpace(nr.DeclInfo.Doc),
							Fields: fns.Map(r.RequestEncoding()[0].AllParameters(), func(f *apienc.ParameterEncoding) *TypeFieldInput {
								return &TypeFieldInput{
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
					if r.ResponseEncoding() != nil {
						e.Types = append(e.Types, &TypeInput{
							Name: nr.String(),
							Doc:  strings.TrimSpace(nr.DeclInfo.Doc),
							Fields: fns.Map(r.ResponseEncoding().AllParameters(), func(f *apienc.ParameterEncoding) *TypeFieldInput {
								return &TypeFieldInput{
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
		for _, d := range schemaParser.ParsedDecls() {
			switch d := d.(type) {
			case *schema.TypeDecl:
				schemaType, ok := d.Type.(schema.StructType)
				if !ok {
					continue
				}
				overlay, ok := overlays.get(d.File.FSPath)
				if !ok {
					continue
				}
				e := overlay.endpoint
				if slices.ContainsFunc(e.Types, func(t *TypeInput) bool { return t.Name == d.Name }) {
					continue
				}
				e.Types = append(e.Types, &TypeInput{
					Name: d.Name,
					Doc:  strings.TrimSpace(d.Info.Doc),
					Fields: fns.Map(schemaType.Fields, func(f schema.StructField) *TypeFieldInput {
						return &TypeFieldInput{
							Name: f.Name.String(),
							Type: f.Type.String(),
							Doc:  strings.TrimSpace(f.Doc),
						}
					}),
				})
			}
		}
	}
	return &SyncResult{
		Services: services,
	}, nil
}
