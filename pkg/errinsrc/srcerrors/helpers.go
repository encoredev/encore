package srcerrors

import (
	cueerrors "cuelang.org/go/cue/errors"

	"encr.dev/pkg/errinsrc"
	. "encr.dev/pkg/errinsrc/internal"
)

func handleCUEError(err error, pathPrefix string, param ErrParams) error {
	if err == nil {
		return nil
	}

	toReturn := make(errinsrc.List, 0, 1)

	if param.Detail == "" {
		param.Detail = "For more information on CUE and this error, see https://cuelang.org/docs/"
	}

	for _, e := range cueerrors.Errors(err) {
		param.Summary = e.Error()
		param.Cause = e
		param.Locations = LocationsFromCueError(e, pathPrefix)
		toReturn = append(toReturn, errinsrc.New(param, false))
	}

	switch len(toReturn) {
	case 0:
		return nil
	case 1:
		return toReturn[0]
	default:
		return toReturn
	}
}
