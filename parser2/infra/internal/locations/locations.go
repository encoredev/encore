package locations

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// Location represents a bitwise flag of locations
type Location int

//go:generate stringer -type=Location -output locations_string.go

// This list is ordered from most outer possible location, to most inner location
const (
	File         Location = 1 << iota // Inside a file, outside a function body
	InitFunction                      // Inside an init function declaration
	Function                          // Inside any function declaration (including the init function)
	FuncCall                          // Inside any function call
	Variable                          // Inside a variable
	endOfList
)

// Describe returns a list of the location parts which make up this location
func (i Location) Describe() string {
	var parts []string

	for j := 0; (1 << j) < endOfList; j++ {
		loc := Location(i & (1 << j))
		if loc != 0 {
			parts = append(parts, loc.String())
		}
	}

	return strings.Join(parts, ", ")
}

// Filters represents a list of location filters
type Filters []Filter

// Allowed returns true if _any_ of the filters allow the location, regardless if others do not
func (f Filters) Allowed(location Location) bool {
	for _, filter := range f {
		if filter.Allowed(location) {
			return true
		}
	}

	return false
}

// Describe returns an english description of where `what` can be `pastTenseVerb`
//
// For example:
//
//	Filter{In(Function), In(InitFunction), In(Variable).NotIn(Function)}.Describe("foo", "called")
//
// will return:
//
//	foo can be only be called in a function, an init function or a package level variable declaration
func (f Filters) Describe(what string, pastTenseVerb string) string {
	var str strings.Builder

	switch len(f) {
	case 0:
		str.WriteString("there are no valid locations where ")
		str.WriteString(what)
		str.WriteString(" can be " + pastTenseVerb)
	default:
		str.WriteString(what)
		str.WriteString(" can only be " + pastTenseVerb + " in ")

		for i, filter := range f {
			if i > 0 {
				if i == len(f)-1 {
					str.WriteString(" or ")
				} else {
					str.WriteString(", ")
				}
			}

			str.WriteString(filter.Describe())
		}
	}

	return str.String()
}

// Filter represents a location filter
type Filter struct {
	Allow, Disallowed Location
}

// AllowedIn creates a location filter, where the given location must fully match all locations given.
//
// For example:
//
//	f := In(File, InitFunction, Variable)
//	f.Allowed(Variable) == false
//	f.Allowed(File & Variable & InitFunction & Function) == true
func AllowedIn(locations ...Location) Filter {
	f := Filter{}
	for _, loc := range locations {
		f.Allow |= loc
	}
	return f
}

// ButNotIn modifies the location filter, where a given location must not be in _any_ of the locations given.
//
// For example:
//
//	f := In(Function).NotIn(InitFunction, Variable)
//	f.Allowed(File & Function) == true
//	f.Allowed(File & Function & Variable) == false
func (f Filter) ButNotIn(locations ...Location) Filter {
	newF := f
	for _, loc := range locations {
		newF.Disallowed |= loc
	}
	return newF
}

// Allowed returns true if this location allowed by this filter.
// See both AllowedIn and ButNotIn for examples
func (f Filter) Allowed(location Location) bool {
	return (f.Allow&location) == f.Allow &&
		(f.Disallowed&location) == 0
}

func (f Filter) Describe() string {
	switch {
	case f.Allow == Variable && f.Disallowed&Function == Function:
		return "a package level variable"
	case f.Allow == Variable && f.Disallowed == 0:
		return "a variable"
	case f.Allow == InitFunction && f.Disallowed == 0:
		return "an init function"
	case f.Allow == Function && f.Disallowed == 0:
		return "a function"
	case f.Allow == Function && f.Disallowed == InitFunction:
		return "any function which is not an init function"
	default:
		return fmt.Sprintf("%s but not in %s", f.Allow.Describe(), f.Disallowed.Describe())
	}
}

// Classify classifies the current location based on the ancestor stack.
func Classify(stack []ast.Node) (loc Location) {
	for idx, n := range stack {
		switch n := n.(type) {
		case *ast.File:
			loc |= File

		case *ast.GenDecl:
			if n.Tok == token.VAR {
				loc |= Variable
			}

		case *ast.CallExpr:
			loc |= FuncCall

		case *ast.FuncDecl:
			loc |= Function

			if n.Name.Name == "init" {
				// This is function named init. To treat this as an init
				// function, make sure it's not nested within another function.
				foundFunc := false
				for i := idx - 1; i >= 0; i-- {
					if _, ok := stack[i].(*ast.FuncDecl); ok {
						// We found a function, so this is not an init function
						foundFunc = true
						break
					}
				}
				if !foundFunc {
					// We couldn't find any parent function, so this is a proper init function.
					loc |= InitFunction
				}
			}
		}
	}

	return
}
