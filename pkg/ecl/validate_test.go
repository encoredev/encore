package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestValidateUnknownKind(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, "for sevice { cpu: default 1 }")
	assertErrContains(c, rs.Validate(),
		"unknown resource kind 'sevice'",
		"did you mean 'service'?")

	// A custom schema replaces the default one.
	rs = parseSet(c, "for widget { size: default 1 }")
	rs.Schema = map[string]Kind{"widget": {}}
	c.Assert(rs.Validate(), qt.IsNil)

	// An explicitly empty schema disables the check.
	rs = parseSet(c, "for anything { size: default 1 }")
	rs.Schema = map[string]Kind{}
	c.Assert(rs.Validate(), qt.IsNil)
}

func TestValidateDefaultViolatesOwnConstraint(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: <= 4 | default 8
}
`)
	assertErrContains(c, rs.Validate(),
		"default value 8 violates the constraint '<= 4' in the same property rule",
		"for service if env.type == \"production\"")
}

func TestValidateDefaultViolatesExactConstraint(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, "for bucket {\n    public_access: false | default true\n}\n")
	assertErrContains(c, rs.Validate(),
		"default value true violates the constraint 'false'")
}

func TestValidateDuplicateProperty(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: >= 1
    cpu: <= 4
}
`)
	assertErrContains(c, rs.Validate(),
		"duplicate property rule for 'cpu' in the same rule",
		"'cpu' was first defined at policy.encore:3:5",
		"combine the constraints with '&'")
}

func TestValidateContradictorySelector(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		src  string
		want []string
	}{
		{
			src:  "for service if env.type == \"production\" && env.type == \"staging\" { cpu: >= 1 }",
			want: []string{"this rule can never match", "'env.type == \"staging\"' contradicts 'env.type == \"production\"'"},
		},
		{
			src:  "for service if team == \"a\" && team != \"a\" { cpu: >= 1 }",
			want: []string{"this rule can never match"},
		},
		{
			src:  "for service if env.type == \"preview\" && env.type in [\"production\", \"staging\"] { cpu: >= 1 }",
			want: []string{"this rule can never match"},
		},
		{
			src:  "service \"api\" if name == \"web\" { cpu: >= 1 }",
			want: []string{"this rule can never match", `'name == "web"' contradicts 'name == "api"'`},
		},
	}
	for _, tt := range tests {
		rs := parseSet(c, tt.src)
		assertErrContains(c, rs.Validate(), tt.want...)
	}
}

func TestValidateMixedTypesInProperty(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, "for service {\n    cpu: >= 1 & <= 2Gi\n}\n")
	assertErrContains(c, rs.Validate(),
		"mixed value types for property 'cpu'",
		"'>= 1' is a number, but '<= 2Gi' is a size")

	// The default's type must match the constraint's type too.
	rs = parseSet(c, "for service {\n    memory: >= 1Gi | default 2\n}\n")
	assertErrContains(c, rs.Validate(),
		"mixed value types for property 'memory'",
		"the default 2 is a number")
}

func TestValidateImpossibleSingleRule(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, "for service {\n    cpu: >= 4 & <= 2\n}\n")
	assertErrContains(c, rs.Validate(),
		"impossible constraints for property 'cpu'",
		"'>= 4' conflicts with '<= 2'")
}

// TestValidateImpossibleRulePair is the spec's "impossible range"
// example: rules with overlapping selectors whose merged constraints
// admit no value.
func TestValidateImpossibleRulePair(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: >= 4
}
for service if team == "payments" {
    cpu: <= 2
}
`)
	assertErrContains(c, rs.Validate(),
		"impossible constraints for property 'cpu'",
		"'>= 4' conflicts with '<= 2'",
		"the rules can match the same resource")
}

// TestValidateConflictingExactPair is the spec's "conflicting exact
// values" example.
func TestValidateConflictingExactPair(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for bucket if env.type == "production" {
    public_access: false
}
bucket "uploads" if env.type == "production" {
    public_access: true
}
`)
	assertErrContains(c, rs.Validate(),
		"impossible constraints for property 'public_access'",
		"'false' conflicts with 'true'")
}

// TestValidateDisjointSelectorsNotFlagged: rules whose selectors cannot
// both match are not a conflict.
func TestValidateDisjointSelectorsNotFlagged(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: >= 4
}
for service if env.type == "preview" {
    cpu: <= 2
}
`)
	c.Assert(rs.Validate(), qt.IsNil)

	// Likewise for different resource names.
	rs = parseSet(c, `
bucket "uploads" {
    public_access: true
}
bucket "internal" {
    public_access: false
}
`)
	c.Assert(rs.Validate(), qt.IsNil)
}

func TestValidateCrossRuleDefaultViolation(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: <= 4
}
service "api" {
    cpu: default 8
}
`)
	assertErrContains(c, rs.Validate(),
		"default value 8 for property 'cpu' violates the constraint '<= 4' of another rule that can match the same resource",
		`service "api"`,
		"for service")
}

func TestValidateBoolExclusionConflict(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    flag: != true
}
for service if team == "payments" {
    flag: != false
}
`)
	assertErrContains(c, rs.Validate(),
		"impossible constraints for property 'flag'",
		"a bool cannot differ from both true and false")
}

// TestValidateAmbiguousDefaultsNotStatic: ambiguous defaults depend on
// which rules match a concrete resource, so Validate does not flag them;
// Evaluate does (see TestEvalAmbiguousDefault).
func TestValidateAmbiguousDefaultsNotStatic(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: default 1
}
for service if team == "payments" {
    cpu: default 2
}
`)
	c.Assert(rs.Validate(), qt.IsNil)
}

func TestValidateCleanRuleSet(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, completeExample)
	c.Assert(rs.Validate(), qt.IsNil)
}
