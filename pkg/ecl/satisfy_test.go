package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// Tests for satisfiability analysis of merged constraints when a
// property is unset.

// TestSatExactNotInAllowedSet: an exact value constraint acts as an
// implicit default, so when it conflicts with another rule's allowed-value
// set the error is reported as a default violation.
func TestSatExactNotInAllowedSet(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    region: "us-central1"
}
for service if env.type == "production" {
    region: "europe-west1" | "europe-north1"
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}),
		`default value "us-central1" for property 'region' violates constraint '"europe-west1" | "europe-north1"'`)
}

func TestSatExactVsExclusion(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    flag: true
}
for service if env.type == "production" {
    flag: != true
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}),
		"default value true for property 'flag' violates constraint '!= true'")
}

// TestSatConflictsBehindAmbiguousDefaults: when default resolution is
// ambiguous, the underlying constraint conflict (if any) is reported as
// the root cause instead of the ambiguity.
func TestSatConflictsBehindAmbiguousDefaults(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Exact value vs exclusion, with a second (ambiguous) default in play.
	rs := parseSet(c, `
for service if env.type == "production" {
    flag: true
}
for service if team == "x" {
    flag: default false
}
for service {
    flag: != true
}
`)
	err := evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "x"),
	})
	assertErrContains(c, err,
		"impossible constraints for property 'flag'",
		"it cannot both equal and not equal true")
	c.Assert(err.Error(), qt.Not(qt.Contains), "ambiguous")

	// Exact value vs allowed-value set.
	rs = parseSet(c, `
for service if env.type == "production" {
    region: "a"
}
for service if team == "x" {
    region: default "b"
}
for service {
    region: "b" | "c"
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "x"),
	}),
		"impossible constraints for property 'region'",
		`"a" is not one of the allowed values "b" | "c"`)
}

func TestSatExactOutsideBounds(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Exact value below another rule's minimum, with the exact rule
	// listed first so the conflict is found via the bound, not a default.
	rs := parseSet(c, `
for service if team == "payments" {
    cpu: >= 4
}
service "api" {
    cpu: 2
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("team", "payments"),
	}), "violates constraint '>= 4'")
}

func TestSatAllowedSetFilteredByBounds(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    tier_level: 1 | 2
}
for service if env.type == "production" {
    tier_level: >= 3
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}),
		"impossible constraints for property 'tier_level'",
		"no value satisfies all the allowed-value constraints",
		"'1 | 2' at policy.encore",
		"'>= 3' at policy.encore")
}

func TestSatAllowedSetFilteredByExclusions(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    tier: "small" | "medium"
}
for service if env.type == "production" {
    tier: != "small" & != "medium"
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}), "no value satisfies all the allowed-value constraints")
}

// TestSatDisjunctionWithRangesNotAnalyzed: disjunctions containing
// range alternatives are beyond the conservative analysis, so no
// conflict is reported for an unset property — but a concrete value is
// still checked precisely.
func TestSatDisjunctionWithRangesNotAnalyzed(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: <= 1 | >= 4
}
for service if env.type == "production" {
    cpu: >= 2 & <= 3
}
`)
	attrs := strAttrs("env.type", "production")

	// Unset: conservatively accepted even though no value exists.
	_, err := rs.Evaluate(&Resource{Kind: "service", Attrs: attrs})
	c.Assert(err, qt.IsNil)

	// A concrete value is checked against both rules.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Name: "api", Attrs: attrs,
		Config: map[string]Value{"cpu": Number(2.5)},
	}), "property 'cpu' value 2.5 violates constraint '<= 1 | >= 4'")
}

func TestSatTighterBoundWins(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// The strict bound > 2 is tighter than >= 2 at the same value;
	// combined with <= 2 it admits no value.
	rs := parseSet(c, `
for service {
    cpu: >= 2
}
for service if env.type == "production" {
    cpu: > 2
}
for service if team == "payments" {
    cpu: <= 2
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	}), "'> 2' conflicts with '<= 2'")

	// Without the strict rule, exactly 2 remains viable.
	_, err := rs.Evaluate(&Resource{
		Kind:  "service",
		Attrs: strAttrs("team", "payments"),
	})
	c.Assert(err, qt.IsNil)
}

func TestEvalRequiredAlone(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for sql_database {
    backup_retention: required
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{Kind: "sql_database", Name: "main"}),
		"property 'backup_retention' is required but not set")

	result := evalOK(c, rs, &Resource{
		Kind: "sql_database", Name: "main",
		Config: map[string]Value{"backup_retention": MustParseQuantity("7d")},
	})
	assertValue(c, result.Properties["backup_retention"].Value, MustParseQuantity("7d"))
}

// TestEvalImplicitDefaultVariants: explicit `== v` and `required & v`
// also act as defaults, while ranges and disjunctions do not.
func TestEvalImplicitDefaultVariants(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    a: == 2
    b: required & 3
    c: >= 1
    d: "small" | "large"
}
`)
	result, err := rs.Evaluate(&Resource{Kind: "service"})
	c.Assert(err, qt.IsNil)

	assertValue(c, result.Properties["a"].Value, Number(2))
	c.Assert(result.Properties["a"].Source, qt.Equals, SourceDefault)
	assertValue(c, result.Properties["b"].Value, Number(3))

	_, ok := result.Properties["c"]
	c.Assert(ok, qt.IsFalse)
	_, ok = result.Properties["d"]
	c.Assert(ok, qt.IsFalse)
}

func TestEvalNotEqualConstraint(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    region: != "us-central1"
}
`)
	// A different value passes.
	result := evalOK(c, rs, &Resource{
		Kind:   "service",
		Config: map[string]Value{"region": String("europe-west1")},
	})
	assertValue(c, result.Properties["region"].Value, String("europe-west1"))

	// The excluded value fails.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Name: "api",
		Config: map[string]Value{"region": String("us-central1")},
	}), `property 'region' value "us-central1" violates constraint '!= "us-central1"'`)

	// Unset: no default is implied by a != constraint.
	result = evalOK(c, rs, &Resource{Kind: "service"})
	_, ok := result.Properties["region"]
	c.Assert(ok, qt.IsFalse)
}
