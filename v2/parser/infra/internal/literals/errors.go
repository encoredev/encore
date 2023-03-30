package literals

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"literals",
		"",

		errors.WithRangeSize(20),
	)

	errArgumentMustBeStruct = errRange.New(
		"Invalid Argument",
		"The argument must be given as a struct literal.",

		errors.MarkAsInternalError(),
	)

	errUnexpectedField = errRange.New(
		"Invalid Argument",
		"Unexpected field in struct literal.",
	)

	errAnonymousFieldsNotSupported = errRange.New(
		"Invalid Argument",
		"Anonymous fields are not supported.",

		errors.MarkAsInternalError(),
	)

	errUnexportedFieldsNotSupported = errRange.New(
		"Invalid Argument",
		"Unexported fields are not supported.",

		errors.MarkAsInternalError(),
	)

	errInvalidTag = errRange.New(
		"Invalid Argument",
		"Invalid tag on field",

		errors.MarkAsInternalError(),
	)

	errMissingRequiredField = errRange.Newf(
		"Invalid Argument",
		"Missing required field `%s`",
	)

	errIsntConstant = errRange.Newf(
		"Invalid Argument",
		"Field `%s` must be a constant literal.",
	)

	errDyanmicFieldNotExpr = errRange.New(
		"Invalid Argument",
		"Dynamic field must be an expression.",
	)

	errWrongDynamicType = errRange.Newf(
		"Invalid Argument",
		"The field `%s` must be a %s literal.",
	)

	errUnsupportedType = errRange.Newf(
		"Invalid Argument",
		"Unsupported type %v.",
	)

	errZeroValue = errRange.Newf(
		"Invalid Argument",
		"The field `%s` must not be the zero value.",
	)

	errNotLiteral = errRange.Newf(
		"Invalid Argument",
		"Expected a literal instance of %s, got %s.",
	)

	errExpectedKeyToBeIdentifier = errRange.Newf(
		"Invalid Argument",
		"Expected key to be an identifier, got %v.",
	)

	errExpectedKeyPair = errRange.Newf(
		"Invalid Argument",
		"Expected key pair, got %v.",
	)

	errPanicParsingExpression = errRange.Newf(
		"Internal Error",
		"Unexpected panic while parsing expression: %v",

		errors.MarkAsInternalError(),
	)

	errUnableToParseLiteral = errRange.New(
		"Internal Error",
		"Unable to parse literal.",

		errors.MarkAsInternalError(),
	)

	errDivideByZero = errRange.New(
		"Invalid Argument",
		"Cannot divide by zero.",
	)

	errInvalidShift = errRange.New(
		"Invalid Argument",
		"Shift count must be an unsigned integer.",
	)

	errUnsupportedOperation = errRange.Newf(
		"Invalid Argument",
		"%s is an unsupported operation here.",
	)

	errDefaultTagNotSupported = errRange.New(
		"Invalid Argument",
		"Default tags are not supported.",
		errors.MarkAsInternalError(),
	)
)
