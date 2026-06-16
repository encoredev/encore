package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParseNamedManagedBlock(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
sql_cluster "main" if env.type == "production" {
    engine: "postgres"
    version: "16"
    cpu: >= 2 & <= 16 | default 4
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	c.Assert(f.Rules, qt.HasLen, 1)
	r := f.Rules[0]
	c.Assert(r.Kind, qt.Equals, "sql_cluster")
	c.Assert(r.Name, qt.Equals, "main")
	c.Assert(r.Header(), qt.Equals, `sql_cluster "main" if env.type == "production"`)
	c.Assert(r.Props, qt.HasLen, 3)
}

func TestParseDefineRemoved(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("p.encore", []byte("define sql_cluster \"main\" {\n    engine: \"postgres\"\n}\n"))
	assertErrContains(c, err,
		"the 'define' keyword has been removed",
		`declare managed resources directly, e.g.: sql_cluster "main" { ... }`)
}

func TestParseObjectConstraint(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
for sql_database if tags.data == "customer" {
    cluster: {
        backup_retention: >= 30d
        point_in_time_recovery: true
    }
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)
	r := f.Rules[0]
	c.Assert(r.Props, qt.HasLen, 1)
	rv := r.Props[0].ref()
	c.Assert(rv, qt.IsNotNil)
	c.Assert(rv.Ref, qt.IsNil)
	c.Assert(rv.Object, qt.IsNotNil)
	c.Assert(rv.Object.Props, qt.HasLen, 2)
	c.Assert(rv.Object.Props[0].String(), qt.Equals, "backup_retention: >= 30d")
	c.Assert(rv.Object.Props[1].String(), qt.Equals, "point_in_time_recovery: true")
}

func TestParseReferenceValues(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	src := `
sql_database "audit" {
    cluster: sql_cluster.audit & {
        backup_retention: >= 90d
    }
}
for sql_database {
    cluster: default sql_cluster.main
}
for sql_database if tags.domain exists {
    cluster: default sql_cluster[tags.domain]
}
`
	f, err := ParseFile("p.encore", []byte(src))
	c.Assert(err, qt.IsNil)

	audit := f.Rules[0].Props[0].ref()
	c.Assert(audit.Ref.Mode, qt.Equals, StaticRef)
	c.Assert(audit.Ref.Kind, qt.Equals, "sql_cluster")
	c.Assert(audit.Ref.Name, qt.Equals, "audit")
	c.Assert(audit.Object, qt.IsNotNil)

	def := f.Rules[1].Props[0].ref()
	c.Assert(def.Default, qt.IsNotNil)
	c.Assert(def.Default.Ref.String(), qt.Equals, "sql_cluster.main")

	dyn := f.Rules[2].Props[0].ref()
	c.Assert(dyn.Default.Ref.Mode, qt.Equals, DynamicRef)
	c.Assert(dyn.Default.Ref.String(), qt.Equals, "sql_cluster[tags.domain]")
}

func TestParseRequireRemoved(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := ParseFile("p.encore", []byte("for sql_database {\n    require cluster {\n        backup_retention: >= 30d\n    }\n}\n"))
	assertErrContains(c, err,
		"the 'require' block has been removed",
		"constrain a referenced resource with nested object syntax")
}

func TestParseObjectConstraintErrors(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "default in object constraint",
			src:  "for sql_database {\n    cluster: {\n        backup_retention: >= 30d | default 30d\n    }\n}\n",
			want: []string{"'default' is not allowed inside an object constraint"},
		},
		{
			name: "reference combined with scalar",
			src:  "for sql_database {\n    cluster: sql_cluster.main & >= 5\n}\n",
			want: []string{"a reference cannot be combined with scalar constraints"},
		},
		{
			name: "two references",
			src:  "for sql_database {\n    cluster: sql_cluster.main & sql_cluster.audit\n}\n",
			want: []string{"a property cannot have more than one reference"},
		},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			_, err := ParseFile("policy.encore", []byte(tt.src))
			assertErrContains(c, err, tt.want...)
		})
	}
}

