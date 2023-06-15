package build

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range("test", "", errors.WithRangeSize(20))

	ErrTestFailed = errRange.New("Test Failure", "One or more more tests failed.")
)
