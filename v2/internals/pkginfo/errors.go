package pkginfo

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"pkginfo",
		"",

		errors.WithRangeSize(25),
	)

	errReadingGoMod = errRange.New(
		"Error Reading go.mod",
		"An error occurred while trying to read the go.mod file.",
	)

	errInvalidModulePath = errRange.Newf(
		"Invalid Module Path",
		"The module path %q in the go.mod file is invalid.",
	)

	errReadingFile = errRange.New(
		"Error Reading File",
		"An error occurred while trying to read a file.",
	)

	errMatchingFile = errRange.New(
		"Error Matching File",
		"An error occurred while trying to match a file to the build.",
	)
)
