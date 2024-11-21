package gcsemu

import (
	"fmt"

	"google.golang.org/api/googleapi"
)

func httpStatusCodeOf(err error) int {
	if gapiErr, ok := err.(*googleapi.Error); ok {
		return gapiErr.Code
	}

	if httpErr, ok := err.(*httpError); ok {
		if httpErr.code != 0 {
			return httpErr.code
		}
		return httpStatusCodeOf(httpErr.cause)
	}
	return 0
}

func fmtErrorfCode(httpCode int, f string, args ...interface{}) error {
	return &httpError{
		cause: fmt.Errorf(f, args...),
		code:  httpCode,
	}
}

// httpError is a custom error type that decorates with an HTTP error code
type httpError struct {
	cause error
	code  int
}

// Error returns a string describing the entire causal chain.
func (err *httpError) Error() string {
	if err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("http error %d: %s", err.code, err.cause)
}
