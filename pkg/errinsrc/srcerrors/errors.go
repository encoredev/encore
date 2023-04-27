package srcerrors

import (
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"encr.dev/pkg/errinsrc"
	. "encr.dev/pkg/errinsrc/internal"
)

// UnhandledPanic is an error we use to wrap a panic that was not handled
// It should ideally never be seen by users, but if it is, it means we have
// a bug within Encore which needs fixing.
func UnhandledPanic(recovered any) error {
	if err := errinsrc.ExtractFromPanic(recovered); err != nil {
		return err
	}

	// If recovered is an error, then track it as the source
	var srcError error
	if err, ok := recovered.(error); ok {
		srcError = err
	}
	// If we get here, it's an unhandled panic / error
	return errinsrc.New(ErrParams{
		Code:    1,
		Title:   "Internal compiler error",
		Summary: fmt.Sprintf("An unhandled panic occurred in the Encore compiler: %v", recovered),
		Detail:  internalErrReportToEncore,
		Cause:   srcError,
	}, true)
}

// GenericGoParserError reports an error was that was reported from the Go parser.
// It should not be returned by any errors caused by Encore's own parser as they
// should have specific errors listed below
func GenericGoParserError(err *scanner.Error) *errinsrc.ErrInSrc {
	locs := SrcLocations{}
	if pos := FromGoTokenPositions(err.Pos, err.Pos); pos != nil {
		locs = SrcLocations{pos}
	}

	return errinsrc.New(ErrParams{
		Code:      2,
		Title:     "Parse Error in Go Source",
		Summary:   err.Msg,
		Cause:     err,
		Locations: locs,
	}, false)
}

// GenericGoPackageError reports an error was that was reported from the Go package loader.
// It should not be returned by any errors caused by Encore's own parser as they
// should have specific errors listed below
func GenericGoPackageError(err packages.Error) *errinsrc.ErrInSrc {
	var locations SrcLocations

	// Extract the position from the error
	var pos token.Position
	switch p := strings.SplitN(err.Pos, ":", 3); len(p) {
	case 3:
		pos.Column, _ = strconv.Atoi(p[2])
		fallthrough
	case 2:
		pos.Line, _ = strconv.Atoi(p[1])
		fallthrough
	case 1:
		if p[0] != "" && p[0] != "-" {
			pos.Filename = p[0]
		}
	}
	if pos.Filename != "" && pos.Line > 0 {
		locations = SrcLocations{FromGoTokenPositions(pos, pos)}
	}

	return errinsrc.New(ErrParams{
		Code:      3,
		Title:     "Go Package Error",
		Summary:   err.Msg,
		Cause:     err,
		Locations: locations,
	}, false)
}

// GenericGoCompilerError reports an error was that was reported from the Go compiler.
// It should not be returned by any errors caused by Encore's own compiler as they
// should have specific errors listed below.
func GenericGoCompilerError(fileName string, lineNumber int, column int, error string) error {
	errLocation := token.Position{
		Filename: fileName,
		Offset:   0,
		Line:     lineNumber,
		Column:   column,
	}

	return errinsrc.New(ErrParams{
		Code:      3,
		Title:     "Go Compilation Error",
		Summary:   strings.TrimSpace(error),
		Locations: SrcLocations{FromGoTokenPositions(errLocation, errLocation)},
	}, false)
}

// StandardLibraryError is an error that is not caused by Encore, but is
// returned by a standard library function. We wrap it in an ErrInSrc so that
// we can still possibly provide a source location.
func StandardLibraryError(err error) *errinsrc.ErrInSrc {
	return errinsrc.New(ErrParams{
		Code:    3,
		Title:   "Error",
		Summary: err.Error(),
		Cause:   err,
	}, true)
}

// GenericError is a place holder for errors reported through perr.Add or perr.Addf
func GenericError(pos token.Position, msg string) *errinsrc.ErrInSrc {
	return errinsrc.New(ErrParams{
		Code:      3,
		Title:     "Error",
		Summary:   msg,
		Locations: SrcLocations{FromGoTokenPositions(pos, pos)},
	}, false)
}

func UnableToLoadCUEInstances(err error, pathPrefix string) error {
	return handleCUEError(err, pathPrefix, ErrParams{
		Code:  6,
		Title: "Unable to load CUE instances",
	})
}

func UnableToAddOrphanedCUEFiles(err error, pathPrefix string) error {
	return handleCUEError(err, pathPrefix, ErrParams{
		Code:  7,
		Title: "Unable to add orphaned CUE files",
	})
}

func CUEEvaluationFailed(err error, pathPrefix string) error {
	return handleCUEError(err, pathPrefix, ErrParams{
		Code:  8,
		Title: "CUE evaluation failed",
		Detail: "While evaluating the CUE configuration to generate a concrete configuration for your application, CUE returned an error. " +
			"This is usually caused by either a constraint on a field being unsatisfied or there being two different values for a given field. " +
			"For more information on CUE and this error, see https://cuelang.org/docs/",
	})
}

func ResourceNameReserved(fileset *token.FileSet, node ast.Node, resourceType string, paramName string, name, reservedPrefix string, isSnakeCase bool) error {
	suggestion := ""
	if strings.HasPrefix(name, reservedPrefix) { // should always be the case, but better to be safe
		suggestion = fmt.Sprintf("try %q?", name[len(reservedPrefix):])
	}

	var detail string
	if isSnakeCase {
		detail = resourceNameHelpSnakeCase(resourceType, paramName)
	} else {
		detail = resourceNameHelpKebabCase(resourceType, paramName)
	}

	return errinsrc.New(ErrParams{
		Code:  37,
		Title: "Reserved resource name",
		// The metrics.NewCounter metric name "e_blah" uses the reserved prefix "e_".
		Summary:   fmt.Sprintf("The %s %s %q uses the reserved prefix %q", resourceType, paramName, name, reservedPrefix),
		Detail:    detail,
		Locations: SrcLocations{FromGoASTNodeWithTypeAndText(fileset, node, LocError, suggestion)},
	}, false)
}
