package legacymeta_test

import (
	"runtime"
	"testing"
	"weak"

	qt "github.com/frankban/quicktest"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/app"
	"encr.dev/v2/app/legacymeta"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
)

// The daemon keeps the metadata for as long as an app runs, so it must not keep the parse alive.
func TestComputeDoesNotRetainParse(t *testing.T) {
	c := qt.New(t)
	md, refs := compute(c, `
-- svc/svc.go --
package svc

import (
	"context"

	"encore.dev/metrics"
	"encore.dev/middleware"
)

type Params struct {
	Header string `+"`header:\"X-Header\"`"+`
	Query  string `+"`query:\"q\"`"+`
	Cookie string `+"`cookie:\"c\"`"+`
}

//encore:api public method=GET
func Get(ctx context.Context, p *Params) error { return nil }

//encore:middleware target=all
func MW(req middleware.Request, next middleware.Next) middleware.Response { return next(req) }

var Requests = metrics.NewCounter[uint64]("requests", metrics.CounterConfig{})
`)
	// The service-scoped middleware and metric, and the three wire tags, are what used to be pointed into.
	c.Assert(md.Middleware, qt.HasLen, 1)
	c.Assert(md.Metrics, qt.HasLen, 1)
	c.Assert(refs, qt.HasLen, 4)

	runtime.GC()
	for _, ref := range refs {
		c.Assert(ref.freed(), qt.IsTrue, qt.Commentf("the metadata still references %s from the parse", ref.name))
	}
	runtime.KeepAlive(md)
}

// weakRef is a weak reference to an object of the parse, which is freed once nothing else holds it.
type weakRef struct {
	name  string
	freed func() bool
}

// compute parses the app and returns its metadata, with weak references to the parse objects that
// metadata fields have pointed into: each service, and the wire tags of each endpoint's request.
func compute(c *qt.C, archiveContent string) (*meta.Data, []weakRef) {
	archive := testutil.ParseTxtar(`
-- go.mod --
module example.com

go 1.20

require encore.dev v1.52.0

` + archiveContent)

	tc := testutil.NewContext(c, false, archive)
	tc.GoModDownload()
	tc.FailTestOnBailout()
	desc := app.ValidateAndDescribe(tc.Context, parser.NewParser(tc.Context).Parse())
	md, _ := legacymeta.Compute(tc.Errs, desc)
	if tc.Errs.Len() > 0 {
		c.Fatalf("parsing failed: %v", tc.Errs.FormatErrors())
	}

	var refs []weakRef
	for _, svc := range desc.Services {
		refs = append(refs, weakRefTo("service "+svc.Name, svc))
		fw, ok := svc.Framework.Get()
		if !ok {
			continue
		}
		for _, ep := range fw.Endpoints {
			ref, ok := schemautil.ResolveNamedStruct(ep.Request, false)
			if !ok {
				continue
			}
			for _, f := range ref.Decl.Type.(schema.StructType).Fields {
				for _, key := range []string{"header", "query", "cookie"} {
					if tag, _ := f.Tag.Get(key); tag != nil {
						refs = append(refs, weakRefTo("the "+key+" tag of "+ep.Name, tag))
					}
				}
			}
		}
	}
	return md, refs
}

func weakRefTo[T any](name string, ptr *T) weakRef {
	w := weak.Make(ptr)
	return weakRef{name: name, freed: func() bool { return w.Value() == nil }}
}
