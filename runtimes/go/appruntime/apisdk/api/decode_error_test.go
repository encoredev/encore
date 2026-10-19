package api

import (
	"fmt"
	"strings"
	"testing"

	"encore.dev/beta/errs"
)

type fakeBodyError struct{ error }

func (fakeBodyError) IsRequestBodyError() {}

func TestPublicDecodeErr_Body(t *testing.T) {
	decodeErr := fmt.Errorf("Title: invalid json parameter: %w",
		fakeBodyError{fmt.Errorf(`found 1, error found in #10 byte of ...|"title": 123}|...`)})

	err := publicDecodeErr(decodeErr).(*errs.Error)
	if err.Code != errs.InvalidArgument {
		t.Fatalf("code = %v, want invalid_argument", err.Code)
	}
	if got, want := err.ErrorMessage(), "unable to decode request body"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got, _ := err.Meta["decode_error"].(string); !strings.Contains(got, "Title") {
		t.Errorf("decode_error meta = %q, want the full decode error", got)
	}
}

func TestPublicDecodeErr_Parameter(t *testing.T) {
	err := publicDecodeErr(fmt.Errorf("limit: invalid parameter: bad int")).(*errs.Error)
	if got, want := err.ErrorMessage(), "decode request: limit: invalid parameter: bad int"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if err.Meta["decode_error"] != nil {
		t.Errorf("parameter errors must not be redacted")
	}
}
