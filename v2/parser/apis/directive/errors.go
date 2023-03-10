package directive

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"directive",
		"",

		errors.WithRangeSize(20),
	)

	errMultipleDirectives = errRange.New(
		"Multiple Directives",
		"Multiple directives are not allowed on the same declaration.",
	)

	errMissingDirectiveName = errRange.New(
		"Invalid Encore Directive",
		"Directives must have a name. For example, `//encore:api` is valid, but `//encore:` is not.",
	)

	errDuplicateTag = errRange.Newf(
		"Duplicate Tag",
		"The tag %q is already defined on this declaration. Tags must be unique per directive.",
	)

	errInvalidTag = errRange.Newf(
		"Invalid Tag",
		"Invalid tag %q. Tags must start with a letter and contain only letters, numbers, hyphens and underscores.",
	)

	errFieldHasNoValue = errRange.New(
		"Invalid Directive Field",
		"Directive fields must have a value. For example, `//encore:api foo=bar` is valid, but `//encore:api foo=` is not.",
	)

	errInvalidFieldName = errRange.Newf(
		"Invalid Directive Field",
		"Invalid field name %q. Field names must start only contain letters.",
	)

	errDuplicateField = errRange.Newf(
		"Duplicate Directive Field",
		"The field %q is already defined on this directive. Fields must be unique per directive.",
	)

	errUnknownField = errRange.Newf(
		"Invalid Directive Field",
		"Unknown field %q. Fields must be one of %s.",
	)

	errInvalidOptionName = errRange.Newf(
		"Invalid Directive Option",
		"Invalid option name %q. Options must start only contain letters.",
	)

	errDuplicateOption = errRange.Newf(
		"Duplicate Directive Option",
		"The option %q is already defined on this directive. Options must be unique per directive.",
	)

	errUnknownOption = errRange.Newf(
		"Invalid Directive Option",
		"Unknown option %q. Options must be one of %s.",
	)
)
