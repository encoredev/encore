package natspubsub

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/v2/internals/perr"
	"encr.dev/v2/parser/apis/directive"
)

func TestParsePubSubDirective(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantOK   bool
		wantErr  string
		subject  string
		wantName string
	}{
		{
			name: "valid pubsub handler",
			src: `package svc
import "context"

type Event struct{}

//encore:pubsub orders.created
func HandleOrderCreated(ctx context.Context, evt *Event) error { return nil }
`,
			wantOK:   true,
			subject:  "orders.created",
			wantName: "pubsub",
		},
		{
			name: "missing subject",
			src: `package svc
import "context"

type Event struct{}

//encore:pubsub
func HandleOrderCreated(ctx context.Context, evt *Event) error { return nil }
`,
			wantErr: `Plugin parser for //encore:pubsub failed: pubsub directive requires exactly one subject argument,\s*got 0`,
		},
		{
			name: "too many subjects",
			src: `package svc
import "context"

type Event struct{}

//encore:pubsub orders.created orders.updated
func HandleOrderCreated(ctx context.Context, evt *Event) error { return nil }
`,
			wantErr: `Plugin parser for //encore:pubsub failed: pubsub directive requires exactly one subject argument,\s*got 2`,
		},
		{
			name: "wrong parameter count",
			src: `package svc
import "context"

type Event struct{}

//encore:pubsub orders.created
func HandleOrderCreated(ctx context.Context) error { return nil }
`,
			wantErr: `Plugin parser for //encore:pubsub failed: pubsub: handler must have two parameters \(ctx, \*Event\)`,
		},
		{
			name: "wrong return count",
			src: `package svc
import "context"

type Event struct{}

//encore:pubsub orders.created
func HandleOrderCreated(ctx context.Context, evt *Event) {}
`,
			wantErr: `Plugin parser for //encore:pubsub failed: pubsub: handler must return exactly one value \(error\)`,
		},
		{
			name: "wrong return type",
			src: `package svc
import "context"

type Event struct{}

//encore:pubsub orders.created
func HandleOrderCreated(ctx context.Context, evt *Event) int { return 0 }
`,
			wantErr: `Plugin parser for //encore:pubsub failed: pubsub: handler must return error`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, "svc.go", tc.src, parser.ParseComments)
			c.Assert(err, qt.IsNil)

			fn := firstFuncDecl(f)
			c.Assert(fn, qt.IsNotNil)

			errs := perr.NewList(context.Background(), fs)
			dir, _, ok := directive.Parse(errs, fn)

			if tc.wantErr != "" {
				c.Assert(ok, qt.IsFalse)
				errStr := errs.FormatErrors()
				re := regexp.MustCompile(tc.wantErr)
				if !re.MatchString(errStr) {
					c.Fatalf("error did not match regexp %s: %s", tc.wantErr, errStr)
				}
				c.Assert(dir, qt.IsNil)
				return
			}

			c.Assert(ok, qt.IsTrue)
			c.Assert(dir, qt.IsNotNil)
			c.Assert(dir.Name, qt.Equals, tc.wantName)
			c.Assert(len(dir.Options), qt.Equals, 1)
			c.Assert(dir.Options[0].Value, qt.Equals, tc.subject)
		})
	}
}

func firstFuncDecl(f *ast.File) *ast.FuncDecl {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return fn
		}
	}
	return nil
}
