package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParseIfBlockDesugar(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
if env.type == "production" {
    for service {
        cpu: default 1
    }
    if env.name == "prod-eu" {
        service "api" {
            cpu: default 2
        }
        sql_cluster "main" {
            engine: "postgres"
        }
    }
    for bucket {
        public_access: false
    }
}
for service {
    cpu: default 0.5
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)

	headers := make([]string, len(f.Rules))
	for i, r := range f.Rules {
		headers[i] = r.Header()
	}
	c.Assert(headers, qt.DeepEquals, []string{
		`for service if env.type == "production"`,
		`service "api" if env.type == "production" && env.name == "prod-eu"`,
		`sql_cluster "main" if env.type == "production" && env.name == "prod-eu"`,
		`for bucket if env.type == "production"`,
		`for service`,
	})
}

// TestEvalIfBlockScoping: rules inside if blocks behave exactly like their
// desugared form, including for specificity. Per-resource conditions go on the
// rule's own where clause.
func TestEvalIfBlockScoping(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: default 0.5
}
if env.type == "production" {
    for service {
        cpu: default 1
    }
    for service if team == "payments" {
        cpu: default 2
    }
}
`)
	result := evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(2))

	result = evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "platform"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(1))

	result = evalOK(c, rs, &Resource{Kind: "service"})
	assertValue(c, result.Properties["cpu"].Value, Number(0.5))
}

// TestValidateIfBlockContradiction: a rule's own selector composes with the
// enclosing if block, so contradictions are detected.
func TestValidateIfBlockContradiction(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
if env.type == "production" {
    for service if env.type == "staging" {
        cpu: >= 1
    }
}
`)
	assertErrContains(c, rs.Validate(),
		"this rule can never match",
		"'env.type == \"staging\"' contradicts 'env.type == \"production\"'")
}

// TestValidateIfBlockResourceAttr: an if block tests only environment
// attributes; resource attributes belong on the rule's where clause.
func TestValidateIfBlockResourceAttr(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
if team == "payments" {
    for service {
        cpu: default 1
    }
}
`)
	assertErrContains(c, rs.Validate(),
		"'team' is not an environment attribute and cannot be tested in an 'if' block",
		"move resource conditions to each rule's 'if' clause")

	// A custom environment scope can allow additional attributes.
	rs = parseSet(c, `
if region == "eu" {
    for service {
        cpu: default 1
    }
}
`)
	rs.EnvScope = []string{"env", "region"}
	c.Assert(rs.Validate(), qt.IsNil)
}

// TestParseWhereBlockRemoved: the standalone where block is now an if block.
func TestParseWhereBlockRemoved(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("policy.encore", []byte("where env.type == \"production\" {\n    for service { cpu: default 1 }\n}\n"))
	assertErrContains(c, err,
		"'where' blocks are now written as 'if' blocks",
		"if env.type == \"production\" { ... }")
}

// TestParseWhereClauseRemoved: the where clause on a rule is now an if clause.
func TestParseWhereClauseRemoved(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("policy.encore", []byte("for service where env.type == \"production\" {\n}\n"))
	assertErrContains(c, err,
		"conditions on a rule now use 'if', not 'where'",
		`for service if env.type == "production" { ... }`)
}

func TestParseIfBlockErrors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "missing condition",
			src:  "if {\n    for service {\n        cpu: default 1\n    }\n}\n",
			want: []string{"expected a condition after 'if'", "e.g.: if env.type == \"production\" { ... }"},
		},
		{
			name: "property directly in if block",
			src:  "if env.type == \"production\" {\n    cpu: default 1\n}\n",
			want: []string{"property rules must appear inside a rule body, not at this level"},
		},
		{
			name: "missing brace",
			src:  "if env.type == \"production\"\n",
			want: []string{"expected '{' to begin the if block, found newline"},
		},
		{
			name: "unclosed if block",
			src:  "if env.type == \"production\" {\n    for service {\n        cpu: default 1\n    }\n",
			want: []string{"expected '}' to close the if block, found end of file"},
		},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			_, err := ParseFile("policy.encore", []byte(tt.src))
			assertErrContains(c, err, tt.want...)
		})
	}
}

// TestParseIfBlockRecovery: a rule with an error inside an if block does not
// break the rest of the block.
func TestParseIfBlockRecovery(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `if env.type == "production" {
    for service {
        cpu: default 2 | <= 4
    }
    for bucket {
        public_access: false
    }
}
`
	f, err := ParseFile("policy.encore", []byte(src))
	c.Assert(err, qt.IsNotNil)
	errs := err.(ErrorList)
	c.Assert(errs, qt.HasLen, 1)
	c.Assert(errs[0].Message, qt.Contains, "'default' must be the last clause")
	c.Assert(f.Rules, qt.HasLen, 2)
	c.Assert(f.Rules[1].Header(), qt.Equals, "for bucket if env.type == \"production\"")
}

func TestParseVersionAsPropertyName(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// "version" is not a reserved keyword: it works as a property name
	// and as a selector field, while the version declaration still parses.
	src := `version 1
sql_cluster "main" {
    version: "16"
}
for service if version == "2" {
    cpu: default 1
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Version.Num, qt.Equals, 1)
	c.Assert(f.Rules[0].Props[0].String(), qt.Equals, `version: "16"`)
	c.Assert(f.Rules[1].Header(), qt.Equals, `for service if version == "2"`)
}
