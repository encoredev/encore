package clientgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/clientgen/clientgentypes"
	"encr.dev/v2/v2builder"
)

// TestQuoteJSString verifies that values which would otherwise break out of a
// JS/TS string literal are escaped.
func TestQuoteJSString(t *testing.T) {
	c := qt.New(t)
	cases := []struct {
		in   string
		want string
	}{
		{`plain`, `"plain"`},
		{`x-token`, `"x-token"`},
		{`a"b`, `"a\"b"`},
		{`a\b`, `"a\\b"`},
		{"line\nbreak", `"line\nbreak"`},
		{"tab\there", `"tab\there"`},
		// The header-name breakout payload from the vulnerability report.
		{`x");evil();("`, `"x\");evil();(\""`},
	}
	for _, tc := range cases {
		c.Assert(quoteJSString(tc.in), qt.Equals, tc.want, qt.Commentf("input %q", tc.in))
		// The result must never contain an unescaped double-quote in its interior.
		inner := quoteJSString(tc.in)
		inner = inner[1 : len(inner)-1]
		c.Assert(strings.Contains(strings.ReplaceAll(inner, `\"`, ""), `"`), qt.IsFalse)
	}
}

// TestEscapeDocComment verifies that a doc comment cannot terminate the
// enclosing block comment.
func TestEscapeDocComment(t *testing.T) {
	c := qt.New(t)
	c.Assert(strings.Contains(escapeDocComment(`ok */ evil()`), `*/`), qt.IsFalse)
	c.Assert(strings.Contains(escapeDocComment(`**/`), `*/`), qt.IsFalse)
	c.Assert(escapeDocComment(`no terminator here`), qt.Equals, `no terminator here`)
}

// TestClientGenInjectionNeutralized reproduces the code-injection vectors from
// the vulnerability report against the real generator and asserts the emitted
// client is neither syntactically broken nor executes injected code on import.
func TestClientGenInjectionNeutralized(t *testing.T) {
	c := qt.New(t)

	// The service (package) doc breaks out of a /** */ block comment; the
	// response header wire name breaks out of a string literal.
	src := `-- go.mod --
module app

require (
	encore.dev v1.52.1
)

-- encore.app --
{"id": ""}

-- evil/evil.go --
// Package evil is documented.
// */ ;import("node:fs").then(f=>f.writeFileSync(process.env.CLIENTGEN_MARKER,"pwned")); /*
package evil

import "context"

// Evil is a normal endpoint.
// */ ;import("node:fs").then(f=>f.writeFileSync(process.env.CLIENTGEN_MARKER,"pwned")); /*
type Response struct {
	Token string ` + "`header:\"x\\\");import(process.env.clientgen_marker);((\\\"\"`" + `
}

//encore:api public method=GET path=/evil
func Evil(ctx context.Context) (*Response, error) {
	return &Response{}, nil
}
`

	base := t.TempDir()
	c.Assert(txtar.Write(txtar.Parse([]byte(src)), base), qt.IsNil)

	ctx := context.Background()
	bld := v2builder.New()
	app := apps.NewInstance(base, "app", "")
	prepareResult, err := bld.Prepare(ctx, builder.PrepareParams{
		Build: builder.DefaultBuildInfo(), App: app, WorkingDir: ".",
	})
	c.Assert(err, qt.IsNil)
	res, err := bld.Parse(ctx, builder.ParseParams{
		Build: builder.DefaultBuildInfo(), App: app, WorkingDir: ".", Prepare: prepareResult,
	})
	c.Assert(err, qt.IsNil)

	services := clientgentypes.AllServices(res.Meta)

	genJS, err := Client(LangJavascript, "app", res.Meta, services, clientgentypes.TagSet{}, clientgentypes.Options{})
	c.Assert(err, qt.IsNil)
	genTS, err := Client(LangTypeScript, "app", res.Meta, services, clientgentypes.TagSet{}, clientgentypes.Options{})
	c.Assert(err, qt.IsNil)

	// The doc payload must be neutralized: no "*/" that would close the comment.
	// (The escaped form "*\/" is what should appear instead.)
	c.Assert(strings.Contains(string(genJS), `*/ ;import`), qt.IsFalse)
	c.Assert(strings.Contains(string(genTS), `*/ ;import`), qt.IsFalse)

	node, lookErr := exec.LookPath("node")
	if lookErr != nil {
		t.Skip("node not available; skipping runtime checks")
	}

	jsPath := filepath.Join(base, "client.mjs")
	c.Assert(os.WriteFile(jsPath, genJS, 0o644), qt.IsNil)

	// 1. The generated client must be syntactically valid JS. Before the fix the
	//    header breakout produced "SyntaxError: missing ) after argument list".
	checkOut, err := exec.Command(node, "--check", jsPath).CombinedOutput()
	c.Assert(err, qt.IsNil, qt.Commentf("node --check failed: %s", checkOut))

	// 2. Merely importing the client must not execute injected code. The payloads
	//    would write CLIENTGEN_MARKER; assert the file is never created.
	marker := filepath.Join(base, "MARKER")
	runCmd := exec.Command(node, "--input-type=module", "-e",
		"await import(process.argv[1]); await new Promise(r=>setTimeout(r,200));", jsPath)
	runCmd.Env = append(os.Environ(), "CLIENTGEN_MARKER="+marker, "clientgen_marker="+marker)
	runOut, _ := runCmd.CombinedOutput()
	_, statErr := os.Stat(marker)
	c.Assert(os.IsNotExist(statErr), qt.IsTrue,
		qt.Commentf("injected payload executed on import; node output: %s", runOut))
}
