package srcerrors

import (
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"strings"

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
		Title:   "Unhandled Panic",
		Summary: fmt.Sprintf("A unhandled panic occurred: %v", recovered),
		Detail:  internalErrReportToEncore,
		Cause:   srcError,
	}, true)
}

// GenericGoParserError reports an error was that was reported from the Go parser.
// It should not be returned by any errors caused by Encore's own parser as they
// should have specific errors listed below
func GenericGoParserError(err *scanner.Error) *errinsrc.ErrInSrc {
	return errinsrc.New(ErrParams{
		Code:      2,
		Title:     "Parse Error in Go Source",
		Summary:   err.Msg,
		Cause:     err,
		Locations: SrcLocations{FromGoTokenPositions(err.Pos, err.Pos)},
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

func DatabaseNotFound(fileset *token.FileSet, node ast.Node, dbName string) error {
	return errinsrc.New(ErrParams{
		Code:      4,
		Title:     "Database Not Found",
		Summary:   fmt.Sprintf("The database %s was not found", dbName),
		Locations: SrcLocations{FromGoASTNode(fileset, node)},
	}, false)
}

func UnknownErrorCompilingConfig(fileset *token.FileSet, node ast.Node, err error) error {
	return errinsrc.New(ErrParams{
		Code:      5,
		Title:     "Error compiling configuration",
		Summary:   err.Error(),
		Cause:     err,
		Locations: SrcLocations{FromGoASTNode(fileset, node)},
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

func ConfigOnlyLoadedFromService(fileset *token.FileSet, node ast.Node) error {
	return errinsrc.New(ErrParams{
		Code:      9,
		Title:     "Invalid call to config.Load[T]()",
		Summary:   "A call to config.Load[T]() can only be made from within a service.",
		Detail:    combine(makeService, configHelp),
		Locations: SrcLocations{FromGoASTNode(fileset, node)},
	}, false)
}

func ConfigMustBeTopLevelPackage(fileset *token.FileSet, node ast.Node) error {
	return errinsrc.New(ErrParams{
		Code:      10,
		Title:     "Invalid call to config.Load[T]()",
		Summary:   "A call to config.Load[T]() can only be made from the top level package of a service.",
		Detail:    configHelp,
		Locations: SrcLocations{FromGoASTNode(fileset, node)},
	}, false)
}

func ConfigLoadNoArguments(fileset *token.FileSet, node *ast.CallExpr) error {
	start := fileset.Position(node.Lparen + 1)
	end := fileset.Position(node.Rparen)

	return errinsrc.New(ErrParams{
		Code:      11,
		Title:     "Invalid call to config.Load[T]()",
		Summary:   "A call to config.Load[T]() does not accept any arguments.",
		Detail:    configHelp,
		Locations: SrcLocations{FromGoTokenPositions(start, end)},
	}, false)
}

func ConfigOnlyReferencedSameService(fileset *token.FileSet, reference ast.Node, defined ast.Node) error {
	refLoc := FromGoASTNode(fileset, reference)
	refLoc.Text = "referenced here"

	definedLoc := FromGoASTNode(fileset, defined)
	definedLoc.Type = LocHelp
	definedLoc.Text = "defined here"

	return errinsrc.New(ErrParams{
		Code:      12,
		Title:     "Cross service resource reference",
		Summary:   "A config instance can only be referenced from within the service that the call to `config.Load[T]()` was made in.",
		Detail:    configHelp,
		Locations: SrcLocations{refLoc, definedLoc},
	}, false)
}

func UnknownConfigWrapperType(fileset *token.FileSet, node ast.Node, ident *ast.Ident) error {
	return errinsrc.New(ErrParams{
		Code:      13,
		Title:     "Unknown config type",
		Summary:   fmt.Sprintf("config.%s is not type which can be used within data structures", ident.Name),
		Detail:    configHelp,
		Locations: SrcLocations{FromGoASTNode(fileset, node)},
	}, false)
}

func ConfigValueTypeNotSet(fileset *token.FileSet, node ast.Node) error {
	return errinsrc.New(ErrParams{
		Code:      14,
		Title:     "Internal Error",
		Summary:   "The type of a config value was not set.",
		Detail:    internalErrReportToEncore,
		Locations: SrcLocations{FromGoASTNode(fileset, node)},
	}, true)
}

func ConfigWrapperNested(fileset *token.FileSet, node ast.Node, funcCall *ast.CallExpr) error {
	loc := FromGoASTNode(fileset, funcCall)
	loc.Text = "loaded from here"

	locs := SrcLocations{loc}

	if node != nil {
		loc.Type = LocHelp

		field := FromGoASTNode(fileset, node)
		locs = SrcLocations{field, loc}
	}

	return errinsrc.New(ErrParams{
		Code:      15,
		Title:     "Invalid config type",
		Summary:   "The type of config.Value[T] cannot be another config.Value[T]",
		Detail:    configHelp,
		Locations: locs,
	}, false)
}

func ConfigTypeHasUnexportFields(fileset *token.FileSet, loadCall ast.Node, field *ast.Field) error {
	loadLoc := FromGoASTNode(fileset, loadCall)
	loadLoc.Text = "loaded from here"
	loadLoc.Type = LocHelp

	return errinsrc.New(ErrParams{
		Code:      16,
		Title:     "Invalid config type",
		Summary:   fmt.Sprintf("Field %s is not exported and is in a datatype which is used by a call to `config.Load[T]()`. Unexported fields cannot be initialised by Encore, thus are not allowed in this context.", field.Names[0].Name),
		Detail:    configHelp,
		Locations: SrcLocations{FromGoASTNode(fileset, field), loadLoc},
	}, false)
}
