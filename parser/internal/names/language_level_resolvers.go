package names

import (
	"fmt"
	"go/ast"
	"sort"

	"github.com/hashicorp/go-version"
)

// languageLevelResolver allows us to add behaviour at a language level (i.e. go1.16, go1.17, ..., go2.0) and have them
// contained within a single code base, but in a way which allows us to use `go:build` tags to no even attempt to compile
// against newer versions of the language that encore itself isn't compiled against
type languageLevelResolver interface {
	LanguageVersion() string // The language minor version this is for (i.e. `1.17` not `1.17.6`)

	expr(r *resolver, expr ast.Expr) (ok bool)
}

// languageLevelResolvers is a sorted list of language level resolvers, with the first index being the _highest_ language
// version we've compiled against. This allows newer language versions to override behaviour in older language versions
// when needed.
//
// use registerLanguageLevelResolver to add to this slice.
var languageLevelResolvers = make([]languageLevelResolver, 0)

// registerLanguageLevelResolver adds the given resolver to our slice of registered resolvers and sorts them
// such that the first resolver in the slice is the highest language version we've got support for.
func registerLanguageLevelResolver(resolver languageLevelResolver) {
	languageLevelResolvers = append(languageLevelResolvers, resolver)

	sort.Slice(languageLevelResolvers, func(i, j int) bool {
		iVersion, err := version.NewVersion(languageLevelResolvers[i].LanguageVersion())
		if err != nil {
			panic(fmt.Sprintf("Unable to parse version: %+v", err))
		}

		jVersion, err := version.NewVersion(languageLevelResolvers[j].LanguageVersion())
		if err != nil {
			panic(fmt.Sprintf("Unable to parse version: %+v", err))
		}

		return iVersion.GreaterThan(jVersion)
	})
}
