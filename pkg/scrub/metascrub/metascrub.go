// Package metascrub computes scrub paths for a metadata type.
package metascrub

import (
	"strconv"

	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"

	"encr.dev/pkg/scrub"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// New constructs a new Computer for the given metadata.
func New(md *meta.Data, log zerolog.Logger) *Computer {
	return &Computer{
		md:        md,
		log:       log.With().Str("component", "metascrub").Logger(),
		declCache: make(map[uint32]declResult),
	}
}

// Computer computes scrub paths for types, caching the computation.
// It can safely be reused across multiple types for the same metadata.
// It is not safe for concurrent use.
type Computer struct {
	md  *meta.Data
	log zerolog.Logger

	// declCache caches the scrub paths for encountered declarations
	declCache map[uint32]declResult

	// steps is a fail-safe to catch any potential infinite loops.
	steps int
}

// Compute computes the scrub paths for the given typ.
// It is not safe for concurrent use.
func (c *Computer) Compute(typ *schema.Type) []scrub.Path {
	if typ == nil {
		return nil
	}

	defer func() {
		if err := recover(); err != nil {
			c.log.Error().Stack().Msgf("metascrub.Compute panicked: %v", err)
		}
	}()

	c.steps = 0
	res := c.typ(typ)
	// Did we exceed the steps?
	if c.steps > maxSteps {
		c.log.Error().Msg("metascrub.Compute aborted due to exceeding max steps")
	}

	return res.scrub
}

const maxSteps = 10000

func (c *Computer) decl(id uint32) declResult {
	if res, ok := c.declCache[id]; ok {
		return res
	}

	// Mark that we're beginning to process this decl,
	// so we avoid infinite recursion.
	c.declCache[id] = declResult{}

	// Do the actual parsing.
	decl := c.md.Decls[id]
	res := c.typ(decl.Type)

	// We're done, cache the final result.
	c.declCache[id] = res
	return res
}

type declResult struct {
	scrub      []scrub.Path
	typeParams []typeParamPath
}

func (c *Computer) typ(typ *schema.Type) declResult {
	if typ == nil {
		return declResult{}
	}
	c.steps++
	if c.steps > maxSteps {
		return declResult{}
	}

	switch t := typ.Typ.(type) {
	case *schema.Type_Named:
		decl := c.decl(t.Named.Id)
		out := declResult{
			scrub: slices.Clone(decl.scrub),
		}

		// MultiGenericTwo[TypeParameter:1(B), string]
		for i, arg := range t.Named.TypeArguments {
			argRes := c.typ(arg)
			if len(argRes.scrub) == 0 && len(argRes.typeParams) == 0 {
				// Nothing to do
				continue
			}

			// For every type parameter, find the places
			// where it's used and copy it to the combined result.
			for _, declParam := range decl.typeParams {
				if declParam.paramIdx == uint32(i) {
					for _, s := range argRes.scrub {
						path := append(slices.Clone(declParam.p), s...)
						out.scrub = append(out.scrub, path)
					}
					for _, s := range argRes.typeParams {
						path := append(slices.Clone(declParam.p), s.p...)
						out.typeParams = append(out.typeParams, typeParamPath{
							p:        path,
							paramIdx: s.paramIdx,
						})
					}
				}
			}

		}
		return out

	case *schema.Type_Pointer:
		return c.typ(t.Pointer.Base)

	case *schema.Type_List:
		return c.typ(t.List.Elem)

	case *schema.Type_Map:
		key := c.typ(t.Map.Key)
		val := c.typ(t.Map.Value)

		combined := declResult{
			scrub:      make([]scrub.Path, 0, len(key.scrub)+len(val.scrub)),
			typeParams: make([]typeParamPath, 0, len(key.typeParams)+len(val.typeParams)),
		}
		for i, res := range [...]declResult{key, val} {
			kind := scrub.MapKey
			if i == 1 {
				kind = scrub.MapValue
			}
			for _, e := range res.scrub {
				path := append(scrub.Path{{Kind: kind}}, e...)
				combined.scrub = append(combined.scrub, path)
			}
			for _, e := range res.typeParams {
				path := append(scrub.Path{{Kind: kind}}, e.p...)
				combined.typeParams = append(combined.typeParams, typeParamPath{p: path, paramIdx: e.paramIdx})
			}
		}
		return combined

	case *schema.Type_Struct:
		var out declResult
		for _, f := range t.Struct.Fields {
			sensitive, fieldName, caseSensitive := isSensitive(f)
			if sensitive {
				// If the field is sensitive add it directly.
				out.scrub = append(out.scrub, scrub.Path{{Kind: scrub.ObjectField, FieldName: fieldName, CaseSensitive: caseSensitive}})
			} else {
				// Otherwise check the type and see if there's anything to scrub within it.
				fieldRes := c.typ(f.Typ)
				for _, e := range fieldRes.scrub {
					path := append(scrub.Path{{Kind: scrub.ObjectField, FieldName: fieldName, CaseSensitive: caseSensitive}}, e...)
					out.scrub = append(out.scrub, path)
				}
				for _, e := range fieldRes.typeParams {
					path := append(scrub.Path{{Kind: scrub.ObjectField, FieldName: fieldName, CaseSensitive: caseSensitive}}, e.p...)
					out.typeParams = append(out.typeParams, typeParamPath{p: path, paramIdx: e.paramIdx})
				}
			}
		}
		return out

	case *schema.Type_Builtin:
		// Nothing to do
		return declResult{}

	case *schema.Type_TypeParameter:
		return declResult{typeParams: []typeParamPath{{
			paramIdx: t.TypeParameter.ParamIdx,
		}}}

	default:
		c.log.Warn().Msgf("got unexpected schema.Type %T in metascrub, skipping", t)
		return declResult{}
	}
}

func isSensitive(f *schema.Field) (sensitive bool, fieldName string, caseSensitive bool) {
	fieldName = f.Name
	if f.JsonName != "" {
		fieldName = f.JsonName
		caseSensitive = true
	}
	fieldName = strconv.Quote(fieldName) // the scrub package wants exact byte matches

	for _, t := range f.Tags {
		if t.Key == "encore" {
			sensitive = t.Name == "sensitive" || slices.Contains(t.Options, "sensitive")
			break
		}
	}
	return sensitive, fieldName, caseSensitive
}

type typeParamPath struct {
	p        scrub.Path
	paramIdx uint32
}
