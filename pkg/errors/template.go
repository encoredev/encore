package errors

import (
	goAst "go/ast"
	goToken "go/token"
	"strings"
)

// Template represents a template for a new error.
//
// It itself is not an error, but can be used to initialize a new [errorinsrc.ErrInSrc].
type Template struct {
	Code               int
	Title              string
	Summary            string
	Detail             string
	Cause              error
	Locations          []SrcLocation
	AlwaysIncludeStack bool
}

// TemplateOption can be passed into the [Range] when createing a new Template
type TemplateOption func(*Template)

// AlwaysIncludeStack will setup a Template so it always includes a stack trace
// even in a production build of Encore.
func AlwaysIncludeStack() TemplateOption {
	return func(template *Template) {
		template.AlwaysIncludeStack = true
	}
}

// WithDetails will setup a template so it uses a different details to the
// range default
func WithDetails(details string) TemplateOption {
	return func(template *Template) {
		template.Detail = details
	}
}

// PrependDetails will setup a template so it prepends the given details to the
// range default
func PrependDetails(details string) TemplateOption {
	return func(template *Template) {
		if template.Detail != "" {
			details = details + "\n\n" + template.Detail
		}
		template.Detail = details
	}
}

// MarkAsInternalError will setup a template so it is reported as an internal error.
//
// This means that the error will be reported to the user as an internal error
// with a link to the Encore issue tracker and will include a stack trace.
func MarkAsInternalError() TemplateOption {
	return func(template *Template) {
		template.AlwaysIncludeStack = true
		template.Title = "Internal Error: " + template.Title
		template.Detail = "This is a bug in Encore and should not have occurred. Please report this issue to the " +
			"Encore team either on Github at https://github.com/encoredev/encore/issues/new and include this error."
	}
}

// WithDetails will replace the details of the template with the given details
func (t Template) WithDetails(details string) Template {
	t.Detail = details
	return t
}

// Wrapping wraps the given error with the template.
//
// It will append the given error to the summary of the template
// as well as setting the cause of the template to the given error.
func (t Template) Wrapping(err error) Template {
	if err == nil {
		return t
	}
	t.Summary += "\n\n" + err.Error()
	t.Summary = strings.TrimSpace(t.Summary)

	t.Cause = err
	return t
}

// atLocation is a helper method for the various with methods
func (t Template) atLocation(location SrcLocation, options []LocationOption) Template {
	for _, o := range options {
		o(&location)
	}
	t.Locations = append([]SrcLocation{location}, t.Locations...)
	return t
}

// InFile adds the given file as a src of the error location
//
// Note: It is preferable to use one of the other location functions
// as they will render the source around the error, not just the file name
func (t Template) InFile(filepath string, options ...LocationOption) Template {
	if filepath == "" {
		return t
	}

	return t.atLocation(SrcLocation{Kind: LocFile, Filepath: filepath}, options)
}

// AtGoNode adds the given Go node to the template. If the node is nil, nothing happens.
//
// You can use the [LocationOption]s to add additional information to the location.
//
// Example:
//
//	errMyErrorTemplate.AtGoNode(node, errtmp.AsHelp("this is where it was defined before"))
func (t Template) AtGoNode(node goAst.Node, options ...LocationOption) Template {
	if node == nil {
		return t
	}

	return t.atLocation(SrcLocation{Kind: LocGoNode, GoNode: node}, options)
}

// AtGoPos adds the given start and end [token.Pos] to the template. If both positions are token.NoPos, nothing happens.
// If one of the two positions are token.NoPos, the other position will be used.
//
// It is valid to use the same value for start and end positions, in which case the error will estimate which Node
// you are referencing.
//
// Example:
//
//	errMyErrorTemplate.AtGoPos(start, token.NoPos, errtmp.AsHelp("this is where it was defined before"))
func (t Template) AtGoPos(start, end goToken.Pos, options ...LocationOption) Template {
	switch {
	case start == goToken.NoPos && end == goToken.NoPos:
		return t
	case start == goToken.NoPos && end != goToken.NoPos:
		start = end
	case end == goToken.NoPos && start != goToken.NoPos:
		end = start
	}

	return t.atLocation(SrcLocation{Kind: LocGoPos, GoStartPos: start, GoEndPos: end}, options)
}

// AtGoPosition adds the given Go positions to the template.
//
// It is valid to use the same value for start and end positions, in which case the error will estimate which Node
// you are referencing.
//
// Example:
//
//	errMyErrorTemplate.AtGoPosition(start, end, errtmp.AsHelp("this is where it was defined before"))
func (t Template) AtGoPosition(start, end goToken.Position, options ...LocationOption) Template {
	return t.atLocation(SrcLocation{Kind: LocGoPositions, GoStartPosition: start, GoEndPosition: end}, options)
}
