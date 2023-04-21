package metascrub

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/scrub"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/v2builder"
)

func TestScrub(t *testing.T) {
	c := qt.New(t)
	md := testParse(c, `
-- svc/svc.go --
package svc
import "context"

type Params struct {
	Foo string SCRUB

	NestedScrub struct {
		Inner string SCRUB
	} SCRUB

	Nested struct {
		Inner string SCRUB
	}

	List []string SCRUB
	ListInner []struct { Inner string SCRUB }

	RecurseScrub *Params SCRUB
	// Ideally this should yield nested scrubs but we don't support that yet.
	Recurse *Params

	Gen      Generic[string]
	GenScrub Generic[string] SCRUB
	GenInner Generic[Bar]
	Multi    NestedGeneric[string, Bar]

	MapOne map[Generic[Bar]]NestedGeneric[string, Bar]
	MapTwo GenericMap[Bar, NestedGeneric[string, Bar]]

	JsonKey string `+"`"+`json:"json_key" encore:"sensitive"`+"`"+`
	Header string `+"`"+`header:"X-Header" encore:"sensitive"`+"`"+`
}

type Generic[T any] struct {
	Foo T
	FooScrub T SCRUB
}

type NestedGeneric[A any, B any] struct {
	One NestedGenericTwo[B, string]
	Two B
}

type NestedGenericTwo[A any, B any] struct {
	Alpha A
	Beta B
}

type GenericMap[K comparable, V any] struct {
	Foo map[K]V
}

type Bar struct {
	Scrub string SCRUB
}

//encore:api public
func Foo(ctx context.Context, p *Params) error { return nil }
`)

	cmp := New(md, zerolog.New(os.Stdout))
	rpc := md.Svcs[0].Rpcs[0]
	res := cmp.Compute(rpc.RequestSchema, 0)

	f := func(name string) scrub.PathEntry {
		return scrub.PathEntry{Kind: scrub.ObjectField, FieldName: strconv.Quote(name)}
	}
	fCase := func(name string) scrub.PathEntry {
		return scrub.PathEntry{Kind: scrub.ObjectField, FieldName: strconv.Quote(name), CaseSensitive: true}
	}
	mapKey := scrub.PathEntry{Kind: scrub.MapKey}
	mapVal := scrub.PathEntry{Kind: scrub.MapValue}

	c.Assert(res.Payload, qt.DeepEquals, []scrub.Path{
		{f("Foo")},
		{f("NestedScrub")},
		{f("Nested"), f("Inner")},
		{f("List")},
		{f("ListInner"), f("Inner")},
		{f("RecurseScrub")},
		{f("Gen"), f("FooScrub")},
		{f("GenScrub")},
		{f("GenInner"), f("FooScrub")},
		{f("GenInner"), f("Foo"), f("Scrub")},
		{f("Multi"), f("One"), f("Alpha"), f("Scrub")},
		{f("Multi"), f("Two"), f("Scrub")},

		{f("MapOne"), mapKey, f("FooScrub")},
		{f("MapOne"), mapKey, f("Foo"), f("Scrub")},
		{f("MapOne"), mapVal, f("One"), f("Alpha"), f("Scrub")},
		{f("MapOne"), mapVal, f("Two"), f("Scrub")},
		{f("MapTwo"), f("Foo"), mapKey, f("Scrub")},
		{f("MapTwo"), f("Foo"), mapVal, f("One"), f("Alpha"), f("Scrub")},
		{f("MapTwo"), f("Foo"), mapVal, f("Two"), f("Scrub")},
		{fCase("json_key")},
		{f("Header")},
	})

	c.Assert(res.Headers, qt.DeepEquals, []string{"X-Header"})
}

func TestScrubAuthHandler(t *testing.T) {
	c := qt.New(t)
	md := testParse(c, `
-- svc/svc.go --
package svc
import "context"

type Params struct {
	Header string `+"`"+`header:"Foo"`+"`"+`
	Query string `+"`"+`query:"query"`+"`"+`
	Other string
}

//encore:api public
func Foo(ctx context.Context, p *Params) error { return nil }
`)

	cmp := New(md, zerolog.New(os.Stdout))
	rpc := md.Svcs[0].Rpcs[0]
	res := cmp.Compute(rpc.RequestSchema, AuthHandler)

	f := func(name string) scrub.PathEntry {
		return scrub.PathEntry{Kind: scrub.ObjectField, FieldName: strconv.Quote(name)}
	}

	c.Assert(res.Payload, qt.DeepEquals, []scrub.Path{
		{f("Header")},
		{f("Query")},
		{f("Other")},
	})

	c.Assert(res.Headers, qt.DeepEquals, []string{"Foo"})
}

func testParse(c *qt.C, code string) *meta.Data {
	code = strings.Replace(code, "SCRUB", "`encore:\"sensitive\"`", -1)
	ar := txtar.Parse([]byte(code))
	ar.Files = append(ar.Files, txtar.File{Name: "go.mod", Data: []byte("module test")})

	root := c.TempDir()
	err := txtar.Write(ar, root)
	c.Assert(err, qt.IsNil)

	bld := v2builder.BuilderImpl{}
	ctx := context.Background()

	res, err := bld.Parse(ctx, builder.ParseParams{
		Build:       builder.DefaultBuildInfo(),
		App:         apps.NewInstance(root, "test", ""),
		Experiments: nil,
		WorkingDir:  ".",
		ParseTests:  false,
	})
	c.Assert(err, qt.IsNil)
	return res.Meta
}
