package eclmeta

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/ecl"
	metav2 "encr.dev/proto/encore/parser/meta/v2"
)

func ruleSet(c *qt.C, src string) *ecl.RuleSet {
	c.Helper()
	f, err := ecl.ParseFile("policy.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	return ecl.NewRuleSet(f)
}

func TestResources(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	md := &metav2.Resources{
		Services:     map[string]*metav2.Service{"api": {Name: "api"}},
		SqlDatabases: map[string]*metav2.SQLDatabase{"orders": {Name: "orders"}},
		Buckets:      map[string]*metav2.Bucket{"uploads": {Name: "uploads", Public: true, Versioned: false}},
	}
	rs := Resources(md, nil)
	c.Assert(rs, qt.HasLen, 3)

	var bucket *ecl.Resource
	for _, r := range rs {
		if r.Kind == "bucket" {
			bucket = r
		}
	}
	c.Assert(bucket, qt.IsNotNil)
	c.Assert(bucket.Name, qt.Equals, "uploads")
	// Declared config from metadata is folded in.
	c.Assert(bucket.Config["public_access"], qt.Equals, ecl.Bool(true))
	c.Assert(bucket.Config["versioning"], qt.Equals, ecl.Bool(false))
}

func TestEvaluate(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	policy := ruleSet(c, `
for service {
    cpu: >= 1 & <= 4 | default 1
}
for bucket {
    public_access: false
    versioning: true
}
`)

	// An app whose declared resources comply with the policy.
	good := &metav2.Resources{
		Services: map[string]*metav2.Service{"api": {Name: "api"}},
		Buckets:  map[string]*metav2.Bucket{"uploads": {Name: "uploads", Public: false, Versioned: true}},
	}
	er, err := Evaluate(policy, nil, good, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(er.Results, qt.HasLen, 2)
	c.Assert(er.Get("service", "api").Properties["cpu"].Value, qt.Equals, ecl.Number(1))
	c.Assert(er.Get("bucket", "uploads").Properties["public_access"].Value, qt.Equals, ecl.Bool(false))

	// A bucket the app declares as public violates `public_access: false`.
	bad := &metav2.Resources{
		Buckets: map[string]*metav2.Bucket{"leaky": {Name: "leaky", Public: true}},
	}
	_, err = Evaluate(policy, nil, bad, nil)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, `bucket "leaky": property 'public_access' value true violates constraint 'false'`)
}

func TestEvaluateEnrich(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// The policy selects by team, which the metadata doesn't carry; the enrich
	// hook supplies it.
	policy := ruleSet(c, `
for service if team == "payments" {
    cpu: default 4
}
for service {
    cpu: default 1
}
`)
	md := &metav2.Resources{Services: map[string]*metav2.Service{
		"api":    {Name: "api"},
		"worker": {Name: "worker"},
	}}
	enrich := func(kind, name string, r *ecl.Resource) {
		if kind == "service" && name == "api" {
			r.Attrs = map[string]ecl.Value{"team": ecl.String("payments")}
		}
	}
	er, err := Evaluate(policy, nil, md, enrich)
	c.Assert(err, qt.IsNil)
	c.Assert(er.Get("service", "api").Properties["cpu"].Value, qt.Equals, ecl.Number(4))
	c.Assert(er.Get("service", "worker").Properties["cpu"].Value, qt.Equals, ecl.Number(1))
}