func TestValidateObjectConstraint(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// A scalar constraint on a reference property is a type error.
	rs := parseSet(c, "for sql_database {\n    cluster: >= 5\n}\n")
	assertErrContains(c, rs.Validate(),
		"property 'cluster' is a reference and cannot take a scalar constraint")

	// A reference pointed at the wrong kind.
	rs = parseSet(c, "for sql_database {\n    cluster: default service_instance.main\n}\n")
	assertErrContains(c, rs.Validate(),
		"property 'cluster' references service_instance, but it must reference a sql_cluster")

	// A reference on a non-reference property.
	rs = parseSet(c, "for service {\n    cpu: sql_cluster.main\n}\n")
	assertErrContains(c, rs.Validate(),
		"property 'cpu' of service is not a reference property")

	// Duplicate property within an object constraint.
	rs = parseSet(c, "for sql_database {\n    cluster: {\n        cpu: >= 1\n        cpu: <= 4\n    }\n}\n")
	assertErrContains(c, rs.Validate(),
		"duplicate property 'cpu' in the same object constraint")

	// Impossible constraints within an object constraint.
	rs = parseSet(c, "for sql_database {\n    cluster: {\n        cpu: >= 4 & <= 2\n    }\n}\n")
	assertErrContains(c, rs.Validate(),
		"impossible constraints for property 'cpu'",
		"'>= 4' conflicts with '<= 2'")
}

// TestEvalManagedBlockMerging: a named managed block acts like a for rule
// for that resource, merging with other matching rules.
func TestEvalManagedBlockMerging(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_cluster "main" if env.type == "production" {
    cpu: >= 2 & <= 16 | default 4
}
for sql_cluster {
    cpu: <= 8
}
`)
	attrs := strAttrs("env.type", "production")

	// The named block's default applies, narrowed by the kind-wide rule.
	result := evalOK(c, rs, &Resource{Kind: "sql_cluster", Name: "main", Attrs: attrs})
	c.Assert(result.Matched, qt.HasLen, 2)
	assertValue(c, result.Properties["cpu"].Value, Number(4))

	// Constraints from both rules apply.
	assertErrContains(c, evalErr(c, rs, &Resource{
		Kind: "sql_cluster", Name: "main", Attrs: attrs,
		Config: map[string]Value{"cpu": Number(12)},
	}), "property 'cpu' value 12 violates constraint '<= 8'")

	// Other clusters only match the kind-wide rule.
	result = evalOK(c, rs, &Resource{Kind: "sql_cluster", Name: "other", Attrs: attrs})
	c.Assert(result.Matched, qt.HasLen, 1)
}

func TestDefinitions(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_cluster "main" if env.type == "production" {
    engine: "postgres"
}
sql_cluster "audit" if env.type == "production" && tags.compliance exists {
    engine: "postgres"
}
sql_cluster "main" if env.type == "production" && provider == "gcp" {
    cpu: default 8
}
`)

	// Production: main is instantiated (once, despite two blocks).
	defs, err := rs.Definitions(strAttrs("env.type", "production"))
	c.Assert(err, qt.IsNil)
	c.Assert(defs, qt.HasLen, 1)
	c.Assert(defs[0].Kind, qt.Equals, "sql_cluster")
	c.Assert(defs[0].Name, qt.Equals, "main")
	c.Assert(defs[0].Rule.Header(), qt.Equals, `sql_cluster "main" if env.type == "production"`)

	// With the compliance tag, audit is instantiated too.
	defs, err = rs.Definitions(strAttrs("env.type", "production", "tags.compliance", "pci"))
	c.Assert(err, qt.IsNil)
	c.Assert(defs, qt.HasLen, 2)

	// Development environments instantiate nothing.
	defs, err = rs.Definitions(strAttrs("env.type", "development"))
	c.Assert(err, qt.IsNil)
	c.Assert(defs, qt.HasLen, 0)

	// App-discovered named blocks are not instantiated as managed resources.
	rs = parseSet(c, `service "api" { cpu: default 2 }`)
	defs, err = rs.Definitions(nil)
	c.Assert(err, qt.IsNil)
	c.Assert(defs, qt.HasLen, 0)
}

