use std::rc::Rc;

use crate::ast::{CompareOp, Comparison, Constraint};
use crate::diagnostic::ErrorList;
use crate::eval::{compare_values, RuleProp};
use crate::specificity::value_in_set;
use crate::value::{values_equal, ValueKind};

// Satisfiability checking of merged constraints: detecting that no possible
// value could satisfy the combination of constraints from a set of rules
// (impossible ranges, conflicting exact values, empty allowed-value sets, and
// conflicting types).
//
// The analysis is conservative: it fully understands conjunctions of
// comparisons and disjunctions of exact values; other shapes are skipped rather
// than guessed at. Concrete values are always checked precisely by check_value.

#[derive(Clone)]
pub(crate) struct SatEntry {
    pub(crate) cmp: Comparison,
    pub(crate) src: RuleProp,
}

impl SatEntry {
    fn describe_at(&self) -> String {
        format!(
            "'{}' at {} in rule: {}",
            self.cmp,
            self.cmp.pos,
            self.src.rule.header()
        )
    }
}

#[derive(Clone)]
struct OrEntry {
    or: Constraint,
    values: Vec<crate::value::Value>,
    src: RuleProp,
}

impl OrEntry {
    fn describe_at(&self) -> String {
        format!(
            "'{}' at {} in rule: {}",
            self.or,
            self.or.span().start,
            self.src.rule.header()
        )
    }
}

pub(crate) struct SatChecker<'a> {
    path: String,
    /// resource description for messages; "" outside evaluation
    resource: String,
    diags: &'a mut ErrorList,

    kind_set: bool,
    kind: ValueKind,
    kind_first: Option<SatEntry>,

    lower: Option<SatEntry>, // >= / >
    upper: Option<SatEntry>, // <= / <
    exact: Option<SatEntry>, // ==
    excluded: Vec<SatEntry>,
    sets: Vec<OrEntry>, // disjunctions of exact values

    bail: bool,   // saw a shape the analysis does not understand
    failed: bool, // already reported a conflict
}

/// Verifies that the merged constraints for a property admit at least one
/// value, reporting conflicts otherwise.
pub(crate) fn check_satisfiable(
    path: &str,
    resource: &str,
    diags: &mut ErrorList,
    rcs: &[RuleProp],
) {
    let mut sc = SatChecker::new(path.to_string(), resource.to_string(), diags);
    for rc in rcs {
        sc.feed(rc);
    }
    sc.check();
}

