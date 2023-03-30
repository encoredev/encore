package resourcepaths

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"paths",
		"Paths must be not be empty and always start with a '/'. You cannot define paths that conflict "+
			"with each other, including static and parameterized paths. For example `/blog/:id` would conflict with `/:username`.\n\n"+
			"For more information about configuring Paths, see https://encore.dev/docs/primitives/services-and-apis#rest-apis",

		errors.WithRangeSize(20),
	)

	errEmptyPath = errRange.New(
		"Invalid Path",
		"Paths must not be empty.",
	)

	errInvalidPathPrefix = errRange.New(
		"Invalid Path",
		"Paths cannot start with a '/'.",
	)

	errInvalidPathMissingPrefix = errRange.New(
		"Invalid Path",
		"Paths must always start with a '/'.",
	)

	errInvalidPathURI = errRange.New(
		"Invalid Path",
		"Paths must be valid URIs. There was an error parsing the path.",
	)

	errPathContainedQuery = errRange.New(
		"Invalid Path",
		"Paths must not contain the '?' character.",
	)

	errEmptySegment = errRange.New(
		"Invalid Path",
		"Paths cannot contain an empty segment, i.e. a double slash ('//').",
	)

	errTrailingSlash = errRange.New(
		"Invalid Path",
		"Paths cannot end with a trailing slash ('/').",
	)

	errParameterMissingName = errRange.New(
		"Invalid Path",
		"Path parameters must have a name. For example, `/:id` is valid, but `/:` is not.",
	)

	errInvalidParamIdentifier = errRange.New(
		"Invalid Path",
		"Path parameters must be valid Go identifiers.",
	)

	errWildcardNotLastSegment = errRange.New(
		"Invalid Path",
		"Path wildcards must be the last segment in the path.",
	)

	errDuplicatePath = errRange.New(
		"Path Conflict",
		"Duplicate Paths found.",
	)

	errConflictingParameterizedPath = errRange.Newf(
		"Path Conflict",
		"The parameter `:%s` conflicts with the path `%s`.",
	)

	errConflictingWildcardPath = errRange.Newf(
		"Path Conflict",
		"The wildcard `*%s` conflicts with the path `%s`.",
	)

	errConflictingLiteralPath = errRange.Newf(
		"Path Conflict",
		"The path segment `%s` conflicts with the path `%s`.",
	)
)
