package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// These tests pin down the exact rendering of diagnostics: position,
// source snippet with caret, detail lines, notes, and help text.

func TestDiagRenderParseError(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("policy.encore", []byte("for service if env.type = \"production\" {\n}\n"))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, `policy.encore:1:25: error: '=' is not an operator
   |
 1 | for service if env.type = "production" {
   |                         ^
  help: use '==' for equality comparisons`)
}

func TestDiagRenderCaretWidth(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("policy.encore", []byte("for service {\n    memory: >= 512G\n}\n"))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, `policy.encore:2:16: error: unknown unit 'G' in '512G'
   |
 2 |     memory: >= 512G
   |                ^^^^
  help: did you mean '512GB'? valid units are B, KB, Ki, MB, Mi, GB, Gi, TB, Ti (size) and ms, s, m, h, d (duration)`)
}

func TestDiagRenderAmbiguousDefault(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `for service if env.type == "production" {
    cpu: default 1
}
for service if team == "payments" {
    cpu: default 2
}
`)
	_, err := rs.Evaluate(&Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, `policy.encore:2:18: error: ambiguous default for property 'cpu' of service "api"
   |
 2 |     cpu: default 1
   |                  ^
  matching rules provide different defaults:
    policy.encore:1:1: for service if env.type == "production"
        cpu: default 1
    policy.encore:4:1: for service if team == "payments"
        cpu: default 2
  no rule is more specific than all the others
  help: add a more specific rule that decides the default, e.g.:
    for service if env.type == "production" && team == "payments" {
        cpu: default 2
    }`)
}

func TestDiagRenderDefaultViolation(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `for service {
    cpu: <= 4
}
service "api" {
    cpu: 8
}
`)
	_, err := rs.Evaluate(&Resource{Kind: "service", Name: "api"})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, `policy.encore:2:10: error: service "api": default value 8 for property 'cpu' violates constraint '<= 4'
   |
 2 |     cpu: <= 4
   |          ^^^^
  the constraint is defined at policy.encore:1:1 in rule:
    for service
        cpu: <= 4
  note: the default is defined at policy.encore:5:10 in rule: service "api"`)
}

func TestDiagRenderMultipleErrors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Multiple diagnostics are rendered separated by blank lines, in
	// source order.
	_, err := ParseFile("policy.encore", []byte("for service {\n    cpu = 1\n    mem = 2\n}\n"))
	c.Assert(err, qt.IsNotNil)
	errs := err.(ErrorList)
	c.Assert(errs, qt.HasLen, 2)
	c.Assert(errs[0].Summary(), qt.Equals, "policy.encore:2:9: property rules use ':', not '='")
	c.Assert(errs[1].Summary(), qt.Equals, "policy.encore:3:9: property rules use ':', not '='")
	c.Assert(err.Error(), qt.Contains, "\n\n")
}

func TestDiagTabAlignment(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Tabs in the source line are mirrored in the caret line so the
	// caret stays aligned regardless of tab width.
	_, err := ParseFile("policy.encore", []byte("for service {\n\tcpu = 1\n}\n"))
	c.Assert(err, qt.IsNotNil)
	errs := err.(ErrorList)
	c.Assert(errs[0].Error(), qt.Contains, " 2 | \tcpu = 1\n   | \t    ^")
}
