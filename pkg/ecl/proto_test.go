package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"

	eclv1 "encr.dev/proto/encore/ecl/v1"
)

func TestToProto(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	rs := parseSet(c, `
if env.type == "production" {
    for service {
        cpu: >= 1 & <= 4 | default 2
    }
    sql_cluster "main" {
        engine: "postgres"
        backup_retention: required & >= 30d | default 30d
    }
    for sql_database {
        cluster: sql_cluster.main & {
            backup_retention: >= 30d
        }
    }
}
`)
	er, err := rs.EvaluateEnv(strAttrs("env.type", "production"), []*Resource{
		{Kind: "service", Name: "api"},
		{Kind: "sql_database", Name: "orders"},
	})
	c.Assert(err, qt.IsNil)

	pb := rs.ToProto(er)
	// 2 inputs + the instantiated sql_cluster "main".
	c.Assert(pb.Resources, qt.HasLen, 3)

	const day = float64(24 * 60 * 60 * 1000)

	// service "api": app-discovered; cpu defaulted to 2, constrained to [1, 4].
	api := protoResource(pb, "service", "api")
	c.Assert(api, qt.IsNotNil)
	c.Assert(api.Managed, qt.IsFalse)
	cpu := protoProperty(api, "cpu")
	c.Assert(cpu, qt.IsNotNil)
	c.Assert(cpu.Value.GetNumberValue(), qt.Equals, 2.0)
	c.Assert(cpu.Source, qt.Equals, eclv1.ValueSource_VALUE_SOURCE_DEFAULT)
	c.Assert(cpu.Constraint.Required, qt.IsFalse)
	c.Assert(cpu.Constraint.Min.Value.GetNumberValue(), qt.Equals, 1.0)
	c.Assert(cpu.Constraint.Min.Inclusive, qt.IsTrue)
	c.Assert(cpu.Constraint.Max.Value.GetNumberValue(), qt.Equals, 4.0)
	c.Assert(cpu.Constraint.Max.Inclusive, qt.IsTrue)
	c.Assert(cpu.Constraint.Expr, qt.Equals, ">= 1 & <= 4")

	// sql_cluster "main": managed and instantiated.
	main := protoResource(pb, "sql_cluster", "main")
	c.Assert(main, qt.IsNotNil)
	c.Assert(main.Managed, qt.IsTrue)
	c.Assert(protoProperty(main, "engine").Value.GetStringValue(), qt.Equals, "postgres")
	br := protoProperty(main, "backup_retention")
	c.Assert(br.Value.GetDurationMs(), qt.Equals, 30*day)
	c.Assert(br.Value.Unit, qt.Equals, "d")
	c.Assert(br.Source, qt.Equals, eclv1.ValueSource_VALUE_SOURCE_DEFAULT)
	c.Assert(br.Constraint.Required, qt.IsTrue)
	c.Assert(br.Constraint.Min.Value.GetDurationMs(), qt.Equals, 30*day)
	c.Assert(br.Constraint.Expr, qt.Equals, "required & >= 30d")

	// sql_database "orders": cluster reference resolved to main, with an object
	// constraint on the target.
	orders := protoResource(pb, "sql_database", "orders")
	c.Assert(orders.References, qt.HasLen, 1)
	ref := orders.References[0]
	c.Assert(ref.Path, qt.Equals, "cluster")
	c.Assert(ref.TargetKind, qt.Equals, "sql_cluster")
	c.Assert(ref.TargetName, qt.Equals, "main")
	c.Assert(ref.Unresolved, qt.Equals, "")
	c.Assert(ref.Object, qt.HasLen, 1)
	c.Assert(ref.Object[0].Path, qt.Equals, "backup_retention")
	c.Assert(ref.Object[0].Constraint.Min.Value.GetDurationMs(), qt.Equals, 30*day)
}

func protoResource(pb *eclv1.EvaluationResult, kind, name string) *eclv1.Resource {
	for _, r := range pb.Resources {
		if r.Kind == kind && r.Name == name {
			return r
		}
	}
	return nil
}

func protoProperty(r *eclv1.Resource, path string) *eclv1.Property {
	for _, p := range r.Properties {
		if p.Path == path {
			return p
		}
	}
	return nil
}
