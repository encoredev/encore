package typescrub

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/scrub"
	"encr.dev/v2/app"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
)

func TestScrub(t *testing.T) {
	c := qt.New(t)
	reqType := testParse(c, `
-- svc/svc.go --
package svc
import (
	"context"
	"encore.dev/types/option"
)

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

	Option option.Option[Generic[string]]
	OptionScrub option.Option[Generic[string]] SCRUB

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

	cmp := NewComputer(zerolog.New(os.Stdout))
	res := cmp.Compute(reqType, 0)

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

		{f("Option"), f("FooScrub")},
		{f("OptionScrub")},

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
	reqType := testParse(c, `
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

	cmp := NewComputer(zerolog.New(os.Stdout))
	res := cmp.Compute(reqType, AuthHandler)

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

// testParse parses the given txtar code and returns the request schema.Type
// for the first endpoint in the first service.
func testParse(c *qt.C, code string) schema.Type {
	c.Helper()
	code = strings.ReplaceAll(code, "SCRUB", "`encore:\"sensitive\"`")
	ar := txtar.Parse([]byte(code))

	tc := testutil.NewContext(c, false, ar)
	tc.FailTestOnErrors()

	// Create a go.mod file if it doesn't already exist.
	modPath := tc.MainModuleDir.Join("go.mod").ToIO()
	if _, err := os.Stat(modPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			c.Fatal(err)
		}
		modContents := "module example.com\nrequire encore.dev v1.52.0"
		err := os.WriteFile(modPath, []byte(modContents), 0644)
		c.Assert(err, qt.IsNil)
	}

	tc.GoModTidy()
	tc.GoModDownload()

	p := parser.NewParser(tc.Context)
	parserResult := p.Parse()
	appDesc := app.ValidateAndDescribe(tc.Context, parserResult)

	svc := appDesc.Services[0]
	fw, ok := svc.Framework.Get()
	if !ok {
		c.Fatal("service has no framework")
	}
	ep := fw.Endpoints[0]
	if ep.Request == nil {
		c.Fatal("endpoint has no request type")
	}
	return ep.Request
}
