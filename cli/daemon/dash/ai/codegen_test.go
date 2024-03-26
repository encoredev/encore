package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"

	"encr.dev/internal/env"
	"encr.dev/pkg/paths"
	"encr.dev/v2/codegen/rewrite"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/resource/resourceparser"
)

func TestLoader(t *testing.T) {
	ctx := context.Background()
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	dir := "/Users/stefan/src/bauta"
	pc := &parsectx.Context{
		Ctx: ctx,
		Log: zerolog.Logger{},
		Build: parsectx.BuildInfo{
			Experiments: nil,
			GOROOT:      paths.RootedFSPath(env.EncoreGoRoot(), "."),
			GOARCH:      runtime.GOARCH,
			GOOS:        runtime.GOOS,
		},
		MainModuleDir: paths.RootedFSPath(dir, "."),
		FS:            fs,
		ParseTests:    false,
		Errs:          errs,
		Overlay: map[string][]byte{
			"birds/outer.go": []byte(`
package birds

import (
	"context"
)

type Birder struct {
	Name time.Time
}

// encore:api auth method=POST path=/bannn
func bannn(ctx context.Context, req BirdRequest) (Birder, error) {
	panic("not implemented")
}

`)}}
	loader := pkginfo.New(pc)
	pkg, _ := loader.LoadPkg(token.NoPos, "encore.app/birds")
	schemaParser := schema.NewParser(pc, loader)
	pass := &resourceparser.Pass{
		Context:      pc,
		SchemaParser: schemaParser,
		Pkg:          pkg,
	}
	apis.Parser.Run(pass)
	fmt.Print("done")

}

func TestGenerator(t *testing.T) {
	c := qt.New(t)
	var services []ServiceInput
	err := json.Unmarshal([]byte(`
[
      {
        "name": "birds",
        "doc": "The birds service maintains the database of birds observed in the system",
        "endpoints": [
          {
			"name": "OtherBiiird"
          },
          {
            "name": "Biiird",
            "doc": "Add a new bird observation to the system.\nErrors:\n\n\tAlreadyExists:",
            "method": "POST",
            "visibility": "auth",
            "path": [
              {
                "type": "literal",
                "value": "birrds"
              }
            ],
            "requestType": "BirdRequest",
            "responseType": "BirdResponse",
            "errors": [
              {
                "code": "AlreadyExists"
              }
            ],
            "types": [
              {
                "name": "BirdRequest",
                "fields": [
                  {
                    "name": "Species",
                    "wireName": "Species",
                    "type": "string",
                    "location": "body",
                    "doc": "The species of the observed bird. It must be a recognized species name.\n"
                  },
                  {
                    "name": "Location",
                    "wireName": "Location",
                    "type": "string",
                    "location": "body",
                    "doc": "A description of the location where the bird was observed.\n"
                  },
                  {
                    "name": "Unknown",
                    "wireName": "Unknown",
                    "type": "Unknown",
                    "location": "body",
                    "doc": "The date and time when the bird was observed.\n"
                  },
                  {
                    "name": "ObservedAt",
                    "wireName": "ObservedAt",
                    "type": "time.Time",
                    "location": "body"
                  }
                ]
              },
              {
                "name": "BirdResponse",
                "doc": "Response confirming a bird observation\n",
                "fields": [
                  {
                    "name": "ID",
                    "wireName": "ID",
                    "type": "string",
                    "location": "body",
                    "doc": "A unique identifier for the observation.\n"
                  },
                  {
                    "name": "ConfirmationID",
                    "wireName": "ConfirmationID",
                    "type": "string",
                    "location": "body"
                  },
                  {
                    "name": "Status",
                    "wireName": "Status",
                    "type": "string",
                    "location": "body",
                    "doc": "The status of the observation submission, e.g., \"Success\".\n"
                  }
                ]
              },
              {
                "name": "Unknown",
                "doc": "Attributes of a bird observation\n"
              }
            ],
            "language": "GO",
            "typeSource": "type Unknown struct {}\n// Attributes of a bird observation\ntype BirdRequest struct {\n\n  // The species of the observed bird. It must be a recognized species name.\n  Species string\n\n  // A description of the location where the bird was observed.\n  Location string\n\n  // The date and time when the bird was observed.\n  Unknown Unknown \n  ObservedAt time.Time\n\n}\n\n// Response confirming a bird observation\ntype BirdResponse struct {\n\n ID string // A unique identifier for the observation.\n  ConfirmationID string\n\n  // The status of the observation submission, e.g., \"Success\".\n  Status string\n\n}\n",
            "endpointSource": "func a() {} \n// Add a new bird observation to the system.\n// Errors:\n//\n//\tAlreadyExists:\n//\n// encore:api auth method=POST path=/birrds\nfunc Biiird(ctx context.Context, req BirdRequest) (BirdResponse, error) {\n\tpanic(\"not implemented\")\n}\n"
          }
        ]
      }
    ]
`), &services)
	c.Assert(err, qt.IsNil)

	dir := "/Users/stefan/src/bauta"
	files := map[string][]byte{}
	pathToEp := map[string]*EndpointInput{}
	for _, s := range services {
		for _, e := range s.Endpoints {
			prefix := filepath.Join(dir, s.Name, e.Name)
			path := prefix + "_ep_tmp.go"
			files[path] = toSrcFile(s.Name, HEADER_DIVIDER, e.EndpointSource)
			pathToEp[path] = e
			path = prefix + "_types_tmp.go"
			files[path] = toSrcFile(s.Name, HEADER_DIVIDER, e.TypeSource)
			pathToEp[path] = e
		}
	}
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Mode:    packages.NeedTypes | packages.NeedSyntax,
		Dir:     dir,
		Fset:    fset,
		Overlay: files,
	}, "encore.app/birds")
	c.Assert(err, qt.IsNil)
	pathToAst := map[string]*ast.File{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			fPos := pkg.Fset.Position(f.Pos())
			pathToAst[fPos.Filename] = f
		}
	}

	for path, ep := range pathToEp {
		astFile := pathToAst[path]
		rewriter := rewrite.New(files[path], int(astFile.FileStart))
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
		if strings.HasSuffix(path, "_ep_tmp.go") {
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
				panic("no header divider")
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
				panic("no header divider")
			}
			ep.TypeSource = strings.TrimSpace(content)
		}
	}

	fmt.Println(services)
}
