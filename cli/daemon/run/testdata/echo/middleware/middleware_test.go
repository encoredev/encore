package middleware

import (
	"context"
	"testing"

	"encore.dev/beta/errs"
)

// TestMiddleware tests that middleware is executed during tests.
func TestMiddleware(t *testing.T) {
	ctx := context.Background()
	if err := Error(ctx); err == nil {
		t.Error("Error(): want non-nil error, got nil")
	} else if ee, ok := err.(*errs.Error); !ok {
		t.Errorf("Error(): want *errs.Error, got %T", err)
	} else if ee.Code != errs.Internal {
		t.Errorf("Error(): want code=Internal, got %v", ee.Code)
	} else if ee.Message != "middleware error" {
		t.Errorf("Error(): want msg=\"middleware error\", got %q", ee.Message)
	}

	resp, err := ResponseRewrite(ctx, &Payload{Msg: "foo"})
	if err != nil {
		t.Errorf("ResponseRewrite(): got non-nil error: %v", err)
	} else if want := "middleware(req=foo, resp=handler(foo))"; resp.Msg != want {
		t.Errorf("ResponseRewrite(): got %q, want %q", resp.Msg, want)
	}

	resp, err = ResponseGen(ctx, &Payload{Msg: "foo"})
	if err != nil {
		t.Errorf("ResponseGen(): got non-nil error: %v", err)
	} else if want := "middleware generated"; resp.Msg != want {
		t.Errorf("ResponseGen(): got %q, want %q", resp.Msg, want)
	}
}
