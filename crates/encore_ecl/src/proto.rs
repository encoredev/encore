//! Conversion of an ECL evaluation result into the `encore.ecl.v1` wire schema
//! (see `proto/encore/ecl/v1/ecl.proto`).

use std::collections::{HashMap, HashSet};
use std::rc::Rc;

use crate::ast::{CompareOp, Comparison, Constraint, Rule};
use crate::env::EnvResult;
use crate::eval::{EvalResult, RuleSet, ValueSource};
use crate::pb;
use crate::satisfy::{conjuncts, exact_alternatives, tighter_lower, tighter_upper};
use crate::specificity::value_in_set;
use crate::value::{Value, ValueKind};

impl RuleSet {
    /// Converts an environment evaluation result into the `encore.ecl.v1` wire
    /// schema. `self` supplies whether each resource kind is managed.
    pub fn to_proto(&self, er: &EnvResult) -> pb::EvaluationResult {
        pb::EvaluationResult {
            resources: er
                .results
                .iter()
                .map(|r| self.resource_to_proto(r))
                .collect(),
        }
    }

    fn resource_to_proto(&self, r: &EvalResult) -> pb::Resource {
        // Scalar properties (reference-valued ones go under `references`).
        let mut paths: Vec<&String> = r
            .properties
            .iter()
            .filter(|(_, rp)| rp.reference.is_none())
            .map(|(p, _)| p)
            .collect();
        paths.sort();
        let properties: Vec<pb::Property> = paths
            .into_iter()
            .map(|path| {
                let rp = &r.properties[path];
                pb::Property {
                    path: path.clone(),
                    value: Some(value_to_proto(&rp.value)),
                    source: source_to_proto(rp.source),
                    constraint: merge_constraints(&prop_constraints(path, &r.matched)),
                }
            })
            .collect();

        // Reference-valued properties, grouped by path.
        let mut by_path: HashMap<String, pb::Reference> = HashMap::new();
        let mut order: Vec<String> = Vec::new();
        for rr in &r.references {
            let entry = by_path.entry(rr.path.clone()).or_insert_with(|| {
                order.push(rr.path.clone());
                pb::Reference {
                    path: rr.path.clone(),
                    target_kind: rr.target_kind.clone(),
                    target_name: rr.target_name.clone(),
                    unresolved: rr.unresolved.clone(),
                    object: Vec::new(),
                }
            });
            if let Some(obj) = &rr.object {
                for p in &obj.props {
                    if let Some(sv) = p.scalar() {
                        if let Some(c) = &sv.constraint {
                            entry.object.push(pb::PropertyConstraint {
                                path: p.path.clone(),
                                constraint: merge_constraints(std::slice::from_ref(c)),
                            });
                        }
                    }
                }
            }
        }
        order.sort();
        let references = order
            .into_iter()
            .map(|p| by_path.remove(&p).unwrap())
            .collect();

        pb::Resource {
            kind: r.resource.kind.clone(),
            name: r.resource.name.clone(),
            managed: self.is_managed(&r.resource.kind),
            properties,
            references,
        }
    }
}

/// Collects the scalar constraint expressions for a property path from every
/// rule that matched the resource.
fn prop_constraints(path: &str, matched: &[Rc<Rule>]) -> Vec<Constraint> {
    let mut cs = Vec::new();
    for r in matched {
        for p in &r.props {
            if p.path != path {
                continue;
            }
            if let Some(sv) = p.scalar() {
                if let Some(c) = &sv.constraint {
                    cs.push(c.clone());
                }
            }
        }
    }
    cs
}

