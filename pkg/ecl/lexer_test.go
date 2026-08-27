package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func lexAll(c *qt.C, src string) ([]token, ErrorList) {
	c.Helper()
	var diags ErrorList
	lx := newLexer(newSourceFile("test.encore", src), &diags)
	return lx.lex(), diags
}

func kindsOf(toks []token) []tokenKind {
	kinds := make([]tokenKind, len(toks))
	for i, t := range toks {
		kinds[i] = t.kind
	}
	return kinds
}

func TestLexerTokens(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, `cpu: >= 1.5 & <= 4Gi | default 2 // trailing comment`)
	c.Assert(diags, qt.HasLen, 0)
	c.Assert(kindsOf(toks), qt.DeepEquals, []tokenKind{
		tokIdent, tokColon, tokGe, tokNumber, tokAmp, tokLe, tokNumber,
		tokPipe, tokDefault, tokNumber, tokEOF,
	})
	c.Assert(toks[3].num, qt.Equals, 1.5)
	c.Assert(toks[3].unit, qt.Equals, "")
	c.Assert(toks[6].num, qt.Equals, 4.0)
	c.Assert(toks[6].unit, qt.Equals, "Gi")
}

func TestLexerOperators(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, `== != >= <= > < && & || | = . , [ ] { } -`)
	c.Assert(diags, qt.HasLen, 0)
	c.Assert(kindsOf(toks), qt.DeepEquals, []tokenKind{
		tokEq, tokNeq, tokGe, tokLe, tokGt, tokLt, tokAndAnd, tokAmp,
		tokOrOr, tokPipe, tokAssign, tokDot, tokComma, tokLBracket,
		tokRBracket, tokLBrace, tokRBrace, tokMinus, tokEOF,
	})
}

func TestLexerKeywords(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, `for where in exists required default import true false forx version`)
	c.Assert(diags, qt.HasLen, 0)
	c.Assert(kindsOf(toks), qt.DeepEquals, []tokenKind{
		tokFor, tokWhere, tokIn, tokExists, tokRequired, tokDefault,
		tokImport, tokTrue, tokFalse,
		// "define", "require" and "version" are not keywords, so they lex
		// as identifiers; "version" is recognized contextually.
		tokIdent, tokIdent, tokEOF,
	})
	c.Assert(toks[9].str, qt.Equals, "forx")
	c.Assert(toks[10].str, qt.Equals, "version")
}

func TestLexerPositions(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, "for service {\n  cpu: 1\n}")
	c.Assert(diags, qt.HasLen, 0)

	c.Assert(toks[0].pos, qt.DeepEquals, Position{File: "test.encore", Offset: 0, Line: 1, Column: 1})
	c.Assert(toks[1].pos.Column, qt.Equals, 5) // service
	c.Assert(toks[2].pos.Column, qt.Equals, 13)
	// newline token, then cpu at line 2 col 3
	c.Assert(toks[3].kind, qt.Equals, tokNewline)
	c.Assert(toks[4].pos, qt.DeepEquals, Position{File: "test.encore", Offset: 16, Line: 2, Column: 3})
	c.Assert(toks[4].end.Column, qt.Equals, 6)
}

func TestLexerNewlineCollapsing(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Leading newlines and blank-line runs collapse; newlines inside
	// brackets are suppressed entirely.
	toks, diags := lexAll(c, "\n\n\na\n\n\nb in [x,\n  y]\n")
	c.Assert(diags, qt.HasLen, 0)
	c.Assert(kindsOf(toks), qt.DeepEquals, []tokenKind{
		tokIdent, tokNewline, tokIdent, tokIn, tokLBracket, tokIdent,
		tokComma, tokIdent, tokRBracket, tokNewline, tokEOF,
	})
}

func TestLexerComments(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, "// line comment\na /* inline */ b\n/* multi\nline */ d")
	c.Assert(diags, qt.HasLen, 0)
	c.Assert(kindsOf(toks), qt.DeepEquals, []tokenKind{
		tokIdent, tokIdent, tokNewline, tokIdent, tokEOF,
	})
}

func TestLexerStrings(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, `"plain" "with \"escapes\"\n\t\\"`)
	c.Assert(diags, qt.HasLen, 0)
	c.Assert(toks[0].str, qt.Equals, "plain")
	c.Assert(toks[1].str, qt.Equals, "with \"escapes\"\n\t\\")
}

func TestLexerErrors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		src     string
		message string
	}{
		{`"unterminated`, "unterminated string literal"},
		{"\"bad \\q escape\"", `invalid escape sequence '\q'`},
		{"/* unterminated", "unterminated block comment"},
		{"a @ b", "unexpected character '@'"},
		{"a ! b", "unexpected '!'; use '!=' for inequality"},
	}
	for _, tt := range tests {
		_, diags := lexAll(c, tt.src)
		c.Assert(diags, qt.Not(qt.HasLen), 0, qt.Commentf("src: %q", tt.src))
		c.Assert(diags[0].Message, qt.Contains, tt.message, qt.Commentf("src: %q", tt.src))
	}
}

func TestLexerNumberUnits(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	toks, diags := lexAll(c, "512Mi 30d 1.5h 100ms 2TB 0.25")
	c.Assert(diags, qt.HasLen, 0)
	want := []struct {
		num  float64
		unit string
	}{
		{512, "Mi"}, {30, "d"}, {1.5, "h"}, {100, "ms"}, {2, "TB"}, {0.25, ""},
	}
	for i, w := range want {
		c.Assert(toks[i].num, qt.Equals, w.num)
		c.Assert(toks[i].unit, qt.Equals, w.unit)
	}
}
