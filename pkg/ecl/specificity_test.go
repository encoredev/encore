package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func condEq(f, v string) normCond {
	return normCond{field: f, op: CondEq, values: []Value{String(v)}}
}

func condNeq(f, v string) normCond {
	return normCond{field: f, op: CondNeq, values: []Value{String(v)}}
}

func condIn(f string, vs ...string) normCond {
	values := make([]Value, len(vs))
	for i, v := range vs {
		values[i] = String(v)
	}
	return normCond{field: f, op: CondIn, values: values}
}

func condExists(f string) normCond {
	return normCond{field: f, op: CondExists}
}

func TestImplies(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		name string
		a, b []normCond
		want bool
	}{
		{"anything implies empty", []normCond{condEq("t", "x")}, nil, true},
		{"empty does not imply condition", nil, []normCond{condEq("t", "x")}, false},
		{"eq implies same eq", []normCond{condEq("t", "x")}, []normCond{condEq("t", "x")}, true},
		{"eq does not imply different eq", []normCond{condEq("t", "x")}, []normCond{condEq("t", "y")}, false},
		{"eq implies exists", []normCond{condEq("t", "x")}, []normCond{condExists("t")}, true},
		{"neq implies exists", []normCond{condNeq("t", "x")}, []normCond{condExists("t")}, true},
		{"in implies exists", []normCond{condIn("t", "x")}, []normCond{condExists("t")}, true},
		{"exists does not imply eq", []normCond{condExists("t")}, []normCond{condEq("t", "x")}, false},
		{"eq implies membership", []normCond{condEq("t", "x")}, []normCond{condIn("t", "x", "y")}, true},
		{"eq does not imply non-membership", []normCond{condEq("t", "z")}, []normCond{condIn("t", "x", "y")}, false},
		{"singleton in implies eq", []normCond{condIn("t", "x")}, []normCond{condEq("t", "x")}, true},
		{"subset implies superset", []normCond{condIn("t", "x", "y")}, []normCond{condIn("t", "x", "y", "z")}, true},
		{"non-subset does not imply", []normCond{condIn("t", "x", "w")}, []normCond{condIn("t", "x", "y", "z")}, false},
		{"eq implies neq of other value", []normCond{condEq("t", "x")}, []normCond{condNeq("t", "y")}, true},
		{"eq does not imply neq of same value", []normCond{condEq("t", "x")}, []normCond{condNeq("t", "x")}, false},
		{"in implies neq of excluded value", []normCond{condIn("t", "x", "y")}, []normCond{condNeq("t", "z")}, true},
		{"in does not imply neq of member", []normCond{condIn("t", "x", "y")}, []normCond{condNeq("t", "x")}, false},
		{"neq implies same neq", []normCond{condNeq("t", "x")}, []normCond{condNeq("t", "x")}, true},
		{"different fields are unrelated", []normCond{condEq("a", "x")}, []normCond{condEq("b", "x")}, false},
		{
			"conjunction implies each part",
			[]normCond{condEq("a", "x"), condEq("b", "y")},
			[]normCond{condEq("a", "x")},
			true,
		},
		{
			"part does not imply conjunction",
			[]normCond{condEq("a", "x")},
			[]normCond{condEq("a", "x"), condEq("b", "y")},
			false,
		},
	}
	for _, tt := range tests {
		c.Assert(implies(tt.a, tt.b), qt.Equals, tt.want, qt.Commentf("%s", tt.name))
	}
}

func TestContradicts(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		name string
		a, b normCond
		want bool
	}{
		{"different eq values", condEq("t", "x"), condEq("t", "y"), true},
		{"same eq values", condEq("t", "x"), condEq("t", "x"), false},
		{"eq vs neq of same value", condEq("t", "x"), condNeq("t", "x"), true},
		{"eq vs neq of other value", condEq("t", "x"), condNeq("t", "y"), false},
		{"eq vs set without value", condEq("t", "x"), condIn("t", "y", "z"), true},
		{"eq vs set with value", condEq("t", "x"), condIn("t", "x", "y"), false},
		{"disjoint sets", condIn("t", "a", "b"), condIn("t", "c", "d"), true},
		{"overlapping sets", condIn("t", "a", "b"), condIn("t", "b", "c"), false},
		{"neq vs singleton set of same value", condNeq("t", "x"), condIn("t", "x"), true},
		{"neq vs larger set", condNeq("t", "x"), condIn("t", "x", "y"), false},
		{"exists never contradicts", condExists("t"), condEq("t", "x"), false},
		{"different fields never contradict", condEq("a", "x"), condEq("b", "y"), false},
	}
	for _, tt := range tests {
		c.Assert(contradicts(tt.a, tt.b), qt.Equals, tt.want, qt.Commentf("%s", tt.name))
		c.Assert(contradicts(tt.b, tt.a), qt.Equals, tt.want, qt.Commentf("%s (reversed)", tt.name))
	}
}

// TestEvalSubsetMembershipSpecificity: a rule with a subset membership
// selector is more specific than one with a superset.
func TestEvalSubsetMembershipSpecificity(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if team in ["a", "b", "c"] {
    cpu: default 1
}
for service if team in ["a", "b"] {
    cpu: default 2
}
`)
	result := evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("team", "a")})
	assertValue(c, result.Properties["cpu"].Value, Number(2))

	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("team", "c")})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestValidateDisjointMembershipSelectors: rules with disjoint membership
// selectors cannot co-match and are not flagged as conflicting.
func TestValidateDisjointMembershipSelectors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type in ["production", "staging"] {
    cpu: >= 4
}
for service if env.type in ["preview", "development"] {
    cpu: <= 2
}
`)
	c.Assert(rs.Validate(), qt.IsNil)
}
