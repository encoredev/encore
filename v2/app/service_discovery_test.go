package app

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func Test_discoverServices(t *testing.T) {
	tests := []struct {
		name         string
		txtar        string
		wantServices []string
		wantErrors   []string
	}{
		// Happy Path Tests
		{
			name: "services defined by APIs",
			txtar: `
-- systemA/svc1/foo.go --
package svc1

import "context"

//encore:api public
func Foo(ctx context.Context) error { return nil }

-- systemA/svc2/bar.go --
package svc2

import "context"

//encore:api public
func Bar(ctx context.Context) error { return nil }

-- svc3/bar.go --
package svc3

import "context"

//encore:api public
func Bar(ctx context.Context) error { return nil }
`,
			wantServices: []string{"svc1", "svc2", "svc3"},
		},
		{
			name: "services defined by service structs",
			txtar: `
-- svc1/foo.go --
package svc1

import "context"

//encore:service
type Foo struct{}

func initFoo() (*Foo, error) { return nil, nil }
`,
			wantServices: []string{"svc1"},
		},
		{
			name: "services defined by pubsub subscriptions",
			txtar: `
-- svc1/foo.go --
package svc1

import (
 	"context"

	"encore.dev/pubsub"
)

type Msg struct{}

var T = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{ DeliveryGuarantee: pubsub.AtLeastOnce })

var _ = pubsub.NewSubscription[Msg](T, "sub", pubsub.SubscriptionConfig[Msg]{
	Handler: func(ctx context.Context, msg Msg) error { return nil },
})
`,
			wantServices: []string{"svc1"},
		},
		{
			name: "services with nested packages with API's and pubsub subscriptions",
			txtar: `
-- svc1/foo.go --
package svc1

import "context"

//encore:api public
func Foo(ctx context.Context) error { return nil }

-- svc1/events/bar.go --
package events

import (
	"context"
	
	"encore.dev/pubsub"
)

type Msg struct{}

var T = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{ DeliveryGuarantee: pubsub.AtLeastOnce })

var _ = pubsub.NewSubscription[Msg](T, "sub", pubsub.SubscriptionConfig[Msg]{
	Handler: func(ctx context.Context, msg Msg) error { return nil },
})

-- svc1/apis/baz.go --
package apis

import "context"

//encore:api public
func Baz(ctx context.Context) error { return nil }
`,
			wantServices: []string{"svc1"},
		},

		// Error Tests
		{
			name: "error if no services",
			txtar: `
-- svc1/foo.go --
package svc1

type Foo struct{}
`,
			wantErrors: []string{"No services were found in the application"},
		},
		{
			// Note: this test is also testing that the Go package name is being used
			// by the API framework - not the folder names/path
			name: "error on duplicate service names",
			txtar: `
-- systemA/svc1/foo.go --
package svc

import "context"

//encore:api public
func Foo(ctx context.Context) error { return nil }

-- systemB/svc2/bar.go --
package svc

import "context"

//encore:api public
func Bar(ctx context.Context) error { return nil }
`,
			wantErrors: []string{"Two services were found with the same name \"svc\", services must have unique names"},
		},
		{
			name: "error if services declared in each other",
			txtar: `
-- svc1/foo.go --
package svc1

import "context"

//encore:api public
func Foo(ctx context.Context) error { return nil }

-- svc1/svc2/bar.go --
package svc2

//encore:service
type Bar struct{}

func initBar() (*Bar, error) { return nil, nil }
`,
			wantErrors: []string{"The service svc2 was found within the service svc1. Encore does not allow services to be nested"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := qt.New(t)

			ctx, result := Parse(c, tt.txtar)

			services := discoverServices(ctx.Context, result)

			if len(tt.wantErrors) > 0 {
				for _, s := range services {
					c.Logf("got service: %s (%s)", s.Name, s.FSRoot)
				}
				ctx.DeferExpectError(tt.wantErrors...)
			} else {
				ctx.FailTestOnErrors()

				wanted := make(map[string]struct{})
				for _, s := range tt.wantServices {
					wanted[s] = struct{}{}
				}

				for _, s := range services {
					if _, ok := wanted[s.Name]; !ok {
						c.Errorf("unexpected service %q", s.Name)
					}
					delete(wanted, s.Name)
				}

				for s := range wanted {
					c.Errorf("missing service %q", s)
				}
			}

		})
	}
}