impl<'a> SatChecker<'a> {
    pub(crate) fn new(path: String, resource: String, diags: &'a mut ErrorList) -> SatChecker<'a> {
        SatChecker {
            path,
            resource,
            diags,
            kind_set: false,
            kind: ValueKind::Number,
            kind_first: None,
            lower: None,
            upper: None,
            exact: None,
            excluded: Vec::new(),
            sets: Vec::new(),
            bail: false,
            failed: false,
        }
    }

    pub(crate) fn feed(&mut self, rc: &RuleProp) {
        let sv = match rc.prop.scalar() {
            Some(s) => s,
            None => return,
        };
        let constraint = match &sv.constraint {
            Some(c) => c.clone(),
            None => return,
        };
        let mut terms = Vec::new();
        conjuncts(&constraint, &mut terms);
        for term in terms {
            match term {
                Constraint::Required(_) => {}
                Constraint::Comparison(cmp) => self.add_comparison(SatEntry {
                    cmp,
                    src: rc.clone(),
                }),
                Constraint::Or(alts) => match exact_alternatives(&alts) {
                    Some(comps) => {
                        for cmp in &comps {
                            let e = SatEntry {
                                cmp: cmp.clone(),
                                src: rc.clone(),
                            };
                            self.check_kind(cmp.value.kind, &e);
                        }
                        let values = comps.iter().map(|c| c.value.clone()).collect();
                        self.sets.push(OrEntry {
                            or: Constraint::Or(alts),
                            values,
                            src: rc.clone(),
                        });
                    }
                    None => self.bail = true,
                },
                _ => self.bail = true,
            }
        }
    }

    fn add_comparison(&mut self, e: SatEntry) {
        if !self.check_kind(e.cmp.value.kind, &e) {
            return;
        }
        match e.cmp.op {
            CompareOp::Eq => {
                let dup = match &self.exact {
                    Some(exact) if !values_equal(&exact.cmp.value, &e.cmp.value) => {
                        Some(exact.clone())
                    }
                    _ => None,
                };
                if let Some(ex) = dup {
                    let a_val = ex.cmp.value.clone();
                    let b_val = e.cmp.value.clone();
                    self.conflict(ex, e, format!("it cannot equal both {a_val} and {b_val}"));
                    return;
                }
                if self.exact.is_none() {
                    self.exact = Some(e);
                }
            }
            CompareOp::Neq => self.excluded.push(e),
            CompareOp::Ge | CompareOp::Gt => {
                if self.lower.is_none() || tighter_lower(&e.cmp, &self.lower.as_ref().unwrap().cmp)
                {
                    self.lower = Some(e);
                }
            }
            CompareOp::Le | CompareOp::Lt => {
                if self.upper.is_none() || tighter_upper(&e.cmp, &self.upper.as_ref().unwrap().cmp)
                {
                    self.upper = Some(e);
                }
            }
        }
    }

    /// Enforces that all constraints on the property use the same value type.
    fn check_kind(&mut self, k: ValueKind, e: &SatEntry) -> bool {
        if self.failed {
            return false;
        }
        if !self.kind_set {
            self.kind_set = true;
            self.kind = k;
            self.kind_first = Some(e.clone());
            return true;
        }
        if self.kind != k {
            let first = self.kind_first.clone().unwrap();
            let msg = format!(
                "conflicting types for property '{}': '{}' is a {} constraint, but '{}' compares against a {}",
                self.path, first.cmp, self.kind, e.cmp, k
            );
            let detail = vec![
                format!("  {}", first.describe_at()),
                format!("  {}", e.describe_at()),
            ];
            let src = e.src.rule.src.clone();
            let d = self
                .diags
                .add(src, e.cmp.pos.clone(), e.cmp.end.clone(), msg);
            d.detail = detail;
            self.failed = true;
            return false;
        }
        true
    }

    pub(crate) fn check(&mut self) {
        if self.failed || self.bail {
            return;
        }

        if let Some(exact) = self.exact.clone() {
            let v = exact.cmp.value.clone();
            for b in [self.lower.clone(), self.upper.clone()]
                .into_iter()
                .flatten()
            {
                if !compare_values(&v, &b.cmp.value, b.cmp.op) {
                    self.conflict(exact.clone(), b, "no value can satisfy both".to_string());
                    return;
                }
            }
            for ex in self.excluded.clone() {
                if values_equal(&v, &ex.cmp.value) {
                    self.conflict(
                        exact.clone(),
                        ex,
                        format!("it cannot both equal and not equal {v}"),
                    );
                    return;
                }
            }
            for set in self.sets.clone() {
                if !value_in_set(&v, &set.values) {
                    self.exact_vs_set(exact.clone(), set);
                    return;
                }
            }
            return;
        }

        // Bounds: highest minimum must not exceed lowest maximum.
        if let (Some(lo), Some(hi)) = (self.lower.clone(), self.upper.clone()) {
            let l = &lo.cmp;
            let h = &hi.cmp;
            if l.value.num > h.value.num
                || (l.value.num == h.value.num && (l.op == CompareOp::Gt || h.op == CompareOp::Lt))
            {
                self.conflict(lo, hi, "no value can satisfy both".to_string());
                return;
            }
        }

        // Allowed-value sets: the intersection, filtered by bounds and
        // exclusions, must be non-empty.
        if !self.sets.is_empty() {
            let sets = self.sets.clone();
            let mut allowed = sets[0].values.clone();
            for set in &sets[1..] {
                allowed.retain(|v| value_in_set(v, &set.values));
            }
            let mut viable = Vec::new();
            for v in &allowed {
                let mut ok = true;
                for b in [self.lower.as_ref(), self.upper.as_ref()]
                    .into_iter()
                    .flatten()
                {
                    if !compare_values(v, &b.cmp.value, b.cmp.op) {
                        ok = false;
                    }
                }
                for ex in &self.excluded {
                    if values_equal(v, &ex.cmp.value) {
                        ok = false;
                    }
                }
                if ok {
                    viable.push(v.clone());
                }
            }
            if viable.is_empty() {
                self.empty_sets();
                return;
            }
        }

        // Booleans only have two values; excluding both is impossible.
        if self.kind_set && self.kind == ValueKind::Bool && self.sets.is_empty() {
            let mut ex_true: Option<SatEntry> = None;
            let mut ex_false: Option<SatEntry> = None;
            for ex in &self.excluded {
                if ex.cmp.value.bool {
                    ex_true = Some(ex.clone());
                } else {
                    ex_false = Some(ex.clone());
                }
            }
            if let (Some(t), Some(f)) = (ex_true, ex_false) {
                self.conflict(
                    t,
                    f,
                    "a bool cannot differ from both true and false".to_string(),
                );
            }
        }
    }

    fn header(&self) -> String {
        if !self.resource.is_empty() {
            format!(
                "impossible constraints for property '{}' of {}",
                self.path, self.resource
            )
        } else {
            format!("impossible constraints for property '{}'", self.path)
        }
    }

    fn conflict(&mut self, a: SatEntry, b: SatEntry, reason: String) {
        self.failed = true;
        let header = self.header();
        let msg = format!(
            "{}: '{}' conflicts with '{}': {}",
            header, a.cmp, b.cmp, reason
        );
        let detail = vec![
            format!("  {}", a.describe_at()),
            format!("  {}", b.describe_at()),
        ];
        let differ = !Rc::ptr_eq(&a.src.rule, &b.src.rule);
        let src = b.src.rule.src.clone();
        let d = self
            .diags
            .add(src, b.cmp.pos.clone(), b.cmp.end.clone(), msg);
        d.detail = detail;
        if differ {
            d.hint = "constraints from all matching rules merge by intersection; a rule cannot weaken another rule's constraints".to_string();
        }
    }

    fn exact_vs_set(&mut self, e: SatEntry, set: OrEntry) {
        self.failed = true;
        let header = self.header();
        let msg = format!(
            "{}: {} is not one of the allowed values {}",
            header, e.cmp.value, set.or
        );
        let detail = vec![
            format!("  {}", e.describe_at()),
            format!("  {}", set.describe_at()),
        ];
        let src = e.src.rule.src.clone();
        let d = self
            .diags
            .add(src, e.cmp.pos.clone(), e.cmp.end.clone(), msg);
        d.detail = detail;
    }

    fn empty_sets(&mut self) {
        self.failed = true;
        let header = self.header();
        let first = self.sets[0].clone();
        let msg = format!(
            "{}: no value satisfies all the allowed-value constraints",
            header
        );
        let mut detail = Vec::new();
        for set in &self.sets {
            detail.push(format!("  {}", set.describe_at()));
        }
        for b in [self.lower.as_ref(), self.upper.as_ref()]
            .into_iter()
            .flatten()
        {
            detail.push(format!("  {}", b.describe_at()));
        }
        for ex in &self.excluded {
            detail.push(format!("  {}", ex.describe_at()));
        }
        let span = first.or.span();
        let src = first.src.rule.src.clone();
        let d = self.diags.add(src, span.start, span.end, msg);
        d.detail = detail;
    }
}

pub(crate) fn tighter_lower(a: &Comparison, b: &Comparison) -> bool {
    if a.value.num != b.value.num {
        return a.value.num > b.value.num;
    }
    a.op == CompareOp::Gt && b.op == CompareOp::Ge
}

pub(crate) fn tighter_upper(a: &Comparison, b: &Comparison) -> bool {
    if a.value.num != b.value.num {
        return a.value.num < b.value.num;
    }
    a.op == CompareOp::Lt && b.op == CompareOp::Le
}

/// Flattens top-level conjunctions into a list of terms.
pub(crate) fn conjuncts(c: &Constraint, out: &mut Vec<Constraint>) {
    if let Constraint::And(terms) = c {
        for t in terms {
            conjuncts(t, out);
        }
    } else {
        out.push(c.clone());
    }
}

/// Returns the comparisons of a disjunction whose alternatives are all exact
/// value comparisons, e.g. `"a" | "b" | "c"`.
pub(crate) fn exact_alternatives(alts: &[Constraint]) -> Option<Vec<Comparison>> {
    let mut comps = Vec::with_capacity(alts.len());
    for alt in alts {
        match alt {
            Constraint::Comparison(cmp) if cmp.op == CompareOp::Eq => comps.push(cmp.clone()),
            _ => return None,
        }
    }
    Some(comps)
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use crate::eval::Resource;
    use crate::testutil::{
        assert_err_contains, assert_value, eval_err, eval_ok, parse_set, str_attrs,
    };
    use crate::value::{must_parse_quantity, number, string, Value};

    fn cfg(pairs: &[(&str, Value)]) -> HashMap<String, Value> {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.clone()))
            .collect()
    }

    #[test]
    fn sat_exact_not_in_allowed_set() {
        let rs = parse_set(
            "\nfor service {\n    region: \"us-central1\"\n}\nfor service if env.type == \"production\" {\n    region: \"europe-west1\" | \"europe-north1\"\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: str_attrs(&[("env.type", "production")]),
                    ..Default::default()
                },
            ),
            &["default value \"us-central1\" for property 'region' violates constraint '\"europe-west1\" | \"europe-north1\"'"],
        );
    }

    #[test]
    fn sat_exact_vs_exclusion() {
        let rs = parse_set(
            "\nfor service {\n    flag: true\n}\nfor service if env.type == \"production\" {\n    flag: != true\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: str_attrs(&[("env.type", "production")]),
                    ..Default::default()
                },
            ),
            &["default value true for property 'flag' violates constraint '!= true'"],
        );
    }

    #[test]
    fn sat_conflicts_behind_ambiguous_defaults() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    flag: true\n}\nfor service if team == \"x\" {\n    flag: default false\n}\nfor service {\n    flag: != true\n}\n",
        );
        let err = eval_err(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "x")]),
                ..Default::default()
            },
        );
        assert_err_contains(
            &err,
            &[
                "impossible constraints for property 'flag'",
                "it cannot both equal and not equal true",
            ],
        );
        assert!(!err.to_string().contains("ambiguous"));

        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    region: \"a\"\n}\nfor service if team == \"x\" {\n    region: default \"b\"\n}\nfor service {\n    region: \"b\" | \"c\"\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: str_attrs(&[("env.type", "production"), ("team", "x")]),
                    ..Default::default()
                },
            ),
            &[
                "impossible constraints for property 'region'",
                "\"a\" is not one of the allowed values \"b\" | \"c\"",
            ],
        );
    }

    #[test]
    fn sat_exact_outside_bounds() {
        let rs = parse_set(
            "\nfor service if team == \"payments\" {\n    cpu: >= 4\n}\nservice \"api\" {\n    cpu: 2\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    attrs: str_attrs(&[("team", "payments")]),
                    ..Default::default()
                },
            ),
            &["violates constraint '>= 4'"],
        );
    }

    #[test]
    fn sat_allowed_set_filtered_by_bounds() {
        let rs = parse_set(
            "\nfor service {\n    tier_level: 1 | 2\n}\nfor service if env.type == \"production\" {\n    tier_level: >= 3\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: str_attrs(&[("env.type", "production")]),
                    ..Default::default()
                },
            ),
            &[
                "impossible constraints for property 'tier_level'",
                "no value satisfies all the allowed-value constraints",
                "'1 | 2' at policy.encore",
                "'>= 3' at policy.encore",
            ],
        );
    }

    #[test]
    fn sat_allowed_set_filtered_by_exclusions() {
        let rs = parse_set(
            "\nfor service {\n    tier: \"small\" | \"medium\"\n}\nfor service if env.type == \"production\" {\n    tier: != \"small\" & != \"medium\"\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: str_attrs(&[("env.type", "production")]),
                    ..Default::default()
                },
            ),
            &["no value satisfies all the allowed-value constraints"],
        );
    }

    #[test]
    fn sat_disjunction_with_ranges_not_analyzed() {
        let rs = parse_set(
            "\nfor service {\n    cpu: <= 1 | >= 4\n}\nfor service if env.type == \"production\" {\n    cpu: >= 2 & <= 3\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production")]);

        assert!(rs
            .evaluate(&Resource {
                kind: "service".into(),
                attrs: attrs(),
                ..Default::default()
            })
            .is_ok());

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    attrs: attrs(),
                    config: cfg(&[("cpu", number(2.5))]),
                },
            ),
            &["property 'cpu' value 2.5 violates constraint '<= 1 | >= 4'"],
        );
    }

    #[test]
    fn sat_tighter_bound_wins() {
        let rs = parse_set(
            "\nfor service {\n    cpu: >= 2\n}\nfor service if env.type == \"production\" {\n    cpu: > 2\n}\nfor service if team == \"payments\" {\n    cpu: <= 2\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
                    ..Default::default()
                },
            ),
            &["'> 2' conflicts with '<= 2'"],
        );

        assert!(rs
            .evaluate(&Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("team", "payments")]),
                ..Default::default()
            })
            .is_ok());
    }

    #[test]
    fn eval_required_alone() {
        let rs = parse_set("\nfor sql_database {\n    backup_retention: required\n}\n");
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "sql_database".into(),
                    name: "main".into(),
                    ..Default::default()
                },
            ),
            &["property 'backup_retention' is required but not set"],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "sql_database".into(),
                name: "main".into(),
                config: cfg(&[("backup_retention", must_parse_quantity("7d"))]),
                ..Default::default()
            },
        );
        assert_value(
            &result.properties.get("backup_retention").unwrap().value,
            &must_parse_quantity("7d"),
        );
    }

    #[test]
    fn eval_implicit_default_variants() {
        let rs = parse_set(
            "\nfor service {\n    a: == 2\n    b: required & 3\n    c: >= 1\n    d: \"small\" | \"large\"\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                ..Default::default()
            },
        );
        assert_value(&result.properties.get("a").unwrap().value, &number(2.0));
        assert_eq!(
            result.properties.get("a").unwrap().source,
            crate::eval::ValueSource::Default
        );
        assert_value(&result.properties.get("b").unwrap().value, &number(3.0));
        assert!(!result.properties.contains_key("c"));
        assert!(!result.properties.contains_key("d"));
    }

    #[test]
    fn eval_not_equal_constraint() {
        let rs = parse_set("\nfor service {\n    region: != \"us-central1\"\n}\n");
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                config: cfg(&[("region", string("europe-west1"))]),
                ..Default::default()
            },
        );
        assert_value(
            &result.properties.get("region").unwrap().value,
            &string("europe-west1"),
        );

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    config: cfg(&[("region", string("us-central1"))]),
                    ..Default::default()
                },
            ),
            &["property 'region' value \"us-central1\" violates constraint '!= \"us-central1\"'"],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                ..Default::default()
            },
        );
        assert!(!result.properties.contains_key("region"));
    }
}
