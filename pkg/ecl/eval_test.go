package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// TestEvalKindWideDefault is example 1 from the language spec: a
// production service picks up the production rule's constraints and
// default.
func TestEvalKindWideDefault(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: >= 0.25 & <= 8 | default 0.5
}
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
}
`)
	result := evalOK(c, rs, &Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("env.type", "production"),
	})
	c.Assert(result.Matched, qt.HasLen, 2)
	rp := result.Properties["cpu"]
	assertValue(c, rp.Value, Number(1))
	c.Assert(rp.Source, qt.Equals, SourceDefault)
	c.Assert(rp.DefaultRule.Header(), qt.Equals, "for service if env.type == \"production\"")

	// A non-production service gets the baseline default.
	result = evalOK(c, rs, &Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("env.type", "development"),
	})
	c.Assert(result.Matched, qt.HasLen, 1)
	assertValue(c, result.Properties["cpu"].Value, Number(0.5))
}

// TestEvalNamedRuleDefault is example 2 from the spec: the named rule is
// more specific and provides the default.
func TestEvalNamedRuleDefault(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
}
service "api" if env.type == "production" {
    cpu: default 2
}
`)
	result := evalOK(c, rs, &Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("env.type", "production"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(2))

	// Another service is not matched by the named rule.
	result = evalOK(c, rs, &Resource{
		Kind: "service", Name: "worker",
		Attrs: strAttrs("env.type", "production"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestEvalAmbiguousDefault is example 3 from the spec: two unrelated
// selectors providing different defaults is an error.
func TestEvalAmbiguousDefault(t *testing.T) {
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
	err := evalErr(c, rs, &Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	})
	assertErrContains(c, err,
		`ambiguous default for property 'cpu' of service "api"`,
		"matching rules provide different defaults:",
		"policy.encore:2:1: for service if env.type == \"production\"",
		"cpu: default 1",
		`policy.encore:5:1: for service if team == "payments"`,
		"cpu: default 2",
		"no rule is more specific than all the others",
		`for service if env.type == "production" && team == "payments"`,
	)

	// A resource matching only one of the rules is fine.
	result := evalOK(c, rs, &Resource{
		Kind: "service", Name: "api",
		Attrs: strAttrs("env.type", "production", "team", "platform"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestEvalSameDefaultNotAmbiguous: multiple rules providing the same
// default value never conflict.
func TestEvalSameDefaultNotAmbiguous(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: default 1
}
for service if team == "payments" {
    cpu: default 1
}
`)
	result := evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestEvalConstraintsMergeByIntersection is example 4 from the spec:
// constraints from unrelated selectors all apply.
func TestEvalConstraintsMergeByIntersection(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: <= 4
}
for service if team == "payments" {
    cpu: >= 2
}
`)
	res := func(cpu float64) *Resource {
		return &Resource{
			Kind:   "service",
			Name:   "api",
			Attrs:  strAttrs("env.type", "production", "team", "payments"),
			Config: map[string]Value{"cpu": Number(cpu)},
		}
	}

	// Within [2, 4]: accepted.
	result := evalOK(c, rs, res(3))
	c.Assert(result.Properties["cpu"].Source, qt.Equals, SourceExplicit)

	// Below the payments minimum.
	assertErrContains(c, evalErr(c, rs, res(1)),
		`service "api": property 'cpu' value 1 violates constraint '>= 2'`,
		`for service if team == "payments"`)

	// Above the production maximum.
	assertErrContains(c, evalErr(c, rs, res(8)),
		`service "api": property 'cpu' value 8 violates constraint '<= 4'`,
		`for service if env.type == "production"`)

	// Unset with no default: allowed, since cpu is not required.
	result = evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	})
	_, ok := result.Properties["cpu"]
	c.Assert(ok, qt.IsFalse)
}

func TestEvalExplicitBeatsDefaultButNotConstraints(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
}
`)
	attrs := strAttrs("env.type", "production")

	// Explicit value within range wins over the default.
	result := evalOK(c, rs, &Resource{
		Kind: "service", Attrs: attrs,
		Config: map[string]Value{"cpu": Number(3)},
	})
	rp := result.Properties["cpu"]
	assertValue(c, rp.Value, Number(3))
	c.Assert(rp.Source, qt.Equals, SourceExplicit)
	c.Assert(rp.DefaultRule, qt.IsNil)

	// Explicit value outside the range is rejected.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Attrs: attrs,
		Config: map[string]Value{"cpu": Number(8)},
	}), "property 'cpu' value 8 violates constraint '<= 4'")

	// Unset: the default applies.
	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: attrs})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestEvalExactValueActsAsDefault: `public_access: false` both
// constrains the value and defaults it when unset.
func TestEvalExactValueActsAsDefault(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for bucket {
    public_access: false
    versioning: true
}
`)
	// Unset: the exact values become defaults.
	result := evalOK(c, rs, &Resource{Kind: "bucket", Name: "uploads"})
	rp := result.Properties["public_access"]
	assertValue(c, rp.Value, Bool(false))
	c.Assert(rp.Source, qt.Equals, SourceDefault)
	assertValue(c, result.Properties["versioning"].Value, Bool(true))

	// Explicitly matching the exact value: fine.
	result = evalOK(c, rs, &Resource{
		Kind: "bucket", Name: "uploads",
		Config: map[string]Value{"public_access": Bool(false)},
	})
	c.Assert(result.Properties["public_access"].Source, qt.Equals, SourceExplicit)

	// Explicitly contradicting it: rejected.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "bucket", Name: "uploads",
		Config: map[string]Value{"public_access": Bool(true)},
	}), `bucket "uploads": property 'public_access' value true violates constraint 'false'`)
}

