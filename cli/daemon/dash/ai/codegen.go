package ai

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	"encr.dev/v2/codegen/rewrite"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/parser/apis/directive"
)

func fmtComment(comment string, before, after int) string {
	if comment == "" {
		return ""
	}
	prefix := fmt.Sprintf("%s//%s", strings.Repeat(" ", before), strings.Repeat(" ", after))
	return prefix + strings.ReplaceAll(comment, "\n", "\n"+prefix)
}

func generateSrcFiles(services []ServiceInput, app *apps.Instance) (map[paths.RelSlash]string, error) {
	svcPaths, err := newServicePaths(app)
	if err != nil {
		return nil, err
	}
	files := map[paths.RelSlash]string{}
	for _, s := range services {
		if svcPaths.IsNew(s.Name) {
			relFile, err := svcPaths.RelFileName(s.Name, s.Name)
			if err != nil {
				return nil, err
			}
			file := paths.FS(app.Root()).JoinSlash(relFile)
			err = os.MkdirAll(file.Dir().ToIO(), 0755)
			if err != nil {
				return nil, err
			}
			files[relFile] = fmt.Sprintf("%s\npackage %s\n", fmtComment(s.Doc, 0, 1), strings.ToLower(s.Name))
		}
		for _, e := range s.Endpoints {
			relFile, err := svcPaths.RelFileName(s.Name, e.Name)
			if err != nil {
				return nil, err
			}
			file := paths.FS(app.Root()).JoinSlash(relFile)
			_, content := toSrcFile(file, s.Name, e.EndpointSource, e.TypeSource)
			files[relFile] = string(content)
		}
	}
	return files, nil
}

func writeFiles(services []ServiceInput, app *apps.Instance) ([]paths.RelSlash, error) {
	files, err := generateSrcFiles(services, app)
	if err != nil {
		return nil, err
	}
	for fileName, content := range files {
		root := paths.FS(app.Root())
		err = os.WriteFile(root.JoinSlash(fileName).ToIO(), []byte(content), 0644)
		if err != nil {
			return nil, err
		}
	}
	return maps.Keys(files), nil
}

// toSrcFile wraps a code fragment in a package declaration and adds missing imports
// using the goimports tool.
func toSrcFile(filePath paths.FS, svc string, srcs ...string) (offset token.Position, data []byte) {
	const divider = "// @code-start\n"
	header := fmt.Sprintf("package %s\n\n", strings.ToLower(svc))
	src := []byte(header + divider + strings.Join(srcs, "\n"))
	importedSrc, err := imports.Process(filePath.ToIO(), src, &imports.Options{
		Comments:  true,
		TabIndent: false,
		TabWidth:  4,
	})
	// We don't need to handle the error here, as we'll catch parser/scanner errors in a later
	// phase. This is just a best effort to import missing packages.
	if err == nil {
		src = importedSrc
	}
	codeOffset := bytes.Index(src, []byte(divider))
	src = append(src[:codeOffset], src[codeOffset+len(divider):]...)
	lines := strings.Split(string(src[:codeOffset]), "\n")
	return token.Position{
		Filename: filePath.ToIO(),
		Offset:   codeOffset,
		Line:     len(lines) - 1,
		Column:   0,
	}, src
}

// updateCode updates the source code fields of the EndpointInputs in the given services.
// if overwrite is set, the code will be regeneratad from scratch and replace the existing code,
// otherwise, we'll modify the code in place
func updateCode(ctx context.Context, services []ServiceInput, app *apps.Instance, overwrite bool) (*SyncResult, error) {
	overlays, err := newOverlays(app, overwrite, services...)
	fset := token.NewFileSet()
	perrs := perr.NewList(ctx, fset)
	pkgs, err := packages.Load(&packages.Config{
		Mode:    packages.NeedTypes | packages.NeedSyntax,
		Dir:     app.Root(),
		Fset:    fset,
		Overlay: fns.TransformMapKeys(overlays.files(), paths.FS.ToIO),
	}, fns.Map(overlays.pkgPaths(), paths.Pkg.String)...)
	if err != nil {
		return nil, err
	}
	pathToAst := map[paths.FS]*ast.File{}
	var errs []ValidationError
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			fPos := pkg.Fset.Position(f.Pos())
			pathToAst[paths.FS(fPos.Filename)] = f
		}
		for _, e := range pkg.Errors {
			file, _, _ := strings.Cut(e.Pos, ":")
			if info, ok := overlays.get(paths.FS(file)); ok {
				errs = append(errs, formatError(info, e)...)
			}
		}
	}

	if len(errs) > 0 {
		return &SyncResult{
			Errors:   errs,
			Services: services,
		}, nil
	}

	for p, ep := range overlays.list {
		astFile := pathToAst[p]
		rewriter := rewrite.New(ep.content, int(astFile.FileStart))
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
		if ep.codeType == CodeTypeEndpoint {
			funcDecl, ok := funcByName[ep.endpoint.Name]
			if !ok {
				for _, f := range funcByName {
					dir, _, _ := directive.Parse(perrs, f.Doc)
					if dir != nil && dir.Name == "api" {
						funcDecl = f
						break
					}
				}
			}
			if funcDecl != nil {
				start := funcDecl.Pos()
				if funcDecl.Doc != nil {
					start = funcDecl.Doc.Pos()
				}
				end := funcDecl.End()
				withBody := true
				if funcDecl.Body != nil {
					withBody = false
					end = funcDecl.Body.Lbrace
				}
				rewriter.Replace(start, end, []byte(ep.endpoint.Render(withBody)))
			} else {
				if len(funcByName) > 0 {
					rewriter.Append([]byte("\n"))
				}
				rewriter.Append([]byte(ep.endpoint.Render(true)))
			}
			content := string(rewriter.Data()[ep.headerOffset.Offset:])
			ep.endpoint.EndpointSource = strings.TrimSpace(content)
		} else {
			for _, typ := range ep.endpoint.Types {
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
			content := string(rewriter.Data()[ep.headerOffset.Offset:])
			ep.endpoint.TypeSource = strings.TrimSpace(content)
		}
	}
	return &SyncResult{
		Services: services,
	}, nil
}
