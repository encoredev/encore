// Package typescrub computes scrub paths for schema types.
package typescrub

import (
	"slices"
	"strconv"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/scrub"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
)

// New constructs a new Computer.
func NewComputer(log zerolog.Logger) *Computer {
	return &Computer{
		log:       log.With().Str("component", "typescrub").Logger(),
		declCache: make(map[declCacheKey]declResult),
	}
}

// Computer computes scrub paths for types, caching the computation.
// It can safely be reused across multiple types.
// It is not safe for concurrent use.
type Computer struct {
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
	// DisableScrubbing specifies that scrubbing should be disabled.
	// Used for local development.
	DisableScrubbing
)

// Compute computes the scrub paths for the given typ.
// It is not safe for concurrent use.
func (c *Computer) Compute(typ schema.Type, mode ParseMode) Desc {
	if typ == nil || (mode&DisableScrubbing) != 0 {
		return Desc{}
	}

	p := &typeParser{c: c, mode: mode}
	res := p.typ(typ)

	// Did we exceed the steps?
	if p.steps > maxSteps {
		c.log.Error().Msg("typescrub.Compute aborted due to exceeding max steps")
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
	qn   pkginfo.QualifiedName
	mode ParseMode
}

func (p *typeParser) decl(d *schema.TypeDecl) declResult {
	key := declCacheKey{d.Info.QualifiedName(), p.mode}
	if res, ok := p.c.declCache[key]; ok {
		return res
	}

	// Mark that we're beginning to process this decl,
	// so we avoid infinite recursion.
	p.c.declCache[key] = declResult{}

	// Do the actual parsing.
	res := p.typ(d.Type)

	// We're done, cache the final result.
	p.c.declCache[key] = res
	return res
}

type declResult struct {
	scrub      []scrub.Path
	typeParams []typeParamPath
	headers    []string
}

func (p *typeParser) typ(typ schema.Type) declResult {
	if typ == nil {
		return declResult{}
	}
	p.steps++
	if p.steps > maxSteps {
		return declResult{}
	}

	switch t := typ.(type) {
	case schema.NamedType:
		decl := p.decl(t.Decl())
		out := declResult{
			// Clone the paths since we're modifying them.
			scrub: slices.Clone(decl.scrub),

			// Copy the headers directly since they never get modified,
			// since we only care about the top-level type's headers.
			headers: decl.headers,
		}

		for i, arg := range t.TypeArgs {
			argRes := p.typ(arg)
			if len(argRes.scrub) == 0 && len(argRes.typeParams) == 0 {
				// Nothing to do
				continue
			}

			// For every type parameter, find the places
			// where it's used and copy it to the combined result.
			for _, declParam := range decl.typeParams {
				if declParam.paramIdx == i {
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

	case schema.PointerType:
		return p.typ(t.Elem)

	case schema.OptionType:
		return p.typ(t.Value)

	case schema.ListType:
		return p.typ(t.Elem)

	case schema.MapType:
		key := p.typ(t.Key)
		val := p.typ(t.Value)

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

	case schema.StructType:
		var out declResult
		for _, f := range t.Fields {
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
				fieldRes := p.typ(f.Type)
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

	case schema.BuiltinType:
		// Nothing to do
		return declResult{}

	case schema.TypeParamRefType:
		return declResult{typeParams: []typeParamPath{{
			paramIdx: t.Index,
		}}}

	default:
		p.c.log.Warn().Msgf("got unexpected schema.Type %T in typescrub, skipping", t)
		return declResult{}
	}
}

func isSensitive(f schema.StructField) (sensitive bool, fieldName string, caseSensitive bool) {
	fieldName = f.Name.GetOrElse("")

	// Check for json tag override of field name
	if jsonTag, err := f.Tag.Get("json"); err == nil && jsonTag.Name != "" && jsonTag.Name != "-" {
		fieldName = jsonTag.Name
		caseSensitive = true
	}

	fieldName = strconv.Quote(fieldName) // the scrub package wants exact byte matches

	if encoreTag, err := f.Tag.Get("encore"); err == nil {
		sensitive = encoreTag.Name == "sensitive" || slices.Contains(encoreTag.Options, "sensitive")
	}
	return sensitive, fieldName, caseSensitive
}

func isHeader(f schema.StructField) (headerName string, ok bool) {
	if headerTag, err := f.Tag.Get("header"); err == nil {
		name := headerTag.Name
		if name == "" {
			name = f.Name.GetOrElse("")
		}
		return name, true
	}
	return "", false
}

type typeParamPath struct {
	p        scrub.Path
	paramIdx int
}
