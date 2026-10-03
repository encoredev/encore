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
