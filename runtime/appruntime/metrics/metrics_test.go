package metrics

import (
	"errors"
	"net/http"
	"testing"

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
			httpStatus: http.StatusNoContent,
			want:       "no_content",
		},
		{
			httpStatus: http.StatusNonAuthoritativeInfo,
			want:       "non_authoritative_information",
		},
		{
			httpStatus: http.StatusTeapot,
			want:       "i_m_a_teapot",
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.want, func(t *testing.T) {
			t.Parallel()

			code := code(testCase.err, testCase.httpStatus)
			if code != testCase.want {
				t.Errorf("got '%s', want '%s'", code, testCase.want)
			}
		})
	}
}
