use std::collections::HashSet;
use std::fmt;
use std::rc::Rc;

use crate::ast::{CondOp, Rule};
use crate::value::{string, values_equal, Value};

// Rule specificity: a rule is more specific than another if its selector
// logically implies the other's selector. A rule's optional resource name is
// folded into the selector as a `name == "..."` condition, so it participates
// in specificity naturally.

/// A normalized selector condition used for implication and contradiction
/// checks.
#[derive(Clone)]
pub(crate) struct NormCond {
    pub(crate) field: String,
    pub(crate) op: CondOp,
    pub(crate) values: Vec<Value>,
}

impl fmt::Display for NormCond {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.op {
            CondOp::Eq => write!(f, "{} == {}", self.field, self.values[0]),
            CondOp::Neq => write!(f, "{} != {}", self.field, self.values[0]),
            CondOp::Exists => write!(f, "{} exists", self.field),
            CondOp::In => {
                write!(f, "{} in [", self.field)?;
                for (i, v) in self.values.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{v}")?;
                }
                write!(f, "]")
            }
        }
    }
}

/// Returns the rule's effective selector conditions, including the resource
/// name from the rule header.
pub(crate) fn normalize_rule(r: &Rule) -> Vec<NormCond> {
    let mut conds = Vec::with_capacity(r.wheres.len() + 1);
    if !r.name.is_empty() {
        conds.push(NormCond {
            field: "name".to_string(),
            op: CondOp::Eq,
            values: vec![string(r.name.clone())],
        });
    }
    for c in &r.wheres {
        conds.push(NormCond {
            field: c.field.clone(),
            op: c.op,
            values: c.values.clone(),
        });
    }
    conds
}

/// Reports whether selector `a` logically implies selector `b`: every resource
/// matching `a` also matches `b`.
pub(crate) fn implies(a: &[NormCond], b: &[NormCond]) -> bool {
    for bc in b {
        if !a.iter().any(|ac| entails(ac, bc)) {
            return false;
        }
    }
    true
}

/// Reports whether `a` is strictly more specific than `b`.
pub(crate) fn strictly_implies(a: &[NormCond], b: &[NormCond]) -> bool {
    implies(a, b) && !implies(b, a)
}

/// Reports whether condition `a` being true guarantees condition `b` is true.
fn entails(a: &NormCond, b: &NormCond) -> bool {
    if a.field != b.field {
        return false;
    }
    match b.op {
        CondOp::Exists => true,
        CondOp::Eq => {
            let v = &b.values[0];
            match a.op {
                CondOp::Eq => values_equal(&a.values[0], v),
                CondOp::In => a.values.len() == 1 && values_equal(&a.values[0], v),
                _ => false,
            }
        }
        CondOp::In => match a.op {
            CondOp::Eq => value_in_set(&a.values[0], &b.values),
            CondOp::In => value_subset(&a.values, &b.values),
            _ => false,
        },
        CondOp::Neq => {
            let w = &b.values[0];
            match a.op {
                CondOp::Neq => values_equal(&a.values[0], w),
                CondOp::Eq => !values_equal(&a.values[0], w),
                CondOp::In => !value_in_set(w, &a.values),
                _ => false,
            }
        }
    }
}

/// Reports whether two conditions can never both hold for the same resource.
pub(crate) fn contradicts(a: &NormCond, b: &NormCond) -> bool {
    if a.field != b.field {
        return false;
    }
    // Normalize so the "smaller" op comes first for fewer cases.
    let (a, b) = if a.op > b.op { (b, a) } else { (a, b) };
    match (a.op, b.op) {
        (CondOp::Eq, CondOp::Eq) => !values_equal(&a.values[0], &b.values[0]),
        (CondOp::Eq, CondOp::Neq) => values_equal(&a.values[0], &b.values[0]),
        (CondOp::Eq, CondOp::In) => !value_in_set(&a.values[0], &b.values),
        (CondOp::Neq, CondOp::In) => {
            b.values.len() == 1 && values_equal(&b.values[0], &a.values[0])
        }
        (CondOp::In, CondOp::In) => values_disjoint(&a.values, &b.values),
        _ => false,
    }
}

