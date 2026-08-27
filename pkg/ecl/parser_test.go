package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParseRuleHeaders(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
for service {
    cpu: default 1
}
service "api" {
    cpu: default 2
}
for service if env.type == "production" {
    cpu: default 3
}
service "api" if env.type == "production" && team == "payments" {
    cpu: default 4
}
for bucket if tags.data exists {
    versioning: true
}
for service if env.type in ["production", "staging"] {
    cpu: >= 0.5
}
for service if env.type != "preview" {
    cpu: >= 0.5
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Rules, qt.HasLen, 7)

	headers := make([]string, len(f.Rules))
	for i, r := range f.Rules {
		headers[i] = r.Header()
	}
	c.Assert(headers, qt.DeepEquals, []string{
		`for service`,
		`service "api"`,
		`for service if env.type == "production"`,
		`service "api" if env.type == "production" && team == "payments"`,
		`for bucket if tags.data exists`,
		`for service if env.type in ["production", "staging"]`,
		`for service if env.type != "preview"`,
	})

	c.Assert(f.Rules[1].Name, qt.Equals, "api")
	c.Assert(f.Rules[3].Where, qt.HasLen, 2)
	c.Assert(f.Rules[5].Where[0].Op, qt.Equals, CondIn)
	c.Assert(f.Rules[5].Where[0].Values, qt.HasLen, 2)
}

func TestParseDynamicBlock(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
for service if tags.domain exists {
    instance: default service_instance[tags.domain]
    service_instance tags.domain {
        cpu: default 2
    }
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Rules, qt.HasLen, 1)
	parent := f.Rules[0]
	c.Assert(parent.Props, qt.HasLen, 1) // the `instance:` reference property
	c.Assert(parent.Blocks, qt.HasLen, 1)
	b := parent.Blocks[0]
	c.Assert(b.Kind, qt.Equals, "service_instance")
	c.Assert(b.DynExpr, qt.Equals, "tags.domain")
	c.Assert(b.Header(), qt.Equals, "service_instance tags.domain")
}

func TestParsePropertyRules(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
for service {
    cpu: >= 0.25 & <= 8 | default 0.5
    memory: >= 256Mi & <= 16Gi | default 512Mi
    instances.min: default 1
    public_access: false
    region: "europe-west1" | "europe-north1"
    tier: "small" | "medium" | "large"
    backup_retention: required & >= 30d
    timeout: != 30s
    provider.gcp.cloud_run.cpu_always_allocated: true
    explicit: false | default false
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Rules, qt.HasLen, 1)

	props := make([]string, len(f.Rules[0].Props))
	for i, p := range f.Rules[0].Props {
		props[i] = p.String()
	}
	c.Assert(props, qt.DeepEquals, []string{
		`cpu: >= 0.25 & <= 8 | default 0.5`,
		`memory: >= 256Mi & <= 16Gi | default 512Mi`,
		`instances.min: default 1`,
		`public_access: false`,
		`region: "europe-west1" | "europe-north1"`,
		`tier: "small" | "medium" | "large"`,
		`backup_retention: required & >= 30d`,
		`timeout: != 30s`,
		`provider.gcp.cloud_run.cpu_always_allocated: true`,
		`explicit: false | default false`,
	})

	// `default` binds last: the constraint is the conjunction, the
	// default is separate.
	cpu := f.Rules[0].Props[0].scalar()
	and, ok := cpu.Constraint.(*AndConstraint)
	c.Assert(ok, qt.IsTrue)
	c.Assert(and.Terms, qt.HasLen, 2)
	c.Assert(cpu.Default.Value, qt.Equals, Number(0.5))

	// A bare exact value parses as an implicit equality comparison.
	pub := f.Rules[0].Props[3].scalar()
	cmp, ok := pub.Constraint.(*Comparison)
	c.Assert(ok, qt.IsTrue)
	c.Assert(cmp.Op, qt.Equals, OpEq)
	c.Assert(cmp.Implicit, qt.IsTrue)
	c.Assert(cmp.Value, qt.Equals, Bool(false))
	c.Assert(pub.Default, qt.IsNil)
}

