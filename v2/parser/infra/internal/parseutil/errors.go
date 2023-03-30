package parseutil

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"parseutil",
		"",
	)

	errRequiresTypeArgumentsNoneFound = errRange.Newf(
		"Missing type arguments",
		"%s requires type arguments, but none were found.",
	)

	errWrongNumberOfTypeArguments = errRange.Newf(
		"Wrong number of type arguments",
		"%s requires%s %d type arguments, but %d were found.",
	)

	errCannotBeReferencedWithoutBeingCalled = errRange.Newf(
		"Invalid reference",
		"%s cannot be referenced without being called.",
	)

	errCannotBeCalledHere = errRange.Newf(
		"Invalid call",
		"%s cannot be called here. It must be called from %s.",
	)

	errResourceNameMustBeStringLiteral = errRange.Newf(
		"Invalid resource name",
		"A %s requires the %s given as a string literal.",
	)

	errResourceNameInvalidLength = errRange.Newf(
		"Invalid resource name",
		"The %s %s needs to be between 1 and 63 characters long.",
	)

	errResourceNameNotCorrectFormat = errRange.Newf(
		"Invalid resource name",
		"The %s %s must be defined in \"%s\".",
	)

	errResourceNameReserved = errRange.Newf(
		"Invalid resource name",
		"The %s %s %q used the reserved prefix %q.",
	)
)
