package sqldb

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"encore.dev/storage/sqldb/sqlerr"
)

func TestErrCode(t *testing.T) {
	tests := []struct {
		err  error
		want sqlerr.Code
	}{
		{err: nil, want: sqlerr.Other},
		{err: io.EOF, want: sqlerr.Other},
		{err: errors.New("some error"), want: sqlerr.Other},
		{err: &Error{Code: sqlerr.UniqueViolation}, want: sqlerr.UniqueViolation},
		{err: fmt.Errorf("wrapped: %w", &Error{Code: sqlerr.UniqueViolation}), want: sqlerr.UniqueViolation},
	}
	for _, tt := range tests {
		if got := ErrCode(tt.err); got != tt.want {
			t.Errorf("ErrCode(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