// TestEvalConflictingExactValues: two matching rules requiring different
// exact values is an impossible constraint set, reported as such rather
// than as an ambiguous default.
func TestEvalConflictingExactValues(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for bucket if env.type == "production" {
    public_access: false
}
bucket "uploads" {
    public_access: true
}
`)
	err := evalErr(c, rs, &Resource{
		Kind: "bucket", Name: "uploads",
		Attrs: strAttrs("env.type", "production"),
	})
	assertErrContains(c, err,
		`impossible constraints for property 'public_access' of bucket "uploads"`,
		"'false' conflicts with 'true'",
		"it cannot equal both false and true",
		"for bucket if env.type == \"production\"",
		`bucket "uploads"`,
	)
	// The root cause is reported, not a default ambiguity.
	c.Assert(err.Error(), qt.Not(qt.Contains), "ambiguous")
}

// TestEvalExactPlusRangeConflict: an exact value acting as a default
// must still satisfy other matching rules' constraints.
func TestEvalExactPlusRangeConflict(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: <= 4
}
service "api" {
    cpu: 8
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{Kind: "service", Name: "api"}),
		`service "api": default value 8 for property 'cpu' violates constraint '<= 4'`,
		"the default is defined at policy.encore:6:10 in rule: service \"api\"")

	// Other services are unaffected.
	result := evalOK(c, rs, &Resource{Kind: "service", Name: "worker"})
	_, ok := result.Properties["cpu"]
	c.Assert(ok, qt.IsFalse)
}

func TestEvalDefaultMustSatisfyOtherRulesConstraints(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: >= 2
}
for service if team == "payments" {
    cpu: default 1
}
`)
	// The payments default of 1 violates the production minimum.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	}),
		"default value 1 for property 'cpu' violates constraint '>= 2'",
		"the default is defined at",
	)

	// Outside production the default is fine.
	result := evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "development", "team", "payments"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestEvalSpecificityChain: the most specific of a chain of matching
// rules provides the default.
func TestEvalSpecificityChain(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: default 0.5
}
for service if env.type == "production" {
    cpu: default 1
}
for service if env.type == "production" && team == "payments" {
    cpu: default 2
}
service "api" if env.type == "production" && team == "payments" {
    cpu: default 3
}
`)
	attrs := strAttrs("env.type", "production", "team", "payments")

	result := evalOK(c, rs, &Resource{Kind: "service", Name: "api", Attrs: attrs})
	assertValue(c, result.Properties["cpu"].Value, Number(3))

	result = evalOK(c, rs, &Resource{Kind: "service", Name: "worker", Attrs: attrs})
	assertValue(c, result.Properties["cpu"].Value, Number(2))

	result = evalOK(c, rs, &Resource{
		Kind: "service", Name: "worker",
		Attrs: strAttrs("env.type", "production", "team", "platform"),
	})
	assertValue(c, result.Properties["cpu"].Value, Number(1))

	result = evalOK(c, rs, &Resource{Kind: "service", Name: "worker"})
	assertValue(c, result.Properties["cpu"].Value, Number(0.5))
}

// TestEvalMembershipSpecificity: `==` is more specific than `in`
// containing the same value.
func TestEvalMembershipSpecificity(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if team in ["payments", "billing"] {
    cpu: default 1
}
for service if team == "payments" {
    cpu: default 2
}
`)
	result := evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("team", "payments")})
	assertValue(c, result.Properties["cpu"].Value, Number(2))

	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("team", "billing")})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
}

