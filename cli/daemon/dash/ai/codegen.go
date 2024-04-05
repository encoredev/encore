package ai

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/env"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	"encr.dev/v2/codegen/rewrite"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/parser/apis/directive"
)

// fmtComment prepends '//' to each line of the given comment and indents it with the given number of spaces.
func fmtComment(comment string, before, after int) string {
	if comment == "" {
		return ""
	}
	prefix := fmt.Sprintf("%s//%s", strings.Repeat(" ", before), strings.Repeat(" ", after))
	return prefix + strings.ReplaceAll(comment, "\n", "\n"+prefix)
}

// generateSrcFiles generates source files for the given services.
func generateSrcFiles(services []Service, app *apps.Instance) (map[paths.RelSlash]string, error) {
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
			filePath := paths.FS(app.Root()).JoinSlash(relFile)
			_, content := toSrcFile(filePath, s.Name, e.EndpointSource, e.TypeSource)
			files[relFile], err = addMissingFuncBodies(content)
			if err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

// addMissingFuncBodies adds a panic statement to functions that are missing a body.
// This is used to generate a valid Go source file when the user has not implemented
// the body of the endpoint functions.
func addMissingFuncBodies(content []byte) (string, error) {
	set := token.NewFileSet()
	rewriter := rewrite.New(content, 0)
	file, err := parser.ParseFile(set, "", content, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return "", err
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Body != nil {
				break
			}
			rewriter.Insert(n.End()-1, []byte("{\n    panic(\"not yet implemented\")\n}\n"))
		}
		return true
	})
	return string(rewriter.Data()), err
}

// writeFiles writes the generated source files to disk.
func writeFiles(services []Service, app *apps.Instance) ([]paths.RelSlash, error) {
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
	// Remove the divider and any formatting made by the imports tool
	src = append(src[:codeOffset], strings.Join(srcs, "\n")...)
	// Compute offset of the user defined code
	lines := strings.Split(string(src[:codeOffset]), "\n")
	return token.Position{
		Filename: filePath.ToIO(),
		Offset:   codeOffset,
		Line:     len(lines) - 1,
		Column:   0,
	}, src
}

// updateCode updates the source code fields of the EndpointInputs in the given services.
// if overwrite is set, the code will be regenerated from scratch and replace the existing code,
// otherwise, we'll modify the code in place
func updateCode(ctx context.Context, services []Service, app *apps.Instance, overwrite bool) (rtn *SyncResult, err error) {
	overlays, err := newOverlays(app, overwrite, services...)
	fset := token.NewFileSet()
	perrs := perr.NewList(ctx, fset, overlays.ReadFile)
	defer func() {
		perr.CatchBailout(recover())
		if rtn == nil {
			rtn = &SyncResult{
				Services: services,
			}
		}
		rtn.Errors = overlays.validationErrors(perrs)
	}()
	for p, olay := range overlays.list {
		astFile, err := parser.ParseFile(fset, p.ToIO(), olay.content, parser.ParseComments|parser.AllErrors)
		if err != nil {
			perrs.AddStd(err)
		}
		rewriter := rewrite.New(olay.content, int(astFile.FileStart))
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
		if olay.codeType == CodeTypeEndpoint {
			funcDecl, ok := funcByName[olay.endpoint.Name]
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
				if funcDecl.Body != nil {
					end = funcDecl.Body.Lbrace
				}
				rewriter.Replace(start, end, []byte(olay.endpoint.Render()))
			} else {
				if len(funcByName) > 0 {
					rewriter.Append([]byte("\n"))
				}
				rewriter.Append([]byte(olay.endpoint.Render()))
			}
			olay.content = rewriter.Data()
			content := string(olay.content[olay.headerOffset.Offset:])
			olay.endpoint.EndpointSource = strings.TrimSpace(content)
		} else {
			for _, typ := range olay.endpoint.Types {
				typeSpec := typeByName[typ.Name]
				code := typ.Render()
				if typeSpec != nil {
					start := typeSpec.Pos()
					if typeSpec.Doc != nil {
						start = typeSpec.Doc.Pos()
					}
					rewriter.Replace(start, typeSpec.End(), []byte(code))
				} else {
					rewriter.Append([]byte("\n\n" + code))
				}
			}
			olay.content = rewriter.Data()
			content := string(olay.content[olay.headerOffset.Offset:])
			olay.endpoint.TypeSource = strings.TrimSpace(content)
		}
	}
	goRoot := paths.RootedFSPath(env.EncoreGoRoot(), ".")
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedTypes | packages.NeedSyntax,
		Dir:  app.Root(),
		Env: append(os.Environ(),
			"GOOS="+runtime.GOOS,
			"GOARCH="+runtime.GOARCH,
			"GOROOT="+goRoot.ToIO(),
			"PATH="+goRoot.Join("bin").ToIO()+string(filepath.ListSeparator)+os.Getenv("PATH"),
		),
		Fset:    fset,
		Overlay: overlays.PkgOverlay(),
	}, fns.Map(overlays.pkgPaths(), paths.Pkg.String)...)
	if err != nil {
		return nil, err
	}
	for _, pkg := range pkgs {
		for _, err := range pkg.Errors {
			perrs.AddStd(err)
		}
	}
	return &SyncResult{
		Services: services,
	}, nil
}