/// Normalizes a set of constraints (all conjoined) into the wire Constraint:
/// required flag, min/max bounds, allowed/excluded value sets, plus the
/// effective constraint rendered into `expr`. Returns `None` if empty.
fn merge_constraints(cs: &[Constraint]) -> Option<pb::Constraint> {
    if cs.is_empty() {
        return None;
    }

    let mut expr_parts: Vec<String> = Vec::new();
    let mut seen: HashSet<String> = HashSet::new();
    let mut required = false;
    let mut lower: Option<Comparison> = None;
    let mut upper: Option<Comparison> = None;
    let mut exact: Option<Comparison> = None;
    let mut excluded: Vec<Value> = Vec::new();
    let mut sets: Vec<Vec<Value>> = Vec::new();

    for c in cs {
        let s = c.to_string();
        if seen.insert(s.clone()) {
            expr_parts.push(s);
        }
        let mut terms = Vec::new();
        conjuncts(c, &mut terms);
        for term in terms {
            match term {
                Constraint::Required(_) => required = true,
                Constraint::Comparison(cmp) => match cmp.op {
                    CompareOp::Eq => exact = Some(cmp),
                    CompareOp::Neq => excluded.push(cmp.value),
                    CompareOp::Ge | CompareOp::Gt => {
                        let tighter = match &lower {
                            None => true,
                            Some(l) => tighter_lower(&cmp, l),
                        };
                        if tighter {
                            lower = Some(cmp);
                        }
                    }
                    CompareOp::Le | CompareOp::Lt => {
                        let tighter = match &upper {
                            None => true,
                            Some(u) => tighter_upper(&cmp, u),
                        };
                        if tighter {
                            upper = Some(cmp);
                        }
                    }
                },
                Constraint::Or(alts) => {
                    if let Some(comps) = exact_alternatives(&alts) {
                        sets.push(comps.into_iter().map(|c| c.value).collect());
                    }
                }
                _ => {}
            }
        }
    }

    let mut out = pb::Constraint {
        required,
        expr: expr_parts.join(" & "),
        ..Default::default()
    };
    if let Some(l) = &lower {
        out.min = Some(pb::Bound {
            value: Some(value_to_proto(&l.value)),
            inclusive: l.op == CompareOp::Ge,
        });
    }
    if let Some(u) = &upper {
        out.max = Some(pb::Bound {
            value: Some(value_to_proto(&u.value)),
            inclusive: u.op == CompareOp::Le,
        });
    }

    let allowed: Vec<Value> = if let Some(e) = &exact {
        vec![e.value.clone()]
    } else if let Some((first, rest)) = sets.split_first() {
        let mut a = first.clone();
        for s in rest {
            a.retain(|v| value_in_set(v, s));
        }
        a
    } else {
        Vec::new()
    };
    out.allowed = allowed.iter().map(value_to_proto).collect();
    out.excluded = excluded.iter().map(value_to_proto).collect();
    Some(out)
}

fn value_to_proto(v: &Value) -> pb::Value {
    let (kind, unit) = match v.kind {
        ValueKind::Number => (pb::value::Kind::NumberValue(v.num), String::new()),
        ValueKind::Bool => (pb::value::Kind::BoolValue(v.bool), String::new()),
        ValueKind::String => (pb::value::Kind::StringValue(v.str.clone()), String::new()),
        ValueKind::Size => (pb::value::Kind::SizeBytes(v.num), v.unit.clone()),
        ValueKind::Duration => (pb::value::Kind::DurationMs(v.num), v.unit.clone()),
    };
    pb::Value {
        kind: Some(kind),
        unit,
    }
}

fn source_to_proto(s: ValueSource) -> i32 {
    match s {
        ValueSource::Explicit => pb::ValueSource::Explicit as i32,
        ValueSource::Default => pb::ValueSource::Default as i32,
    }
}

#[cfg(test)]
mod tests {
    use crate::eval::Resource;
    use crate::pb;
    use crate::testutil::{parse_set, str_attrs};

