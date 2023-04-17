package caches

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

type Keyspace struct {
	AST     *ast.CallExpr
	Doc     string        // The documentation on the keyspace
	File    *pkginfo.File // File the keyspace is declared in.
	Cluster pkginfo.QualifiedName

	KeyType   schema.Type
	ValueType schema.Type
	Path      *resourcepaths.Path

	// The struct literal for the config. Used to inject additional configuration
	// at compile-time.
	ConfigLiteral *ast.CompositeLit
}

func (k *Keyspace) Kind() resource.Kind       { return resource.CacheKeyspace }
func (k *Keyspace) Package() *pkginfo.Package { return k.File.Pkg }
func (k *Keyspace) ASTExpr() ast.Expr         { return k.AST }
func (k *Keyspace) Pos() token.Pos            { return k.AST.Pos() }
func (k *Keyspace) End() token.Pos            { return k.AST.End() }

var KeyspaceParser = &resourceparser.Parser{
	Name: "Cache Keyspace",

	InterestingImports: []paths.Pkg{"encore.dev/storage/cache"},
	Run: func(p *resourceparser.Pass) {
		var (
			names []pkginfo.QualifiedName
			specs = make(map[pkginfo.QualifiedName]*parseutil.ReferenceSpec)
		)
		for _, c := range keyspaceConstructors {
			name := pkginfo.QualifiedName{PkgPath: "encore.dev/storage/cache", Name: c.FuncName}
			names = append(names, name)

			numTypeArgs := 1
			if c.ValueKind != implicitValue {
				numTypeArgs = 2
			}

			c := c // capture for closure
			parseFn := func(d parseutil.ReferenceInfo) {
				parseKeyspace(c, d)
			}

			spec := &parseutil.ReferenceSpec{
				MinTypeArgs: numTypeArgs,
				MaxTypeArgs: numTypeArgs,
				Parse:       parseFn,
			}
			specs[name] = spec
		}

		parseutil.FindPkgNameRefs(p.Pkg, names, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			spec := specs[name]
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

type valueKind int

const (
	// implicitValue means the user does not specify the value type as
	// a type parameter, but is implicit from the constructor name.
	// If used, the ImplicitValueType field must be set on the constructor spec.
	implicitValue valueKind = iota

	// basicValue means the constructor supports basic types only.
	basicValue

	// structValue means the constructor supports struct values only.
	structValue
)

// cacheKeyspaceConstructor describes a particular cache keyspace constructor.
type cacheKeyspaceConstructor struct {
	FuncName          string
	ValueKind         valueKind
	ImplicitValueType schema.Type
}

var keyspaceConstructors = []cacheKeyspaceConstructor{
	{"NewStringKeyspace", implicitValue, schema.BuiltinType{Kind: schema.String}},
	{"NewIntKeyspace", implicitValue, schema.BuiltinType{Kind: schema.Int64}},
	{"NewFloatKeyspace", implicitValue, schema.BuiltinType{Kind: schema.Float64}},
	{"NewListKeyspace", basicValue, nil},
	{"NewSetKeyspace", basicValue, nil},
	{"NewStructKeyspace", structValue, nil},
}

func parseKeyspace(c cacheKeyspaceConstructor, d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs
	constructorName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		errs.Add(errExpectsTwoArgs(constructorName, len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	clusterRef, ok := d.File.Names().ResolvePkgLevelRef(d.Call.Args[0])
	if !ok {
		errs.Add(ErrCouldNotResolveCacheCluster.AtGoNode(d.Call.Args[0]))
		return
	}

	cfgLit, ok := literals.ParseStruct(errs, d.File, "cache.KeyspaceConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}

	keyNode := d.TypeArgs[0].ASTExpr()
	patternNode := cfgLit.Expr("KeyPattern")

	// Decode the config
	type decodedConfig struct {
		KeyPattern    string   `literal:",required"`
		DefaultExpiry ast.Expr `literal:",optional,dynamic"`
	}
	config := literals.Decode[decodedConfig](errs, cfgLit, nil)

	const reservedPrefix = "__encore"
	if strings.HasPrefix(config.KeyPattern, reservedPrefix) {
		errs.Add(errPrefixReserved.AtGoNode(patternNode))
		return
	}

	path, ok := resourcepaths.Parse(
		errs,
		cfgLit.Pos("KeyPattern")+1, // + 1 to offset the opening \"
		config.KeyPattern,
		resourcepaths.Options{
			AllowWildcard: false,
			AllowFallback: false,
			PrefixSlash:   false,
		},
	)
	if !ok {
		return
	}

	// Get key and value types.
	keyType := d.TypeArgs[0]
	var valueType schema.Type
	if c.ValueKind == implicitValue {
		valueType = c.ImplicitValueType
	} else {
		valueType = d.TypeArgs[1]
	}

	// Check that the key type is a basic type or a named struct
	{
		validateBuiltin := func(b schema.BuiltinKind) error {
			switch b {
			case schema.Any:
				return fmt.Errorf("'any'/'interface{}' is not supported")
			case schema.JSON:
				return fmt.Errorf("json.RawMessage is not supported")
			case schema.Float64, schema.Float32:
				return fmt.Errorf("floating point values are not supported")
			}
			return nil
		}

		builtinKey, keyIsBuiltin := keyType.(schema.BuiltinType)

		unusedSegments := make(map[string]resourcepaths.Segment)
		for _, seg := range path.Segments {
			if seg.Type == resourcepaths.Literal {
				continue
			}
			name := seg.Value
			unusedSegments[name] = seg

			if keyIsBuiltin && name != "key" {
				errs.Add(errKeyPatternMustBeNamedKey.AtGoNode(seg))
			}
		}

		// It should either be a builtin type or a (possibly pointer to a) named struct.
		if keyIsBuiltin {
			if err := validateBuiltin(builtinKey.Kind); err != nil {
				errs.Add(errInvalidKeyTypeParameter(constructorName, err.Error()).AtGoNode(keyNode))
			}
		} else {
			ref, ok := schemautil.ResolveNamedStruct(keyType, false)
			if !ok {
				errs.Add(errInvalidKeyTypeParameter(constructorName, "must be a basic type or a named struct type").AtGoNode(keyNode))
			} else if ref.Pointers > 0 {
				errs.Add(errInvalidKeyTypeParameter(constructorName, "must not be a pointer type").AtGoNode(keyNode))
			} else {
				// Validate the struct fields.
				st := schemautil.ConcretizeWithTypeArgs(ref.Decl.Type, ref.TypeArgs).(schema.StructType)

				// Validate struct fields
				for _, f := range st.Fields {
					if f.IsAnonymous() {
						errs.Add(errKeyContainsAnonymousFields.AtGoNode(f.AST))
						continue
					} else if !f.IsExported() {
						errs.Add(errKeyContainsUnexportedFields.AtGoNode(f.AST))
						continue
					}

					fieldName := f.Name.MustGet() // guaranteed by f.IsAnonymous check above
					if _, exists := unusedSegments[fieldName]; !exists {
						errs.Add(errFieldNotUsedInKeyPattern(fieldName).AtGoNode(f.AST).AtGoNode(patternNode))
					} else {
						delete(unusedSegments, fieldName)
					}

					if builtin, ok := f.Type.(schema.BuiltinType); ok {
						if err := validateBuiltin(builtin.Kind); err != nil {
							errs.Add(errFieldIsInvalid(fieldName, err.Error()).AtGoNode(f.AST))
						}
					} else {
						errs.Add(errFieldIsInvalid(fieldName, "must be a basic type").
							AtGoNode(f.AST, errors.AsError(fmt.Sprintf("found %s", f.Type))).
							AtGoNode(keyNode, errors.AsHelp("instantiated here")))
					}
				}

				// Ensure all path segments are valid field names
				for fieldName, segment := range unusedSegments {
					errs.Add(errFieldDoesntExist(fieldName, ref.Decl).AtGoNode(segment).AtGoNode(ref.Decl.AST.Name))
				}
			}
		}
	}

	// Check the value type. We only need to do this for struct types since they need
	// to be represented as 'any' constraints. Basic type constructors enforce that the value type
	// through the Go type system and don't need to be verified again.
	if c.ValueKind == structValue {
		if ref, ok := schemautil.ResolveNamedStruct(valueType, false); !ok {
			errs.Add(errMustBeANamedStructType.AtGoNode(d.TypeArgs[1].ASTExpr()))
		} else if ref.Pointers > 0 {
			errs.Add(errStructMustNotBePointer.AtGoNode(d.TypeArgs[1].ASTExpr()))
		}
	}

	ks := &Keyspace{
		AST:           d.Call,
		Doc:           d.Doc,
		File:          d.File,
		Cluster:       clusterRef,
		ConfigLiteral: cfgLit.Lit(),
		Path:          path,
		KeyType:       keyType,
		ValueType:     valueType,
	}

	d.Pass.RegisterResource(ks)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, ks)
	}
}
