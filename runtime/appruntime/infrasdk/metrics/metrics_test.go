package metrics

import (
	"errors"
	"net/http"
	"testing"

	"encore.dev/appruntime/apisdk/api"
	"encore.dev/beta/errs"
)

func TestCode(t *testing.T) {
	testCases := []struct {
		err        error
		httpStatus int
		want       string
	}{
		{
			err:        nil,
			httpStatus: 0,
			want:       "ok",
		},
		{
			err:  &errs.Error{Code: errs.OK},
			want: errs.OK.String(),
		},
		{
			err:  &errs.Error{Code: errs.Internal},
			want: errs.Internal.String(),
		},
		{
			err:  errors.New("unknown error"),
			want: errs.Unknown.String(),
		},
		{
			httpStatus: http.StatusOK,
			want:       "ok",
		},
		{
			httpStatus: http.StatusCreated,
			want:       "http_201",
		},
		{
			httpStatus: http.StatusTeapot,
			want:       "http_418",
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.want, func(t *testing.T) {
			t.Parallel()

			code := api.Code(testCase.err, testCase.httpStatus)
			if code != testCase.want {
				t.Errorf("got '%s', want '%s'", code, testCase.want)
			}
		})
	}
}