    fn resource<'a>(p: &'a pb::EvaluationResult, kind: &str, name: &str) -> &'a pb::Resource {
        p.resources
            .iter()
            .find(|r| r.kind == kind && r.name == name)
            .expect("resource")
    }
    fn property<'a>(r: &'a pb::Resource, path: &str) -> &'a pb::Property {
        r.properties
            .iter()
            .find(|p| p.path == path)
            .expect("property")
    }
    fn number(v: &Option<pb::Value>) -> f64 {
        match v.as_ref().and_then(|v| v.kind.as_ref()) {
            Some(pb::value::Kind::NumberValue(n)) => *n,
            other => panic!("not a number: {other:?}"),
        }
    }
    fn duration_ms(v: &Option<pb::Value>) -> f64 {
        match v.as_ref().and_then(|v| v.kind.as_ref()) {
            Some(pb::value::Kind::DurationMs(n)) => *n,
            other => panic!("not a duration: {other:?}"),
        }
    }
    fn string_val(v: &Option<pb::Value>) -> &str {
        match v.as_ref().and_then(|v| v.kind.as_ref()) {
            Some(pb::value::Kind::StringValue(s)) => s,
            other => panic!("not a string: {other:?}"),
        }
    }

    #[test]
    fn to_proto() {
        let rs = parse_set(
            "\nif env.type == \"production\" {\n    for service {\n        cpu: >= 1 & <= 4 | default 2\n    }\n    sql_cluster \"main\" {\n        engine: \"postgres\"\n        backup_retention: required & >= 30d | default 30d\n    }\n    for sql_database {\n        cluster: sql_cluster.main & {\n            backup_retention: >= 30d\n        }\n    }\n}\n",
        );
        let er = rs
            .evaluate_env(
                &str_attrs(&[("env.type", "production")]),
                &[
                    Resource {
                        kind: "service".into(),
                        name: "api".into(),
                        ..Default::default()
                    },
                    Resource {
                        kind: "sql_database".into(),
                        name: "orders".into(),
                        ..Default::default()
                    },
                ],
            )
            .unwrap();
        let p = rs.to_proto(&er);
        assert_eq!(p.resources.len(), 3);

        let day = (24 * 60 * 60 * 1000) as f64;

        // service "api": app-discovered; cpu defaulted to 2, constrained [1, 4].
        let api = resource(&p, "service", "api");
        assert!(!api.managed);
        let cpu = property(api, "cpu");
        assert_eq!(number(&cpu.value), 2.0);
        assert_eq!(cpu.source, pb::ValueSource::Default as i32);
        let cc = cpu.constraint.as_ref().unwrap();
        assert!(!cc.required);
        let min = cc.min.as_ref().unwrap();
        assert_eq!(number(&min.value), 1.0);
        assert!(min.inclusive);
        assert_eq!(number(&cc.max.as_ref().unwrap().value), 4.0);
        assert_eq!(cc.expr, ">= 1 & <= 4");

        // sql_cluster "main": managed and instantiated.
        let main = resource(&p, "sql_cluster", "main");
        assert!(main.managed);
        assert_eq!(string_val(&property(main, "engine").value), "postgres");
        let br = property(main, "backup_retention");
        assert_eq!(duration_ms(&br.value), 30.0 * day);
        assert_eq!(br.value.as_ref().unwrap().unit, "d");
        assert_eq!(br.source, pb::ValueSource::Default as i32);
        let bc = br.constraint.as_ref().unwrap();
        assert!(bc.required);
        assert_eq!(duration_ms(&bc.min.as_ref().unwrap().value), 30.0 * day);
        assert_eq!(bc.expr, "required & >= 30d");

        // sql_database "orders": cluster reference resolved to main + object
        // constraint.
        let orders = resource(&p, "sql_database", "orders");
        assert_eq!(orders.references.len(), 1);
        let r = &orders.references[0];
        assert_eq!(r.path, "cluster");
        assert_eq!(r.target_kind, "sql_cluster");
        assert_eq!(r.target_name, "main");
        assert_eq!(r.unresolved, "");
        assert_eq!(r.object.len(), 1);
        assert_eq!(r.object[0].path, "backup_retention");
        let oc = r.object[0].constraint.as_ref().unwrap();
        assert_eq!(duration_ms(&oc.min.as_ref().unwrap().value), 30.0 * day);
    }
}