/// Reports whether some resource could match both selectors. Conservative: it
/// only detects direct per-field contradictions.
pub(crate) fn selectors_can_co_match(a: &[NormCond], b: &[NormCond]) -> bool {
    for ac in a {
        for bc in b {
            if contradicts(ac, bc) {
                return false;
            }
        }
    }
    true
}

/// Renders the union of conditions of the given rules as a selector expression,
/// for use in hints.
pub(crate) fn merged_selector(rules: &[Rc<Rule>]) -> String {
    let mut seen: HashSet<String> = HashSet::new();
    let mut conds: Vec<NormCond> = Vec::new();
    for r in rules {
        for c in normalize_rule(r) {
            let key = c.to_string();
            if seen.insert(key) {
                conds.push(c);
            }
        }
    }
    conds.sort_by(|a, b| a.field.cmp(&b.field));
    conds
        .iter()
        .map(|c| c.to_string())
        .collect::<Vec<_>>()
        .join(" && ")
}

pub(crate) fn value_in_set(v: &Value, set: &[Value]) -> bool {
    set.iter().any(|s| values_equal(v, s))
}

fn value_subset(a: &[Value], b: &[Value]) -> bool {
    a.iter().all(|v| value_in_set(v, b))
}

fn values_disjoint(a: &[Value], b: &[Value]) -> bool {
    !a.iter().any(|v| value_in_set(v, b))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testutil::{assert_value, eval_ok, parse_set, str_attrs};
    use crate::value::number;

    fn cond_eq(f: &str, v: &str) -> NormCond {
        NormCond {
            field: f.into(),
            op: CondOp::Eq,
            values: vec![string(v)],
        }
    }
    fn cond_neq(f: &str, v: &str) -> NormCond {
        NormCond {
            field: f.into(),
            op: CondOp::Neq,
            values: vec![string(v)],
        }
    }
    fn cond_in(f: &str, vs: &[&str]) -> NormCond {
        NormCond {
            field: f.into(),
            op: CondOp::In,
            values: vs.iter().map(|v| string(*v)).collect(),
        }
    }
    fn cond_exists(f: &str) -> NormCond {
        NormCond {
            field: f.into(),
            op: CondOp::Exists,
            values: vec![],
        }
    }

    #[test]
    fn test_implies() {
        #[allow(clippy::type_complexity)]
        let cases: Vec<(&str, Vec<NormCond>, Vec<NormCond>, bool)> = vec![
            (
                "anything implies empty",
                vec![cond_eq("t", "x")],
                vec![],
                true,
            ),
            (
                "empty does not imply condition",
                vec![],
                vec![cond_eq("t", "x")],
                false,
            ),
            (
                "eq implies same eq",
                vec![cond_eq("t", "x")],
                vec![cond_eq("t", "x")],
                true,
            ),
            (
                "eq does not imply different eq",
                vec![cond_eq("t", "x")],
                vec![cond_eq("t", "y")],
                false,
            ),
            (
                "eq implies exists",
                vec![cond_eq("t", "x")],
                vec![cond_exists("t")],
                true,
            ),
            (
                "neq implies exists",
                vec![cond_neq("t", "x")],
                vec![cond_exists("t")],
                true,
            ),
            (
                "in implies exists",
                vec![cond_in("t", &["x"])],
                vec![cond_exists("t")],
                true,
            ),
            (
                "exists does not imply eq",
                vec![cond_exists("t")],
                vec![cond_eq("t", "x")],
                false,
            ),
            (
                "eq implies membership",
                vec![cond_eq("t", "x")],
                vec![cond_in("t", &["x", "y"])],
                true,
            ),
            (
                "eq does not imply non-membership",
                vec![cond_eq("t", "z")],
                vec![cond_in("t", &["x", "y"])],
                false,
            ),
            (
                "singleton in implies eq",
                vec![cond_in("t", &["x"])],
                vec![cond_eq("t", "x")],
                true,
            ),
            (
                "subset implies superset",
                vec![cond_in("t", &["x", "y"])],
                vec![cond_in("t", &["x", "y", "z"])],
                true,
            ),
            (
                "non-subset does not imply",
                vec![cond_in("t", &["x", "w"])],
                vec![cond_in("t", &["x", "y", "z"])],
                false,
            ),
            (
                "eq implies neq of other value",
                vec![cond_eq("t", "x")],
                vec![cond_neq("t", "y")],
                true,
            ),
            (
                "eq does not imply neq of same value",
                vec![cond_eq("t", "x")],
                vec![cond_neq("t", "x")],
                false,
            ),
            (
                "in implies neq of excluded value",
                vec![cond_in("t", &["x", "y"])],
                vec![cond_neq("t", "z")],
                true,
            ),
            (
                "in does not imply neq of member",
                vec![cond_in("t", &["x", "y"])],
                vec![cond_neq("t", "x")],
                false,
            ),
            (
                "neq implies same neq",
                vec![cond_neq("t", "x")],
                vec![cond_neq("t", "x")],
                true,
            ),
            (
                "different fields are unrelated",
                vec![cond_eq("a", "x")],
                vec![cond_eq("b", "x")],
                false,
            ),
            (
                "conjunction implies each part",
                vec![cond_eq("a", "x"), cond_eq("b", "y")],
                vec![cond_eq("a", "x")],
                true,
            ),
            (
                "part does not imply conjunction",
                vec![cond_eq("a", "x")],
                vec![cond_eq("a", "x"), cond_eq("b", "y")],
                false,
            ),
        ];
        for (name, a, b, want) in cases {
            assert_eq!(implies(&a, &b), want, "{name}");
        }
    }

    #[test]
    fn test_contradicts() {
        let cases: Vec<(&str, NormCond, NormCond, bool)> = vec![
            (
                "different eq values",
                cond_eq("t", "x"),
                cond_eq("t", "y"),
                true,
            ),
            (
                "same eq values",
                cond_eq("t", "x"),
                cond_eq("t", "x"),
                false,
            ),
            (
                "eq vs neq of same value",
                cond_eq("t", "x"),
                cond_neq("t", "x"),
                true,
            ),
            (
                "eq vs neq of other value",
                cond_eq("t", "x"),
                cond_neq("t", "y"),
                false,
            ),
            (
                "eq vs set without value",
                cond_eq("t", "x"),
                cond_in("t", &["y", "z"]),
                true,
            ),
            (
                "eq vs set with value",
                cond_eq("t", "x"),
                cond_in("t", &["x", "y"]),
                false,
            ),
            (
                "disjoint sets",
                cond_in("t", &["a", "b"]),
                cond_in("t", &["c", "d"]),
                true,
            ),
            (
                "overlapping sets",
                cond_in("t", &["a", "b"]),
                cond_in("t", &["b", "c"]),
                false,
            ),
            (
                "neq vs singleton set of same value",
                cond_neq("t", "x"),
                cond_in("t", &["x"]),
                true,
            ),
            (
                "neq vs larger set",
                cond_neq("t", "x"),
                cond_in("t", &["x", "y"]),
                false,
            ),
            (
                "exists never contradicts",
                cond_exists("t"),
                cond_eq("t", "x"),
                false,
            ),
            (
                "different fields never contradict",
                cond_eq("a", "x"),
                cond_eq("b", "y"),
                false,
            ),
        ];
        for (name, a, b, want) in cases {
            assert_eq!(contradicts(&a, &b), want, "{name}");
            assert_eq!(contradicts(&b, &a), want, "{name} (reversed)");
        }
    }

    #[test]
    fn eval_subset_membership_specificity() {
        let rs = parse_set(
            "\nfor service if team in [\"a\", \"b\", \"c\"] {\n    cpu: default 1\n}\nfor service if team in [\"a\", \"b\"] {\n    cpu: default 2\n}\n",
        );
        let result = eval_ok(
            &rs,
            &crate::eval::Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("team", "a")]),
                ..Default::default()
            },
        );
        assert_value(&result.properties.get("cpu").unwrap().value, &number(2.0));

        let result = eval_ok(
            &rs,
            &crate::eval::Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("team", "c")]),
                ..Default::default()
            },
        );
        assert_value(&result.properties.get("cpu").unwrap().value, &number(1.0));
    }

    #[test]
    fn validate_disjoint_membership_selectors() {
        let rs = parse_set(
            "\nfor service if env.type in [\"production\", \"staging\"] {\n    cpu: >= 4\n}\nfor service if env.type in [\"preview\", \"development\"] {\n    cpu: <= 2\n}\n",
        );
        assert!(rs.validate().is_ok());
    }
}
