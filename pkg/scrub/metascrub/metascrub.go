// Package metascrub computes scrub paths for a metadata type.
package metascrub

import (
	"slices"
	"strconv"

	"github.com/rs/zerolog"

	"encr.dev/pkg/scrub"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// New constructs a new Computer for the given metadata.
func New(md *meta.Data, log zerolog.Logger) *Computer {
	return &Computer{
		md:        md,
		log:       log.With().Str("component", "metascrub").Logger(),
		declCache: make(map[declCacheKey]declResult),
	}
}

// Computer computes scrub paths for types, caching the computation.
// It can safely be reused across multiple types for the same metadata.
// It is not safe for concurrent use.
type Computer struct {
	md  *meta.Data
	log zerolog.Logger

	// declCache caches the scrub paths for encountered declarations
	declCache map[declCacheKey]declResult
}

type Desc struct {
	Payload []scrub.Path
	Headers []string
}

type ParseMode int

const (
	// AuthHandler specifies that the type is an auth handler.
	AuthHandler ParseMode = 1 << iota
)

// Compute computes the scrub paths for the given typ.
// It is not safe for concurrent use.
func (c *Computer) Compute(typ *schema.Type, mode ParseMode) Desc {
	if typ == nil {
		return Desc{}
	}

	defer func() {
		if err := recover(); err != nil {
			c.log.Error().Stack().Msgf("metascrub.Compute panicked: %v", err)
		}
	}()

	p := &typeParser{c: c, mode: mode}
	res := p.typ(typ)

	// Did we exceed the steps?
	if p.steps > maxSteps {
		c.log.Error().Msg("metascrub.Compute aborted due to exceeding max steps")
	}

	return Desc{
		Payload: res.scrub,
		Headers: res.headers,
	}
}

const maxSteps = 10000

type typeParser struct {
	c    *Computer
	mode ParseMode

	// steps is a fail-safe to catch any potential infinite loops.
	steps int
}

type declCacheKey struct {
	id   uint32
	mode ParseMode
}

func (p *typeParser) decl(id uint32) declResult {
	key := declCacheKey{id, p.mode}
	if res, ok := p.c.declCache[key]; ok {
		return res
	}

	// Mark that we're beginning to process this decl,
	// so we avoid infinite recursion.
	p.c.declCache[key] = declResult{}

	// Do the actual parsing.
	decl := p.c.md.Decls[id]
	res := p.typ(decl.Type)

	// We're done, cache the final result.
	p.c.declCache[key] = res
	return res
}

type declResult struct {
	scrub      []scrub.Path
	typeParams []typeParamPath
	headers    []string
}

func (p *typeParser) typ(typ *schema.Type) declResult {
	if typ == nil {
		return declResult{}
	}
	p.steps++
	if p.steps > maxSteps {
		return declResult{}
	}

	switch t := typ.Typ.(type) {
	case *schema.Type_Named:
		decl := p.decl(t.Named.Id)
		out := declResult{
			// Clone the paths since we're modifying them.
			scrub: slices.Clone(decl.scrub),

			// Copy the headers directly since they never get modified,
			// since we only care about the top-level type's headers.
			headers: decl.headers,
		}

		for i, arg := range t.Named.TypeArguments {
			argRes := p.typ(arg)
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
		return p.typ(t.Pointer.Base)

	case *schema.Type_List:
		return p.typ(t.List.Elem)

	case *schema.Type_Map:
		key := p.typ(t.Map.Key)
		val := p.typ(t.Map.Value)

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

			// For Auth Handlers everything is sensitive.
			if (p.mode & AuthHandler) != 0 {
				sensitive = true
			}

			if sensitive {
				// If the field is sensitive add it directly.
				out.scrub = append(out.scrub, scrub.Path{{Kind: scrub.ObjectField, FieldName: fieldName, CaseSensitive: caseSensitive}})

				// If this is a header field, add it to the headers to scrub.
				if headerName, ok := isHeader(f); ok {
					out.headers = append(out.headers, headerName)
				}
			} else {
				// Otherwise check the type and see if there's anything to scrub within it.
				fieldRes := p.typ(f.Typ)
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
		p.c.log.Warn().Msgf("got unexpected schema.Type %T in metascrub, skipping", t)
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

func isHeader(f *schema.Field) (headerName string, ok bool) {
	for _, t := range f.Tags {
		if t.Key == "header" {
			name := t.Name
			if name == "" {
				name = f.Name
			}
			return name, true
		}
	}
	return "", false
}

type typeParamPath struct {
	p        scrub.Path
	paramIdx uint32
}
