package run

import (
	"errors"

	"encr.dev/pkg/errlist"
	"encr.dev/v2/internals/perr"
)

func AsErrorList(err error) *errlist.List {
	if errList := errlist.Convert(err); errList != nil {
		return errList
	}

	list := &perr.ListAsErr{}
	if errors.As(err, &list) {
		return &errlist.List{List: list.ErrorList()}
	}
	return nil
}
