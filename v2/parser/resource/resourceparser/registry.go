package resourceparser

import (
	"os"

	"golang.org/x/exp/slices"

	"encr.dev/internal/paths"
	"encr.dev/pkg/fns"
	"encr.dev/v2/internals/pkginfo"
)

func NewRegistry(parsers []*Parser) *Registry {
	forImports, always := parsersForImports(parsers)
	subdirs := fns.Filter(parsers, func(p *Parser) bool { return len(p.InterestingSubdirs) > 0 })
	return &Registry{
		parsers:              parsers,
		alwaysInterested:     always,
		interestedForImports: forImports,
		subdirsInterested:    subdirs,
	}
}

type Registry struct {
	parsers []*Parser

	// alwaysInterested are the parses that should be run against all packages.
	alwaysInterested []*Parser

	// interestedForImports maps from import paths to the list of parsers
	// interested in packages that import that package.
	interestedForImports map[paths.Pkg][]*Parser

	// subdirsInterested are the parses that are interested in
	// specific subdirs.
	subdirsInterested []*Parser
}

// InterestedParsers returns the parsers interested in processing a given package.
func (r *Registry) InterestedParsers(pkg *pkginfo.Package) []*Parser {
	parsers := slices.Clone(r.alwaysInterested)

	addParser := func(p *Parser) {
		if !slices.Contains(parsers, p) {
			parsers = append(parsers, p)
		}
	}

	// Find the interested parsers based on imports.
	for imp := range pkg.Imports {
		// Add the parsers that are interested in this package as long
		// as they're not already in the list.
		// Note: this is O(n^2) but n is small, so this should be faster than maintaining a set.
		for _, p := range r.interestedForImports[imp] {
			addParser(p)
		}
	}

	// Find the interested parsers based on subdirs.
ParserLoop:
	for _, p := range r.subdirsInterested {
		for _, dir := range p.InterestingSubdirs {
			if stat, err := os.Stat(pkg.FSPath.Join(dir).ToIO()); err == nil && stat.IsDir() {
				addParser(p)
				continue ParserLoop
			}
		}
	}

	return parsers
}

// parsersForImports returns a map from package paths to the list of parsers
// interested in that package.
func parsersForImports(parsers []*Parser) (forImports map[paths.Pkg][]*Parser, always []*Parser) {
	forImports = make(map[paths.Pkg][]*Parser)

ParserLoop:
	for _, parser := range parsers {
		for _, imp := range parser.InterestingImports {
			if imp == "*" {
				always = append(always, parser)
				continue ParserLoop
			}
			forImports[imp] = append(forImports[imp], parser)
		}
	}
	return forImports, always
}
