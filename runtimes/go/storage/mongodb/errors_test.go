package mongodb

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"encore.dev/beta/errs"
)

func TestConvertErr(t *testing.T) {
	tests := []struct {
		err  error
		want errs.ErrCode
	}{
		{err: nil, want: errs.OK},
		{err: io.EOF, want: errs.Unknown},
		{err: mongo.ErrNoDocuments, want: errs.NotFound},
		{err: fmt.Errorf("wrapped: %w", mongo.ErrNoDocuments), want: errs.NotFound},
	}
	for _, tt := range tests {
		got := convertErr(tt.err)
		if code := errs.Code(got); code != tt.want {
			t.Errorf("convertErr(%v) has code %v, want %v", tt.err, code, tt.want)
		}
		if tt.err != nil && !errors.Is(got, tt.err) {
			t.Errorf("convertErr(%v) = %v, want it to wrap the original error", tt.err, got)
		}
	}
}

func TestServerError(t *testing.T) {
	dup := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{Code: 11000, Message: "E11000 duplicate key error"}},
	}
	conflict := mongo.CommandError{Code: 112, Message: "WriteConflict", Labels: []string{"TransientTransactionError"}}

	tests := []struct {
		err        error
		wantCode   int
		wantLabels []string
		wantDup    bool
	}{
		{err: dup, wantCode: 11000, wantDup: true},
		{err: fmt.Errorf("wrapped: %w", dup), wantCode: 11000, wantDup: true},
		{err: conflict, wantCode: 112, wantLabels: []string{"TransientTransactionError"}},
	}
	for _, tt := range tests {
		got := convertErr(tt.err)
		var e *Error
		if !errors.As(got, &e) {
			t.Fatalf("convertErr(%v) = %T, want *Error", tt.err, got)
		}
		if e.Code != tt.wantCode {
			t.Errorf("convertErr(%v).Code = %d, want %d", tt.err, e.Code, tt.wantCode)
		}
		for _, l := range tt.wantLabels {
			if !e.HasErrorLabel(l) {
				t.Errorf("convertErr(%v) is missing label %q", tt.err, l)
			}
		}
		if IsDuplicateKey(got) != tt.wantDup {
			t.Errorf("IsDuplicateKey(%v) = %v, want %v", got, !tt.wantDup, tt.wantDup)
		}
		// The driver's error stays reachable.
		var se mongo.ServerError
		if !errors.As(got, &se) {
			t.Errorf("convertErr(%v) does not unwrap to a mongo.ServerError", tt.err)
		}
	}
}