// TestEvalReferencesResolved: Evaluate resolves reference properties into
// Result.References using the resource's own resolved configuration.
func TestEvalReferencesResolved(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for sql_database {
    cluster: sql_cluster.main & {
        backup_retention: >= 30d
    }
}
`)
	result := evalOK(c, rs, &Resource{Kind: "sql_database", Name: "orders"})
	c.Assert(result.Properties["cluster"].Ref, qt.IsNotNil)
	c.Assert(result.Properties["cluster"].Ref.Kind, qt.Equals, "sql_cluster")
	c.Assert(result.Properties["cluster"].Ref.Name, qt.Equals, "main")

	// One identity assertion plus one object-constraint entry.
	c.Assert(result.References, qt.HasLen, 2)
}

// realisticExample is the spec's "final shape" example file.
const realisticExample = `version 1
if env.type == "production" {
    for service {
        cpu: >= 1 & <= 4 | default 1
        memory: >= 1Gi & <= 8Gi | default 1Gi
        instances.min: >= 1 | default 1
    }
    sql_cluster "main" {
        engine: "postgres"
        version: "16"
        cpu: >= 2 & <= 16 | default 4
        memory: >= 8Gi & <= 64Gi | default 16Gi
        storage: >= 100Gi | default 100Gi
        backup_retention: >= 30d | default 30d
        point_in_time_recovery: true
        high_availability: true
    }
    sql_cluster "audit" {
        engine: "postgres"
        version: "16"
        cpu: >= 4 & <= 32 | default 8
        memory: >= 16Gi & <= 128Gi | default 32Gi
        storage: >= 500Gi | default 1Ti
        backup_retention: >= 90d | default 90d
    }
    for sql_database {
        cluster: default sql_cluster.main
    }
    sql_database "audit" {
        cluster: sql_cluster.audit & {
            backup_retention: >= 90d
        }
    }
    for sql_database if tags.data == "customer" {
        cluster: {
            backup_retention: >= 30d
            point_in_time_recovery: true
            high_availability: true
        }
    }
    for bucket {
        public_access: false
        versioning: true
    }
}
`

func TestEvaluateEnvRealisticExample(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, realisticExample)
	c.Assert(rs.Validate(), qt.IsNil)

	envAttrs := strAttrs("env.type", "production")
	er, err := rs.EvaluateEnv(envAttrs, []*Resource{
		{Kind: "service", Name: "api"},
		{Kind: "sql_database", Name: "orders"},
		{Kind: "sql_database", Name: "audit"},
		{Kind: "sql_database", Name: "users", Attrs: strAttrs("tags.data", "customer")},
		{Kind: "bucket", Name: "uploads"},
	})
	c.Assert(err, qt.IsNil)

	// 5 input resources plus the two instantiated clusters.
	c.Assert(er.Results, qt.HasLen, 7)

	// The instantiated clusters exist with their resolved defaults.
	main := er.Get("sql_cluster", "main")
	c.Assert(main, qt.IsNotNil)
	assertValue(c, main.Properties["engine"].Value, String("postgres"))
	assertValue(c, main.Properties["version"].Value, String("16"))
	assertValue(c, main.Properties["cpu"].Value, Number(4))
	assertValue(c, main.Properties["backup_retention"].Value, MustParseQuantity("30d"))
	assertValue(c, main.Properties["high_availability"].Value, Bool(true))

	audit := er.Get("sql_cluster", "audit")
	c.Assert(audit, qt.IsNotNil)
	assertValue(c, audit.Properties["storage"].Value, MustParseQuantity("1Ti"))
	assertValue(c, audit.Properties["backup_retention"].Value, MustParseQuantity("90d"))

	// Databases reference clusters: the default for most, the pinned
	// cluster for the audit database (more specific rule).
	c.Assert(er.Get("sql_database", "orders").Properties["cluster"].Ref.Name, qt.Equals, "main")
	c.Assert(er.Get("sql_database", "audit").Properties["cluster"].Ref.Name, qt.Equals, "audit")
	c.Assert(er.Get("sql_database", "users").Properties["cluster"].Ref.Name, qt.Equals, "main")

	// Services and buckets resolve as usual.
	api := er.Get("service", "api")
	assertValue(c, api.Properties["cpu"].Value, Number(1))
	assertValue(c, api.Properties["instances.min"].Value, Number(1))
	assertValue(c, er.Get("bucket", "uploads").Properties["public_access"].Value, Bool(false))

	// Nothing matches a missing resource.
	c.Assert(er.Get("sql_cluster", "nope"), qt.IsNil)
}

func TestEvaluateEnvObjectConstraintViolation(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_cluster "main" if env.type == "production" {
    backup_retention: >= 7d | default 7d
}
for sql_database if env.type == "production" {
    cluster: sql_cluster.main & {
        backup_retention: >= 30d
    }
}
`)
	_, err := rs.EvaluateEnv(strAttrs("env.type", "production"), []*Resource{
		{Kind: "sql_database", Name: "orders"},
	})
	assertErrContains(c, err,
		`sql_database "orders": the referenced sql_cluster "main" has 'backup_retention' = 7d, violating the constraint '>= 30d'`,
		"the constraint is defined at",
		"the value comes from a default in rule at",
	)

	// An explicitly configured cluster that satisfies the constraint
	// makes the same environment pass.
	er, err := rs.EvaluateEnv(strAttrs("env.type", "production"), []*Resource{
		{Kind: "sql_database", Name: "orders"},
		{Kind: "sql_cluster", Name: "main", Config: map[string]Value{
			"backup_retention": MustParseQuantity("45d"),
		}},
	})
	c.Assert(err, qt.IsNil)
	assertValue(c,
		er.Get("sql_cluster", "main").Properties["backup_retention"].Value,
		MustParseQuantity("45d"))
}