// TestEvalNamedRuleEqualsNameSelector: `for service "api"` and
// `for service where name == "api"` are the same selector, so different
// defaults from the two forms are ambiguous.
func TestEvalNamedRuleEqualsNameSelector(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
service "api" {
    cpu: default 1
}
for service if name == "api" {
    cpu: default 2
}
`)
	res := &Resource{Kind: "service", Name: "api"}
	assertErrContains(c, evalErr(c, rs, res), "ambiguous default for property 'cpu'")
}

func TestEvalRequired(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for sql_database if env.type == "production" {
    backup_retention: required & >= 30d
}
`)
	attrs := strAttrs("env.type", "production")

	// Missing with no default: an error.
	assertErrContains(c, evalErr(c, rs, &Resource{Kind: "sql_database", Name: "main", Attrs: attrs}),
		`sql_database "main": property 'backup_retention' is required but not set`,
		"set 'backup_retention' on the resource, or add 'default <value>' to a matching rule")

	// Explicitly set: satisfied.
	result := evalOK(c, rs, &Resource{
		Kind: "sql_database", Name: "main", Attrs: attrs,
		Config: map[string]Value{"backup_retention": MustParseQuantity("45d")},
	})
	assertValue(c, result.Properties["backup_retention"].Value, MustParseQuantity("45d"))

	// Set but violating the range: rejected.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "sql_database", Name: "main", Attrs: attrs,
		Config: map[string]Value{"backup_retention": MustParseQuantity("7d")},
	}), "property 'backup_retention' value 7d violates constraint '>= 30d'")
}

// TestEvalRequiredSatisfiedByDefault: a default from another matching
// rule satisfies a required constraint.
func TestEvalRequiredSatisfiedByDefault(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for sql_database if env.type == "production" {
    backup_retention: required & >= 30d
}
for sql_database if env.type == "production" {
    backup_retention: default 30d
}
`)
	result := evalOK(c, rs, &Resource{
		Kind: "sql_database", Name: "main",
		Attrs: strAttrs("env.type", "production"),
	})
	rp := result.Properties["backup_retention"]
	assertValue(c, rp.Value, MustParseQuantity("30d"))
	c.Assert(rp.Source, qt.Equals, SourceDefault)
}

// TestEvalDisjunctionsIntersect: allowed-value sets from multiple rules
// all apply, narrowing the allowed values.
func TestEvalDisjunctionsIntersect(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    region: "europe-west1" | "europe-north1" | "us-central1"
}
for service if env.type == "production" {
    region: "europe-west1" | "europe-north1"
}
`)
	attrs := strAttrs("env.type", "production")

	// In the intersection: accepted.
	result := evalOK(c, rs, &Resource{
		Kind: "service", Attrs: attrs,
		Config: map[string]Value{"region": String("europe-west1")},
	})
	assertValue(c, result.Properties["region"].Value, String("europe-west1"))

	// Allowed by the baseline but not in production: rejected.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Name: "api", Attrs: attrs,
		Config: map[string]Value{"region": String("us-central1")},
	}), `property 'region' value "us-central1" violates constraint '"europe-west1" | "europe-north1"'`)
}

func TestEvalEmptyDisjunctionIntersection(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    region: "a" | "b"
}
for service if env.type == "production" {
    region: "c" | "d"
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}),
		"impossible constraints for property 'region'",
		"no value satisfies all the allowed-value constraints",
		`'"a" | "b"' at policy.encore`,
		`'"c" | "d"' at policy.encore`,
	)
}

// TestEvalImpossibleRange: merged bounds with no possible value are
// rejected even when the property is unset.
func TestEvalImpossibleRange(t *testing.T) {
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
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "team", "payments"),
	}),
		"impossible constraints for property 'cpu'",
		"'>= 4' conflicts with '<= 2'",
		"a rule cannot weaken another rule's constraints",
	)

	// Matching only one rule is fine.
	result := evalOK(c, rs, &Resource{
		Kind:   "service",
		Attrs:  strAttrs("env.type", "production"),
		Config: map[string]Value{"cpu": Number(4)},
	})
	assertValue(c, result.Properties["cpu"].Value, Number(4))
}

