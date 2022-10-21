package errinsrc

import (
	"go/ast"
	"go/token"

	. "encr.dev/pkg/errinsrc/internal"
)

type ErrorList interface {
	Error() string
	ErrorList() []*ErrInSrc
}

// Bailout is a panic value that can be used to
type Bailout struct {
	List ErrorList
}

func Panic(list ErrorList) {
	panic(Bailout{list})
}

// ExtractFromPanic returns the first ErrInSrc or ErrorList found in the recovered
// value.
//
// If no value is recovered, then nil is returned.
func ExtractFromPanic(recovered any) error {
	// If it's already an ErrInSrc or list of them, just return that
	if unwrapped, ok := recovered.(error); ok {
		// Check the type of the error then unwrap as needed
		for unwrapped != nil {
			switch err := recovered.(type) {
			case *ErrInSrc:
				return err
			case ErrorList:
				return err
			case Bailout:
				return err.List
			case interface{ Unwrap() error }:
				unwrapped = err.Unwrap()
			default:
				// If we get here, it's not an errinsrc or error list, so return nil
				return nil
			}
		}
	}

	return nil
}

func AddHintFromGo(err error, fileset *token.FileSet, node ast.Node, hint string) {
	switch err := err.(type) {
	case *ErrInSrc:
		hintLoc := FromGoASTNode(fileset, node)
		hintLoc.Type = LocHelp
		hintLoc.Text = hint
		err.Params.Locations = append(err.Params.Locations, hintLoc)
	case ErrorList:
		for _, err := range err.ErrorList() {
			AddHintFromGo(err, fileset, node, hint)
		}
	}
}
