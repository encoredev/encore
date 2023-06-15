package build

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range("test", "", errors.WithRangeSize(20))

	ErrTestFailued = errRange.New("Test Failure", "One more more tests failed.")
)
