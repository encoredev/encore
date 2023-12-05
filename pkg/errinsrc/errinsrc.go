package errinsrc

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/pkg/errors"

	encerrors "encr.dev/pkg/errors"
	"encr.dev/pkg/option"

	. "encr.dev/pkg/errinsrc/internal"
)

// ErrInSrc represents an error which occurred due to the source code
// of the application being run through Encore.
//
// It supports the concept of one of more locations within the users
// source code which caused the error. The locations will be rendered
// in the final output.
//
// Construct these using helper functions in the `srcerrors` package
// as we can use that as a central repository error types
type ErrInSrc struct {
	// The parameters of the error
	// This is an internal data type to force the
	// creation of these inside `srcerrors`
	Params ErrParams `json:"params"`

	// The Stack trace of where the error was created within the Encore codebase
	// this will be empty if the error was created in a production build of Encore.
	// To populate this, build Encore with the tag `dev_build`.
	Stack []*StackFrame `json:"stack,omitempty"`
}

var _ error = (*ErrInSrc)(nil)

// New returns a new ErrInSrc with a Stack trace attached
func New(params ErrParams, alwaysIncludeStack bool) *ErrInSrc {
	var stack []*StackFrame

	//goland:noinspection GoBoolExpressions
	if IncludeStackByDefault || alwaysIncludeStack {
		if params.Cause != nil {
			stack = bottomStackTraceFrom(params.Cause)
		}

		if len(stack) == 0 {
			stack = GetStack()
		}
	}

	return &ErrInSrc{
		Params: params,
		Stack:  stack,
	}
}

// FromTemplate returns a new ErrInSrc using the [errs.Template] as a template
func FromTemplate(template encerrors.Template, fileset *token.FileSet) *ErrInSrc {
	// Setup the parameters
	params := ErrParams{
		Code:    template.Code,
		Title:   template.Title,
		Summary: template.Summary,
		Detail:  template.Detail,
		Cause:   template.Cause,
	}

	// Read the locations
	for _, tmplLoc := range template.Locations {
		var location option.Option[*SrcLocation]
		switch tmplLoc.Kind {
		case encerrors.LocFile:
			params.Summary += "\n\nIn file: " + tmplLoc.Filepath
			continue
		case encerrors.LocGoNode:
			location = FromGoASTNode(fileset, tmplLoc.GoNode)
		case encerrors.LocGoPos:
			location = FromGoTokenPos(fileset, tmplLoc.GoStartPos, tmplLoc.GoEndPos)
		case encerrors.LocGoPositions:
			location = FromGoTokenPositions(tmplLoc.GoStartPosition, tmplLoc.GoEndPosition)
		default:
			panic(fmt.Sprintf("unknown location kind: %v", tmplLoc.Kind))
		}

		loc, ok := location.Get()
		if !ok {
			continue
		}

		switch tmplLoc.LocType {
		case encerrors.LocError:
			loc.Type = LocError
		case encerrors.LocWarning:
			loc.Type = LocWarning
		case encerrors.LocHelp:
			loc.Type = LocHelp
		default:
			panic(fmt.Sprintf("unknown location type: %v", tmplLoc.LocType))
		}

		loc.Text = tmplLoc.Text

		params.Locations = append(params.Locations, loc)
	}

	// Create the error
	return New(params, template.AlwaysIncludeStack)
}

// TerminalWidth is the width of the terminal in columns that we're rendering to.
//
// We default to 100 characters, but the CLI overrides this when it renders errors.
// When using the value, note it might be very small (e.g. 5) if the user has shrunk
// their terminal window super small. Thus any code which uses this or creates new
// widths off it should cope with <= 0 values.
var TerminalWidth = 100

func (e *ErrInSrc) Unwrap() error {
	return e.Params.Cause
}

// StackTrace implements the StackTraceProvider interface for some libraries
// including ZeroLog, xerrors and Sentry
func (e *ErrInSrc) StackTrace() errors.StackTrace {
	frames := make([]errors.Frame, len(e.Stack))
	for i, frame := range e.Stack {
		// Note: interpreted as a uintptr its value represents the program counter + 1.
		frames[i] = errors.Frame(frame.ProgramCounter + 1)
	}
	return frames
}

func (e *ErrInSrc) Is(target error) bool {
	if target == nil || e == nil {
		return target == e
	}

	if target, ok := target.(*ErrInSrc); ok && target != nil {
		return target.Params.Title == e.Params.Title
	}
	return false
}

func (e *ErrInSrc) As(target any) bool {
	if target, ok := target.(*ErrInSrc); ok {
		*target = *e
		return true
	}
	return false
}

// Bailout is a helper function which will abort the current process
// and report the error
func (e *ErrInSrc) Bailout() {
	panic(Bailout{List: List{e}})
}

func (e *ErrInSrc) Title() string {
	return e.Params.Title
}

func (e *ErrInSrc) Error() string {
	var b strings.Builder

	// Write the header
	const headerGrayLevel = 12
	const spacing = 4 + 2 + 7 // (4 = "--" on both sides, 2 = " " on the sides of the title, 7 = "[E0000]")
	b.WriteRune('\n')         // Always start with a new line as these errors are expected to be full screen
	b.WriteString(aurora.Gray(headerGrayLevel, fmt.Sprintf("%c%c ", set.HorizontalBar, set.HorizontalBar)).String())
	b.WriteString(aurora.Red(e.Params.Title).String())
	b.WriteByte(' ')
	headerWidth := TerminalWidth - len(e.Params.Title) - spacing
	if headerWidth > 0 {
		b.WriteString(aurora.Gray(headerGrayLevel, strings.Repeat(string(set.HorizontalBar), headerWidth)).String())
	}
	b.WriteString(aurora.Gray(headerGrayLevel, fmt.Sprintf("%cE%04d%c", set.LeftBracket, e.Params.Code, set.RightBracket)).String())
	b.WriteString(aurora.Gray(headerGrayLevel, fmt.Sprintf("%c%c\n\n", set.HorizontalBar, set.HorizontalBar)).String())

	// Write the summary
	if e.Params.Summary != "" {
		wordWrap(e.Params.Summary, &b)
		b.WriteString("\n")
	}

	// List the root causes
	if len(e.Params.Locations) > 0 {
		for _, causes := range e.Params.Locations.GroupByFile() {
			renderSrc(&b, causes)
			b.WriteString("\n")
		}
	}

	// Write any details out
	if e.Params.Detail != "" {
		wordWrap(e.Params.Detail, &b)
		b.WriteString("\n")
	}

	// Write the Stack trace out (for where the error was generated within Encore's source)
	if len(e.Stack) > 0 {
		prettyPrintStack(e.Stack, &b)
	}

	return b.String()
}

func (e *ErrInSrc) OnSameLine(other *ErrInSrc) bool {
	for _, loc := range e.Params.Locations {
		for _, otherLoc := range other.Params.Locations {
			if loc.Start.Line >= otherLoc.Start.Line && loc.End.Line <= otherLoc.End.Line {
				return true
			}
		}
	}
	return false
}

// WithGoNode adds a Go AST node to the error
func (e *ErrInSrc) WithGoNode(fileset *token.FileSet, node ast.Node) {
	if val, ok := FromGoASTNode(fileset, node).Get(); ok {
		e.Params.Locations = append(e.Params.Locations, val)
	}
}
