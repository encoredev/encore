package config

import (
	"go/ast"
	"go/token"

	"encr.dev/internal/paths"
	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

// Load represents a config load statement.
type Load struct {
	AST  *ast.CallExpr
	File *pkginfo.File

	// Type is the type of the config struct being loaded.
	// It's guaranteed to be a (possibly pointer to a) named struct type.
	Type schema.Type

	// FuncCall is the AST node that represents the config.Load expression.
	FuncCall *ast.CallExpr
}

func (*Load) Kind() resource.Kind         { return resource.ConfigLoad }
func (l *Load) Package() *pkginfo.Package { return l.File.Pkg }
func (l *Load) ASTExpr() ast.Expr         { return l.AST }
func (l *Load) Pos() token.Pos            { return l.AST.Pos() }
func (l *Load) End() token.Pos            { return l.AST.End() }

var LoadParser = &resourceparser.Parser{
	Name: "ConfigLoad",

	InterestingImports: []paths.Pkg{"encore.dev/config"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/config", Name: "Load"}

		spec := &parseutil.ReferenceSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 1,
			Parse:       parseLoad,
		}
		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseLoad(d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs

	if len(d.Call.Args) > 0 {
		errs.Add(errInvalidLoad.AtGoNode(d.Call))
		return
	}

	if len(d.TypeArgs) != 1 {
		errs.Add(errInvalidConfigType.AtGoNode(d.Call))
	}

	// Resolve the named struct used for the config type
	ref, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		errs.Add(errInvalidConfigType.AtGoNode(d.TypeArgs[0].ASTExpr()))
		return
	}

	load := &Load{
		AST:      d.Call,
		File:     d.File,
		Type:     d.TypeArgs[0],
		FuncCall: d.Call,
	}

	concrete := schemautil.ConcretizeWithTypeArgs(ref.ToType(), ref.TypeArgs)
	walkCfgToVerify(d.Pass.Errs, load, concrete, false)

	d.Pass.RegisterResource(load)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, load)
	}
}

func walkCfgToVerify(errs *perr.List, load *Load, decl schema.Type, insideConfigValue bool) {
	switch decl := decl.(type) {
	case schema.BuiltinType:
		// no-op ok
	case schema.NamedType:
		if decl.DeclInfo.File.Pkg.ImportPath == "encore.dev/config" {
			if insideConfigValue {
				errs.Add(errNestedValueUsage.
					AtGoNode(decl.ASTExpr(), errors.AsError("cannot use config.Value inside a config.Value")).
					AtGoNode(load, errors.AsHelp("config loaded here")),
				)
			}

			switch decl.DeclInfo.Name {
			case "Value", "Values":
				// Value / Values are magic wrappers that are used to indicate a realtime
				// config update
				if len(decl.TypeArgs) > 0 {
					walkCfgToVerify(errs, load, decl.TypeArgs[0], true)

					// return so we don't verify the standard type
					return
				}
			}
		} else {
			insideConfigValue = false
		}

		walkCfgToVerify(errs, load, decl.Decl().Type, insideConfigValue)
	case schema.PointerType:
		walkCfgToVerify(errs, load, decl.Elem, false)
	case schema.ListType:
		walkCfgToVerify(errs, load, decl.Elem, false)
	case schema.MapType:
		walkCfgToVerify(errs, load, decl.Key, false)
		walkCfgToVerify(errs, load, decl.Value, false)
	case schema.StructType:
		for _, field := range decl.Fields {
			if !field.IsExported() {
				errs.Add(errUnexportedField.
					AtGoNode(field.AST).
					AtGoNode(load, errors.AsHelp("config loaded here")),
				)
			} else if field.IsAnonymous() {
				errs.Add(errAnonymousField.
					AtGoNode(field.AST).
					AtGoNode(load, errors.AsHelp("config loaded here")),
				)
			} else {
				walkCfgToVerify(errs, load, field.Type, false)
			}
		}
	default:
		errs.Add(errInvalidConfigTypeUsed.
			AtGoNode(decl.ASTExpr(), errors.AsError("unsupported type")).
			AtGoNode(load, errors.AsHelp("config loaded here")),
		)
	}
}
