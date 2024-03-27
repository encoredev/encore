package ai

import (
	"context"
	"errors"
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

func parseErrorDoc(doc string) (string, []*ErrorInput) {
	var errs []*ErrorInput
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
			errs = append(errs, &ErrorInput{
				Code: errID,
				Doc:  strings.TrimSpace(doc),
			})
		} else if len(errs) > 0 {
			errs[len(errs)-1].Doc += "\n" + line
		}
	}
	return strings.Join(lines[:errStart], "\n"), errs
}

func parseCode(ctx context.Context, app *apps.Instance, services []ServiceInput) (rtn *SyncResult, err error) {
	overlays, err := newOverlays(app, false, services...)
	if err != nil {
		return nil, err
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
		Overlay:       overlays.files(),
	}
	defer func() {
		perr.CatchBailout(recover())
		if rtn == nil {
			rtn = &SyncResult{
				Services: services,
			}
		}
		rtn.Errors, err = formatSrcErrList(overlays, errs)
	}()

	loader := pkginfo.New(pc)

	pkgs := map[paths.Pkg]*pkginfo.Package{}
	for _, pkg := range overlays.pkgPaths() {
		var ok bool
		pkgs[pkg], ok = loader.LoadPkg(token.NoPos, pkg)
		if !ok {
			return nil, errors.New("failed to load package")
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
				overlay, ok := overlays.get(r.File.FSPath)
				if !ok {
					continue
				}
				e := overlay.endpoint
				e.Doc, e.Errors = parseErrorDoc(r.Doc)
				e.Name = r.Name
				e.Method = r.HTTPMethods[0]
				e.Path = toPathSegments(r.Path)
				e.Visibility = VisibilityType(r.Access)
				e.Language = "GO"
				e.Path = toPathSegments(r.Path)
				e.Types = []*TypeInput{}
				if nr, ok := r.Request.(schema.NamedType); ok {
					e.RequestType = r.Request.String()
					e.Types = append(e.Types, &TypeInput{
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
					e.Types = append(e.Types, &TypeInput{
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
	return &SyncResult{
		Services: services,
	}, nil
}
