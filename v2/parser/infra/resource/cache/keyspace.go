package cache

import (
	"fmt"
	"go/ast"
	"strings"

	literals2 "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/internal/pkginfo"
	"encr.dev/v2/parser/internal/schema"
	"encr.dev/v2/parser/internal/schema/schemautil"
)

type Keyspace struct {
	Doc string // The documentation on the keyspace

	KeyType   schema.Type
	ValueType schema.Type
	Path      *KeyspacePath

	// The struct literal for the config. Used to inject additional configuration
	// at compile-time.
	ConfigLiteral *ast.CompositeLit
}

func (t *Keyspace) Kind() resource.Kind { return resource.CacheKeyspace }

var KeyspaceParser = &resource.Parser{
	Name:      "Cache Keyspace",
	DependsOn: []*resource.Parser{ClusterParser},

	RequiredImports: []string{"encore.dev/storage/cache"},
	Run: func(p *resource.Pass) []resource.Resource {
		var (
			names []pkginfo.QualifiedName
			specs = make(map[pkginfo.QualifiedName]*parseutil.ResourceCreationSpec)
		)
		for _, c := range keyspaceConstructors {
			name := pkginfo.QualifiedName{PkgPath: "encore.dev/storage/cache", Name: c.FuncName}
			names = append(names, name)

			numTypeArgs := 1
			if c.ValueKind != implicitValue {
				numTypeArgs = 2
			}

			c := c // capture for closure
			parseFn := func(d parseutil.ParseData) resource.Resource {
				return parseKeyspace(c, d)
			}

			spec := &parseutil.ResourceCreationSpec{
				AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
				MinTypeArgs: numTypeArgs,
				MaxTypeArgs: numTypeArgs,
				Parse:       parseFn,
			}
			specs[name] = spec
		}

		var resources []resource.Resource
		parseutil.FindPkgNameRefs(p.Pkg, names, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			spec := specs[name]
			r := parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
			if r != nil {
				resources = append(resources, r)
			}
		})
		return resources
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

func parseKeyspace(c cacheKeyspaceConstructor, d parseutil.ParseData) resource.Resource {
	errs := d.Pass.Errs
	constructorName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		errs.Addf(d.Call.Pos(), "%s expects 2 arguments", constructorName)
		return nil
	}

	// TODO(andre) Resolve cluster name

	cfgLit, ok := literals2.ParseStruct(errs, d.File, "cache.KeyspaceConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
	}

	keyPos := d.TypeArgs[0].ASTExpr().Pos()
	patternPos := cfgLit.Pos("KeyPattern")

	// Decode the config
	type decodedConfig struct {
		KeyPattern    string   `literal:",required"`
		DefaultExpiry ast.Expr `literal:",optional,dynamic"`
	}
	config := literals2.Decode[decodedConfig](errs, cfgLit)

	const reservedPrefix = "__encore"
	if strings.HasPrefix(config.KeyPattern, reservedPrefix) {
		errs.Addf(patternPos, "invalid KeyPattern: use of reserved prefix %q", reservedPrefix)
		return nil
	}

	path, err := ParseKeyspacePath(patternPos, config.KeyPattern)
	if err != nil {
		errs.Addf(patternPos, "cache.KeyspaceConfig got an invalid keyspace pattern: %v", err)
		return nil
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

		seenPathSegments := make(map[string]bool)
		for _, seg := range path.Segments {
			if seg.Type == Literal {
				continue
			}
			name := seg.Value
			seenPathSegments[name] = false

			if keyIsBuiltin && name != "key" {
				errs.Addf(patternPos, "KeyPattern parameter must be named ':key' for basic (non-struct) key types")
			}
		}

		// It should either be a builtin type or a (possibly pointer to a) named struct.
		if keyIsBuiltin {
			if err := validateBuiltin(builtinKey.Kind); err != nil {
				errs.Addf(keyPos, "%s has invalid key type parameter: %v", constructorName, err)
			}
		} else {
			ref, ok := schemautil.ResolveNamedStruct(keyType, false)
			if !ok {
				errs.Addf(keyPos, "%s has invalid key type parameter: must be a basic type or a named struct type", constructorName)
			} else {
				// Validate the struct fields.
				st := schemautil.ConcretizeWithTypeArgs(ref.Decl.Type, ref.TypeArgs).(schema.StructType)

				// Validate struct fields
				for _, f := range st.Fields {
					if f.IsAnonymous() {
						errs.Addf(keyPos, "key type %s is invalid: contains anonymous fields",
							ref.Decl)
						continue
					} else if !f.IsExported() {
						errs.Addf(keyPos, "key type %s has invalid field: field %q is unexported",
							ref.Decl, f.Name)
						continue
					}

					fieldName := f.Name.MustGet() // guaranteed by f.IsAnonymous check above
					if _, exists := seenPathSegments[fieldName]; !exists {
						errs.Addf(patternPos, "invalid use of key type %s: field %s not used in KeyPattern",
							ref.Decl, fieldName)
					} else {
						seenPathSegments[fieldName] = true
					}

					if builtin, ok := f.Type.(schema.BuiltinType); ok {
						if err := validateBuiltin(builtin.Kind); err != nil {
							errs.Addf(keyPos, "%s: key type %s is invalid: struct field %s: %v",
								constructorName, ref.Decl, fieldName, err)
						}
					} else {
						errs.Addf(keyPos, "%s: key type %s is invalid: struct field %s: not a basic type",
							constructorName, ref.Decl, fieldName)
					}
				}

				// Ensure all path segments are valid field names
				for fieldName, seen := range seenPathSegments {
					if !seen {
						errs.Addf(patternPos, "%s: invalid KeyPattern: field %s does not exist in key type %s",
							constructorName, fieldName, ref.Decl)
					}
				}
			}
		}
	}

	// Check the value type. We only need to do this for struct types since they need
	// to be represented as 'any' constraints. Basic type constructors enforce that the value type
	// through the Go type system and don't need to be verified again.
	if c.ValueKind == structValue {
		valuePos := d.TypeArgs[1].ASTExpr().Pos()
		if ref, ok := schemautil.ResolveNamedStruct(valueType, false); !ok {
			errs.Addf(valuePos, "%s has invalid value type parameter: must be a named struct type",
				constructorName)
		} else if ref.Pointers > 0 {
			errs.Addf(valuePos, "%s has invalid value type parameter: must not be a pointer type",
				constructorName)
		}
	}

	return &Keyspace{
		Doc:           d.Doc,
		ConfigLiteral: cfgLit.Lit(),
		Path:          path,
		KeyType:       keyType,
		ValueType:     valueType,
	}
}