func TestParseVersionAndImports(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `version 1
import "policies/common.encore"
import "policies/storage.encore"

for service {
    cpu: default 1
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Version.Num, qt.Equals, 1)
	c.Assert(f.Imports, qt.HasLen, 2)
	c.Assert(f.Imports[0].Path, qt.Equals, "policies/common.encore")
	c.Assert(f.Imports[1].Path, qt.Equals, "policies/storage.encore")
}

func TestParseSingleLineRule(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	f, err := ParseFile("p.encore", []byte(`for service { cpu: default 1 }`))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Rules, qt.HasLen, 1)
	c.Assert(f.Rules[0].Props, qt.HasLen, 1)
}

func TestParseNegativeNumbers(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	f, err := ParseFile("p.encore", []byte("for service {\n    offset: >= -2 | default -1\n}\n"))
	c.Assert(err, qt.IsNil)
	sv := f.Rules[0].Props[0].scalar()
	cmp := sv.Constraint.(*Comparison)
	c.Assert(cmp.Value, qt.Equals, Number(-2))
	c.Assert(sv.Default.Value, qt.Equals, Number(-1))
}

func TestParseErrors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		name string
		src  string
		want []string // substrings of the error output
	}{
		{
			name: "single equals in selector",
			src:  "for service if env.type = production {\n}\n",
			want: []string{"'=' is not an operator", "use '==' for equality comparisons"},
		},
		{
			name: "logical or in selector",
			src:  "for service if a == 1 || b == 2 {\n}\n",
			want: []string{"'||' is not supported in selectors", "split the rule into separate rules"},
		},
		{
			name: "single amp in selector",
			src:  "for service if a == 1 & b == 2 {\n}\n",
			want: []string{"selector conditions are combined with '&&', not '&'"},
		},
		{
			name: "double amp in constraint",
			src:  "for service {\n    cpu: >= 1 && <= 4\n}\n",
			want: []string{"property constraints are combined with '&', not '&&'"},
		},
		{
			name: "default not last",
			src:  "for service {\n    cpu: default 2 | <= 4\n}\n",
			want: []string{"'default' must be the last clause in a property rule"},
		},
		{
			name: "default after amp",
			src:  "for service {\n    cpu: >= 1 & default 2\n}\n",
			want: []string{"'default' must be separated from constraints with '|'"},
		},
		{
			name: "missing colon",
			src:  "for service {\n    cpu >= 1\n}\n",
			want: []string{"expected ':' after property path 'cpu'"},
		},
		{
			name: "unknown unit",
			src:  "for service {\n    memory: >= 512G\n}\n",
			want: []string{"unknown unit 'G' in '512G'", "did you mean '512GB'?"},
		},
		{
			name: "ordering comparison on string",
			src:  "for service {\n    region: >= \"a\"\n}\n",
			want: []string{"ordering comparison '>=' requires a number, size, or duration", "string"},
		},
		{
			name: "ordering comparison on bool",
			src:  "for service {\n    flag: < true\n}\n",
			want: []string{"ordering comparison '<' requires a number, size, or duration", "bool"},
		},
		{
			name: "missing rule body",
			src:  "for service\n",
			want: []string{"expected '{' to begin the rule body, found newline"},
		},
		{
			name: "unclosed rule body",
			src:  "for service {\n    cpu: default 1\n",
			want: []string{"expected '}' to close the rule body, found end of file"},
		},
		{
			name: "name before kind",
			src:  "for \"api\" {\n}\n",
			want: []string{"expected a resource kind after 'for'", "the resource kind comes before the name"},
		},
		{
			name: "named block with for",
			src:  "for service \"api\" {\n}\n",
			want: []string{"a named block omits 'for'", `write: service "api" { ... }`},
		},
		{
			name: "dynamic block with for",
			src:  "for service tags.domain {\n}\n",
			want: []string{"a dynamic block omits 'for'"},
		},
		{
			name: "bare block needs name",
			src:  "sql_cluster {\n}\n",
			want: []string{"a resource block needs a name or expression"},
		},
		{
			name: "unsupported version",
			src:  "version 2\nfor service {\n}\n",
			want: []string{"unsupported language version 2", "this parser supports version 1"},
		},
		{
			name: "version not first",
			src:  "for service {\n}\nversion 1\n",
			want: []string{"the version declaration must be the first statement"},
		},
		{
			name: "import after rule",
			src:  "for service {\n}\nimport \"x.encore\"\n",
			want: []string{"import declarations must appear before the first rule"},
		},
		{
			name: "keyword as selector field",
			src:  "for service if default == 1 {\n}\n",
			want: []string{"'default' is a reserved keyword and cannot be used as a field name"},
		},
		{
			name: "keyword as value",
			src:  "for service if env.type == if {\n}\n",
			want: []string{"'if' is a reserved keyword and cannot be used as a value"},
		},
		{
			name: "missing selector operator",
			src:  "for service if env.type {\n}\n",
			want: []string{"incomplete selector condition: 'env.type' needs an operator such as '==' or 'exists'"},
		},
		{
			name: "stray top-level token",
			src:  "\"oops\"\n",
			want: []string{"expected 'for', a resource block, or 'if' to begin a declaration", `string "oops"`},
		},
		{
			name: "empty membership list",
			src:  "for service if env.type in [] {\n}\n",
			want: []string{"expected a value in the membership list"},
		},
		{
			name: "two constraints on one line",
			src:  "for service {\n    cpu: >= 1 <= 4\n}\n",
			want: []string{"expected newline after the property rule for 'cpu'"},
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			_, err := ParseFile("policy.encore", []byte(tt.src))
			assertErrContains(c, err, tt.want...)
		})
	}
}

func TestParseErrorRecovery(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Errors in one rule do not prevent later rules from being parsed
	// and checked: all three problems are reported.
	src := `for service {
    cpu: default 2 | <= 4
}
for bucket {
    versioning = true
}
for sql_database {
    backup_retention: >= 30x
}
`
	f, err := ParseFile("policy.encore", []byte(src))
	c.Assert(err, qt.IsNotNil)
	errs := err.(ErrorList)
	c.Assert(errs, qt.HasLen, 3)
	c.Assert(errs[0].Message, qt.Contains, "'default' must be the last clause")
	c.Assert(errs[1].Message, qt.Contains, "property rules use ':', not '='")
	c.Assert(errs[2].Message, qt.Contains, "unknown unit 'x'")
	c.Assert(f.Rules, qt.HasLen, 3)
}

func TestParseTooManyErrors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := ""
	for range 30 {
		src += "for service {\n    cpu = 1\n}\n"
	}
	_, err := ParseFile("policy.encore", []byte(src))
	c.Assert(err, qt.IsNotNil)
	errs := err.(ErrorList)
	c.Assert(len(errs) <= maxParseErrors+1, qt.IsTrue)
	c.Assert(err.Error(), qt.Contains, "too many errors")
}

func TestParseErrorPositions(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("policy.encore", []byte("for service if env.type = \"production\" {\n}\n"))
	errs := err.(ErrorList)
	c.Assert(errs, qt.HasLen, 1)
	c.Assert(errs[0].Pos, qt.DeepEquals, Position{File: "policy.encore", Offset: 24, Line: 1, Column: 25})
}

func TestParseRequiredInsideOrAlternative(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// `required` inside a '|' alternative parses, but Validate rejects it.
	rs := parseSet(c, "for service {\n    cpu: required | >= 2\n}\n")
	assertErrContains(c, rs.Validate(),
		"'required' cannot be part of a '|' alternative",
		"combine it with '&' instead")
}
