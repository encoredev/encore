package api

import (
	"strconv"

	"encore.dev/beta/errs"
)

func clampTo64Chars(str string) string {
	if len(str) > 64 {
		return str[:64]
	}
	return str
}

func code(err error, httpStatus int) string {
	if err != nil {
		e := errs.Convert(err).(*errs.Error)
		return e.Code.String()
	}

	if httpStatus == 0 {
		return errs.OK.String()
	}

	if code := errs.HTTPStatusToCode(httpStatus); code != errs.Unknown {
		return code.String()
	}
	return "http_" + strconv.Itoa(httpStatus)
}