func TestEvaluateEnvMissingTarget(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_database "audit" {
    cluster: sql_cluster.audit & {
        backup_retention: >= 90d
    }
}
`)
	_, err := rs.EvaluateEnv(nil, []*Resource{
		{Kind: "sql_database", Name: "audit"},
	})
	assertErrContains(c, err,
		`sql_database "audit": property 'cluster' references sql_cluster "audit", but no such resource exists in the environment`,
		`instantiate it with: sql_cluster "audit" { ... }`)
}

func TestEvaluateEnvUnsetReference(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for sql_database {
    cluster: {
        backup_retention: >= 30d
    }
}
`)
	_, err := rs.EvaluateEnv(nil, []*Resource{
		{Kind: "sql_database", Name: "orders"},
	})
	assertErrContains(c, err,
		`sql_database "orders": property 'cluster' is not set, but a constraint applies to the referenced sql_cluster`,
		"set 'cluster' on the resource or add a default to a matching rule")
}

func TestEvaluateEnvTargetMissingProperty(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_cluster "main" {
    engine: "postgres"
}
for sql_database {
    cluster: sql_cluster.main & {
        point_in_time_recovery: true
    }
}
`)
	_, err := rs.EvaluateEnv(nil, []*Resource{
		{Kind: "sql_database", Name: "orders"},
	})
	assertErrContains(c, err,
		`sql_database "orders": the referenced sql_cluster "main" does not set property 'point_in_time_recovery', which the constraint needs`,
		"set 'point_in_time_recovery' on the sql_cluster")
}

// TestEvaluateEnvInputOverridesDefinition: an input resource with the same
// kind and name as a managed block is used as-is, so its explicit config
// participates in evaluation.
func TestEvaluateEnvInputOverridesDefinition(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_cluster "main" if env.type == "production" {
    cpu: >= 2 & <= 16 | default 4
}
`)
	envAttrs := strAttrs("env.type", "production")

	er, err := rs.EvaluateEnv(envAttrs, []*Resource{
		{Kind: "sql_cluster", Name: "main", Config: map[string]Value{"cpu": Number(8)}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(er.Results, qt.HasLen, 1)
	rp := er.Get("sql_cluster", "main").Properties["cpu"]
	assertValue(c, rp.Value, Number(8))
	c.Assert(rp.Source, qt.Equals, SourceExplicit)

	// The block's constraints still apply to the explicit config.
	_, err = rs.EvaluateEnv(envAttrs, []*Resource{
		{Kind: "sql_cluster", Name: "main", Config: map[string]Value{"cpu": Number(32)}},
	})
	assertErrContains(c, err, "property 'cpu' value 32 violates constraint '<= 16'")
}

// TestEvaluateEnvObjectConstraintsMergeAcrossRules: object constraints from
// every matching rule apply, like scalar constraints.
func TestEvaluateEnvObjectConstraintsMergeAcrossRules(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
sql_cluster "main" {
    backup_retention: >= 7d | default 35d
    high_availability: false
}
for sql_database {
    cluster: sql_cluster.main & {
        backup_retention: >= 30d
    }
}
for sql_database if tags.data == "customer" {
    cluster: {
        high_availability: true
    }
}
`)
	// Both object constraints match; the first is satisfied (35d >= 30d),
	// the second is violated (high_availability is false).
	_, err := rs.EvaluateEnv(nil, []*Resource{
		{Kind: "sql_database", Name: "users", Attrs: strAttrs("tags.data", "customer")},
	})
	assertErrContains(c, err,
		`sql_database "users": the referenced sql_cluster "main" has 'high_availability' = false, violating the constraint 'true'`)
	c.Assert(err.Error(), qt.Not(qt.Contains), "backup_retention")

	// A non-customer database only triggers the first object constraint,
	// which is satisfied.
	_, err = rs.EvaluateEnv(nil, []*Resource{
		{Kind: "sql_database", Name: "orders"},
	})
	c.Assert(err, qt.IsNil)
}

// TestEvaluateEnvMergesEnvAttrs: environment attributes are merged into
// each input resource's attributes, with the resource's own winning.
func TestEvaluateEnvMergesEnvAttrs(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if env.type == "production" {
    cpu: default 1
}
for service if env.type == "production" && team == "payments" {
    cpu: default 2
}
`)
	er, err := rs.EvaluateEnv(strAttrs("env.type", "production"), []*Resource{
		{Kind: "service", Name: "api", Attrs: strAttrs("team", "payments")},
		{Kind: "service", Name: "worker"},
	})
	c.Assert(err, qt.IsNil)
	assertValue(c, er.Get("service", "api").Properties["cpu"].Value, Number(2))
	assertValue(c, er.Get("service", "worker").Properties["cpu"].Value, Number(1))
}

// TestEvaluateEnvDynamicBlocks: a dynamic block nested in a matching rule
// instantiates and configures kind/normalize(expr) per matching resource;
// resources sharing a domain merge onto one instance.
func TestEvaluateEnvDynamicBlocks(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if tags.domain exists {
    instance: default service_instance[tags.domain]
    service_instance tags.domain {
        cpu: >= 1 & <= 8 | default 2
        memory: >= 1Gi & <= 16Gi | default 4Gi
    }
}
`)
	c.Assert(rs.Validate(), qt.IsNil)

	er, err := rs.EvaluateEnv(strAttrs("env.type", "production"), []*Resource{
		{Kind: "service", Name: "billing", Attrs: strAttrs("tags.domain", "Billing")},
		{Kind: "service", Name: "invoices", Attrs: strAttrs("tags.domain", "Billing")},
		{Kind: "service", Name: "search", Attrs: strAttrs("tags.domain", "search")},
	})
	c.Assert(err, qt.IsNil)

	// Two service_instances are created: billing (shared) and search.
	c.Assert(er.Get("service_instance", "billing"), qt.IsNotNil)
	c.Assert(er.Get("service_instance", "search"), qt.IsNotNil)
	c.Assert(er.Results, qt.HasLen, 5) // 3 services + 2 instances

	// The shared instance gets the block's defaults.
	assertValue(c, er.Get("service_instance", "billing").Properties["cpu"].Value, Number(2))

	// Each service references its instance (normalized name).
	c.Assert(er.Get("service", "billing").Properties["instance"].Ref.Name, qt.Equals, "billing")
	c.Assert(er.Get("service", "invoices").Properties["instance"].Ref.Name, qt.Equals, "billing")
	c.Assert(er.Get("service", "search").Properties["instance"].Ref.Name, qt.Equals, "search")
}

func TestEvaluateEnvDynamicNameCollision(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
for service if tags.domain exists {
    service_instance tags.domain {
        cpu: default 2
    }
}
`)
	_, err := rs.EvaluateEnv(nil, []*Resource{
		{Kind: "service", Name: "a", Attrs: strAttrs("tags.domain", "Billing API")},
		{Kind: "service", Name: "b", Attrs: strAttrs("tags.domain", "billing-api")},
	})
	assertErrContains(c, err,
		`both normalize to service_instance "billing-api"`,
		"give the resources distinct names that do not collide after normalization")
}