func TestEvalStrictBounds(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// > 2 and <= 2 admit no value.
	rs := parseSet(c, `
for service {
    cpu: > 2
}
for service if env.type == "production" {
    cpu: <= 2
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}), "'> 2' conflicts with '<= 2'")

	// >= 2 and <= 2 admit exactly 2.
	rs = parseSet(c, `
for service {
    cpu: >= 2
}
for service if env.type == "production" {
    cpu: <= 2
}
`)
	result := evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	})
	_, ok := result.Properties["cpu"]
	c.Assert(ok, qt.IsFalse)
}

func TestEvalSizesAndDurations(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    memory: >= 1Gi & <= 8Gi | default 2Gi
}
`)
	attrs := strAttrs("env.type", "production")

	// The default applies and keeps its unit.
	result := evalOK(c, rs, &Resource{Kind: "service", Attrs: attrs})
	rp := result.Properties["memory"]
	assertValue(c, rp.Value, MustParseQuantity("2Gi"))
	c.Assert(rp.Value.String(), qt.Equals, "2Gi")

	// Unit conversions are respected: 2048Mi == 2Gi.
	result = evalOK(c, rs, &Resource{
		Kind: "service", Attrs: attrs,
		Config: map[string]Value{"memory": MustParseQuantity("2048Mi")},
	})
	assertValue(c, result.Properties["memory"].Value, MustParseQuantity("2Gi"))

	// Below the minimum.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Name: "api", Attrs: attrs,
		Config: map[string]Value{"memory": MustParseQuantity("512Mi")},
	}), "property 'memory' value 512Mi violates constraint '>= 1Gi'")
}

func TestEvalSelectorOperators(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for bucket if tags.data exists {
    backup_retention: default 7d
}
for service if env.type != "preview" {
    cpu: default 1
}
for service if env.type in ["production", "staging"] {
    memory: default 1Gi
}
`)

	// exists matches any value for the attribute.
	result := evalOK(c, rs, &Resource{
		Kind:  "bucket",
		Attrs: strAttrs("tags.data", "customer"),
	})
	assertValue(c, result.Properties["backup_retention"].Value, MustParseQuantity("7d"))

	// exists does not match when the attribute is missing.
	result = evalOK(c, rs, &Resource{Kind: "bucket"})
	c.Assert(result.Matched, qt.HasLen, 0)

	// != requires the attribute to exist and differ.
	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("env.type", "production")})
	assertValue(c, result.Properties["cpu"].Value, Number(1))
	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("env.type", "preview")})
	_, ok := result.Properties["cpu"]
	c.Assert(ok, qt.IsFalse)
	result = evalOK(c, rs, &Resource{Kind: "service"})
	c.Assert(result.Matched, qt.HasLen, 0)

	// in matches membership.
	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("env.type", "staging")})
	_, ok = result.Properties["memory"]
	c.Assert(ok, qt.IsTrue)
	result = evalOK(c, rs, &Resource{Kind: "service", Attrs: strAttrs("env.type", "preview")})
	_, ok = result.Properties["memory"]
	c.Assert(ok, qt.IsFalse)
}

func TestEvalSelectorTypeMismatch(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if replicas == 3 {
    cpu: default 1
}
`)
	err := evalErr(c, rs, &Resource{
		Kind:  "service",
		Name:  "api",
		Attrs: map[string]Value{"replicas": String("three")},
	})
	assertErrContains(c, err,
		"type mismatch in selector condition 'replicas == 3'",
		`attribute 'replicas' of service "api" is the string "three", not a number`)
}

func TestEvalPropertyTypeMismatch(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: >= 1
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "service", Name: "api",
		Config: map[string]Value{"cpu": String("high")},
	}), `service "api": property 'cpu' has string value "high", but the constraint '>= 1' expects a number`)
}

func TestEvalConflictingConstraintTypesAcrossRules(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    limit: >= 1
}
for service if env.type == "production" {
    limit: >= 1Gi
}
`)
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production"),
	}),
		"conflicting types for property 'limit'",
		"'>= 1' is a number constraint, but '>= 1Gi' compares against a size")
}

func TestEvalUnrelatedConfigPassesThrough(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service {
    cpu: <= 4
}
`)
	result := evalOK(c, rs, &Resource{
		Kind: "service",
		Config: map[string]Value{
			"custom.setting": String("anything"),
		},
	})
	rp := result.Properties["custom.setting"]
	assertValue(c, rp.Value, String("anything"))
	c.Assert(rp.Source, qt.Equals, SourceExplicit)
}

func TestEvalNoMatchingRules(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for bucket {
    public_access: false
}
`)
	result := evalOK(c, rs, &Resource{Kind: "service", Name: "api"})
	c.Assert(result.Matched, qt.HasLen, 0)
	c.Assert(result.Properties, qt.HasLen, 0)
}

func TestEvalProviderScopedProperties(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" && provider == "gcp" && implementation == "cloud_run" {
    provider.gcp.cloud_run.cpu_always_allocated: true
    provider.gcp.cloud_run.min_instances: >= 1 | default 1
}
`)
	// Matching provider/implementation: properties apply.
	result := evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "provider", "gcp", "implementation", "cloud_run"),
	})
	assertValue(c, result.Properties["provider.gcp.cloud_run.cpu_always_allocated"].Value, Bool(true))
	assertValue(c, result.Properties["provider.gcp.cloud_run.min_instances"].Value, Number(1))

	// Different implementation: rule does not apply.
	result = evalOK(c, rs, &Resource{
		Kind:  "service",
		Attrs: strAttrs("env.type", "production", "provider", "gcp", "implementation", "gke"),
	})
	c.Assert(result.Matched, qt.HasLen, 0)
}

