package ecl

import (
	qt "github.com/frankban/quicktest"
)

// parseSet parses src as a single file and asserts it has no errors.
func parseSet(c *qt.C, src string) *RuleSet {
	c.Helper()
	f, err := ParseFile("policy.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	return NewRuleSet(f)
}

// strAttrs builds an attribute map of string values from key/value pairs.
func strAttrs(pairs ...string) map[string]Value {
	m := make(map[string]Value)
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = String(pairs[i+1])
	}
	return m
}

// assertErrContains asserts that err is non-nil and its message contains
// every given substring.
func assertErrContains(c *qt.C, err error, subs ...string) {
	c.Helper()
	c.Assert(err, qt.IsNotNil)
	for _, s := range subs {
		c.Assert(err.Error(), qt.Contains, s)
	}
}

func evalOK(c *qt.C, rs *RuleSet, res *Resource) *Result {
	c.Helper()
	result, err := rs.Evaluate(res)
	c.Assert(err, qt.IsNil)
	return result
}

func evalErr(c *qt.C, rs *RuleSet, res *Resource) error {
	c.Helper()
	result, err := rs.Evaluate(res)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
	return err
}

// assertValue asserts that got equals want, ignoring display-only
// attributes such as units and quoting style.
func assertValue(c *qt.C, got, want Value) {
	c.Helper()
	c.Assert(got.Kind, qt.Equals, want.Kind)
	c.Assert(valuesEqual(got, want), qt.IsTrue,
		qt.Commentf("got %s, want %s", got, want))
}
