package metascrub

import (
	"os"
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encr.dev/parser"
	"encr.dev/pkg/scrub"
	meta "encr.dev/proto/encore/parser/meta/v1"
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
	p := cmp.Compute(rpc.RequestSchema)

	f := func(name string) scrub.PathEntry {
		return scrub.PathEntry{Kind: scrub.ObjectField, FieldName: strconv.Quote(name)}
	}
	fCase := func(name string) scrub.PathEntry {
		return scrub.PathEntry{Kind: scrub.ObjectField, FieldName: strconv.Quote(name), CaseSensitive: true}
	}
	mapKey := scrub.PathEntry{Kind: scrub.MapKey}
	mapVal := scrub.PathEntry{Kind: scrub.MapValue}

	c.Assert(p, qt.DeepEquals, []scrub.Path{
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
	})

}

func testParse(c *qt.C, code string) *meta.Data {
	code = strings.Replace(code, "SCRUB", "`encore:\"sensitive\"`", -1)
	ar := txtar.Parse([]byte(code))
	ar.Files = append(ar.Files, txtar.File{Name: "go.mod", Data: []byte("module test")})

	root := c.TempDir()
	err := txtar.Write(ar, root)
	c.Assert(err, qt.IsNil)

	cfg := &parser.Config{
		AppRoot:    root,
		ModulePath: "test",
		WorkingDir: ".",
	}
	p, err := parser.Parse(cfg)
	c.Assert(err, qt.IsNil)
	return p.Meta
}