func TestEvalEmptyResourceKind(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, "for service { cpu: default 1 }")
	_, err := rs.Evaluate(&Resource{})
	assertErrContains(c, err, "resource kind must not be empty")
}

// completeExample is the spec's recommended complete example, with the
// API rule additionally scoped to the payments team so its default is
// strictly more specific than the payments-wide default.
const completeExample = `
version 1

// Baseline service limits for all environments.
for service {
    cpu: >= 0.25 & <= 8 | default 0.5
    memory: >= 256Mi & <= 16Gi | default 512Mi
}

// Production services get safer defaults and tighter limits.
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
    memory: >= 1Gi & <= 8Gi | default 1Gi
    instances.min: >= 1 | default 1
    instances.max: <= 20
}

// Payments production services get larger defaults.
for service if env.type == "production" && team == "payments" {
    cpu: default 2
    memory: default 2Gi
}

// The API service needs a larger production default.
service "api" if env.type == "production" && team == "payments" {
    cpu: default 3
}

// Cloud Run-specific production behavior.
for service if env.type == "production" && provider == "gcp" && implementation == "cloud_run" {
    provider.gcp.cloud_run.cpu_always_allocated: true
    provider.gcp.cloud_run.min_instances: >= 1 | default 1
}

// Buckets should not be public by default.
for bucket {
    public_access: false
    versioning: true
}

// Customer data buckets need retention.
for bucket if tags.data == "customer" {
    backup_retention: >= 30d | default 30d
}

// Production SQL databases require stronger data protection.
for sql_database if env.type == "production" {
    backup_retention: >= 30d | default 30d
    point_in_time_recovery: true
    deletion_protection: true
}

// Main production database gets longer retention.
sql_database "main" if env.type == "production" {
    backup_retention: >= 90d | default 90d
}
`

func TestEvalCompleteExample(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, completeExample)
	c.Assert(rs.Validate(), qt.IsNil)

	// A production payments API service on Cloud Run.
	result := evalOK(c, rs, &Resource{
		Kind: "service",
		Name: "api",
		Attrs: strAttrs(
			"env.type", "production",
			"team", "payments",
			"provider", "gcp",
			"implementation", "cloud_run",
		),
		Config: map[string]Value{"instances.max": Number(10)},
	})
	c.Assert(result.Matched, qt.HasLen, 5)

	get := func(path string) Value { return result.Properties[path].Value }
	assertValue(c, get("cpu"), Number(3))                   // named rule
	assertValue(c, get("memory"), MustParseQuantity("2Gi")) // payments rule
	assertValue(c, get("instances.min"), Number(1))         // production rule
	assertValue(c, get("instances.max"), Number(10))        // explicit
	assertValue(c, get("provider.gcp.cloud_run.cpu_always_allocated"), Bool(true))
	assertValue(c, get("provider.gcp.cloud_run.min_instances"), Number(1))

	// The main production database gets the longer retention default.
	result = evalOK(c, rs, &Resource{
		Kind: "sql_database", Name: "main",
		Attrs: strAttrs("env.type", "production"),
	})
	assertValue(c, result.Properties["backup_retention"].Value, MustParseQuantity("90d"))
	assertValue(c, result.Properties["point_in_time_recovery"].Value, Bool(true))
	assertValue(c, result.Properties["deletion_protection"].Value, Bool(true))

	// Explicitly configuring less than the named rule's minimum fails.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "sql_database", Name: "main",
		Attrs:  strAttrs("env.type", "production"),
		Config: map[string]Value{"backup_retention": MustParseQuantity("60d")},
	}), "property 'backup_retention' value 60d violates constraint '>= 90d'")

	// A customer-data bucket picks up retention plus the baseline rules.
	result = evalOK(c, rs, &Resource{
		Kind: "bucket", Name: "uploads",
		Attrs: strAttrs("tags.data", "customer"),
	})
	assertValue(c, result.Properties["public_access"].Value, Bool(false))
	assertValue(c, result.Properties["versioning"].Value, Bool(true))
	assertValue(c, result.Properties["backup_retention"].Value, MustParseQuantity("30d"))
}
