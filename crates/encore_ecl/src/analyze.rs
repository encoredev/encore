use std::collections::{HashMap, HashSet};
use std::rc::Rc;

use crate::ast::{
    walk_constraint, Constraint, ObjectConstraint, Property, PropertyValue, Reference,
    RequiredConstraint, Rule,
};
use crate::diagnostic::{Diagnostic, ErrorList, RelatedInfo};
use crate::eval::{check_value, implicit_default, Kind, RuleProp, RuleSet};
use crate::position::{Position, Span};
use crate::satisfy::SatChecker;
use crate::specificity::{contradicts, normalize_rule, selectors_can_co_match};
use crate::util::suggest;
use crate::value::ValueKind;

impl RuleSet {
    /// Statically analyzes the rule set without reference to any concrete
    /// resource, returning an error describing every problem found.
    pub fn validate(&self) -> Result<(), ErrorList> {
        let mut v = Validator {
            rs: self,
            diags: ErrorList::new(),
            bad_props: HashSet::new(),
            reported: HashSet::new(),
        };
        v.run();
        v.diags.sort();
        v.diags.into_result()
    }
}

struct Validator<'a> {
    rs: &'a RuleSet,
    diags: ErrorList,
    /// marks property positions whose property rule already failed a local
    /// check, to avoid piling on cross-rule diagnostics
    bad_props: HashSet<Position>,
    /// dedupe of cross-rule diagnostics
    reported: HashSet<String>,
}

impl Validator<'_> {
    fn run(&mut self) {
        let all: Vec<Rc<Rule>> = self.rs.rules_iter().cloned().collect();
        for r in &all {
            self.check_kind_name(r);
            self.check_selector(r);
            self.check_env_scope(r);
            self.check_properties(r);
        }
        self.check_rule_pairs(&all);
    }

    /// Flags `if` block conditions that reference a non-environment attribute.
    /// Environment scope is the only thing in scope at the top level, so
    /// resource conditions (team, tags, ...) belong on the rule's `if`
    /// clause instead.
    fn check_env_scope(&mut self, r: &Rc<Rule>) {
        let scope = self.rs.env_scope_names();
        if scope.is_empty() {
            return; // explicitly empty scope disables the check
        }
        for c in &r.wheres {
            if !c.env_scoped || self.rs.is_env_scoped(&c.field) {
                continue;
            }
            // `if` conditions are cloned into the rules they wrap; report each
            // offending condition once via the position+message dedupe.
            let mut d = Diagnostic::new(
                r.src.clone(),
                c.pos.clone(),
                c.field_end.clone(),
                format!(
                    "'{}' is not an environment attribute and cannot be tested in an 'if' block",
                    c.field
                ),
            );
            d.hint = format!(
                "'if' tests environment attributes ({}); move resource conditions to each rule's 'if' clause",
                scope.join(", ")
            );
            self.report(d);
        }
    }

    fn check_kind_name(&mut self, r: &Rc<Rule>) {
        let schema = self.rs.schema_map();
        if schema.is_empty() {
            return; // explicitly empty schema disables the check
        }
        if schema.contains_key(&r.kind) {
            return;
        }
        let kinds = sorted_kinds(schema);
        let suggestion = suggest(&r.kind, &kinds);
        let src = r.src.clone();
        let msg = format!("unknown resource kind '{}'", r.kind);
        let d = self
            .diags
            .add(src, r.kind_pos.clone(), r.kind_end.clone(), msg);
        if !suggestion.is_empty() {
            d.hint = format!("did you mean '{suggestion}'?");
        } else {
            d.hint = format!("known kinds: {}", kinds.join(", "));
        }
    }

    /// Flags selectors that can never match.
    fn check_selector(&mut self, r: &Rc<Rule>) {
        let conds = normalize_rule(r);
        // conds[0] is the name condition from the rule header when named; the
        // rest map to r.wheres in order.
        let offset = if !r.name.is_empty() { 1 } else { 0 };
        for i in 0..conds.len() {
            for j in (i + 1)..conds.len() {
                if contradicts(&conds[i], &conds[j]) {
                    let (pos, end) = if j >= offset {
                        let cond = &r.wheres[j - offset];
                        (cond.pos.clone(), cond.end.clone())
                    } else {
                        (r.pos.clone(), Position::default())
                    };
                    let msg = format!(
                        "this rule can never match: '{}' contradicts '{}'",
                        conds[j], conds[i]
                    );
                    let src = r.src.clone();
                    let d = self.diags.add(src, pos, end, msg);
                    d.detail = vec![format!("in rule: {}", r.header())];
                    return;
                }
            }
        }
    }

    fn check_properties(&mut self, r: &Rc<Rule>) {
        let mut seen: HashMap<String, Rc<Property>> = HashMap::new();
        for p in &r.props {
            if let Some(prev) = seen.get(&p.path) {
                let related = RelatedInfo {
                    pos: prev.pos.clone(),
                    message: format!("'{}' was first defined at {}", p.path, prev.pos),
                };
                let msg = format!("duplicate property rule for '{}' in the same rule", p.path);
                let src = r.src.clone();
                let d = self.diags.add(src, p.pos.clone(), p.path_end.clone(), msg);
                d.related.push(related);
                d.hint = "combine the constraints with '&' in a single property rule".to_string();
                self.bad_props.insert(p.pos.clone());
                continue;
            }
            seen.insert(p.path.clone(), p.clone());
            let kind = r.kind.clone();
            self.check_property(r, p, &kind);
        }
    }

    fn check_property(&mut self, r: &Rc<Rule>, p: &Rc<Property>, kind: &str) {
        match &p.value {
            PropertyValue::Ref(_) => self.check_ref_property(r, p, kind),
            _ => self.check_scalar_property(r, p, kind),
        }
    }

    fn check_ref_property(&mut self, r: &Rc<Rule>, p: &Rc<Property>, kind: &str) {
        let rv = p.ref_value().unwrap();
        let want_opt = self.rs.ref_target(kind, &p.path);
        let is_ref = want_opt.is_some();
        let want = want_opt.unwrap_or_default();

        if !is_ref {
            // Only flag when the kind is known; unknown kinds are reported by
            // check_kind_name.
            if self.rs.kind_schema(kind).is_some() {
                let hint = ref_property_hint(self.rs, kind);
                let msg = format!(
                    "property '{}' of {} is not a reference property",
                    p.path, kind
                );
                let src = r.src.clone();
                let d = self.diags.add(src, p.pos.clone(), p.path_end.clone(), msg);
                d.hint = hint;
                self.bad_props.insert(p.pos.clone());
            }
        } else {
            let refs: Vec<&Reference> = [
                rv.reference.as_ref(),
                rv.default.as_ref().map(|d| &d.reference),
            ]
            .into_iter()
            .flatten()
            .collect();
            for reference in refs {
                if reference.kind != want {
                    let msg = format!(
                        "property '{}' references {}, but it must reference a {}",
                        p.path, reference.kind, want
                    );
                    let src = r.src.clone();
                    let d =
                        self.diags
                            .add(src, reference.kind_pos.clone(), reference.end.clone(), msg);
                    d.detail = vec![format!("in rule: {}", r.header())];
                    self.bad_props.insert(p.pos.clone());
                }
            }
        }

        if let Some(obj) = &rv.object {
            self.check_object_constraint(r, obj, &want);
        }
    }

    fn check_object_constraint(&mut self, r: &Rc<Rule>, obj: &ObjectConstraint, target_kind: &str) {
        let mut seen: HashMap<String, Rc<Property>> = HashMap::new();
        for p in &obj.props {
            if let Some(prev) = seen.get(&p.path) {
                let related = RelatedInfo {
                    pos: prev.pos.clone(),
                    message: format!("'{}' was first defined at {}", p.path, prev.pos),
                };
                let msg = format!(
                    "duplicate property '{}' in the same object constraint",
                    p.path
                );
                let src = r.src.clone();
                let d = self.diags.add(src, p.pos.clone(), p.path_end.clone(), msg);
                d.related.push(related);
                d.hint = "combine the constraints with '&' in a single line".to_string();
                continue;
            }
            seen.insert(p.path.clone(), p.clone());
            match &p.value {
                PropertyValue::Ref(_) => self.check_ref_property(r, p, target_kind),
                _ => {
                    if !self.check_property_types(r, p) {
                        continue;
                    }
                    self.check_property_sat(r, p);
                }
            }
        }
    }

    fn check_scalar_property(&mut self, r: &Rc<Rule>, p: &Rc<Property>, kind: &str) {
        let sv = p.scalar().unwrap();

        // A scalar constraint on a declared reference property is a type error.
        if let Some(want) = self.rs.ref_target(kind, &p.path) {
            let msg = format!(
                "property '{}' is a reference and cannot take a scalar constraint",
                p.path
            );
            let src = r.src.clone();
            let d = self.diags.add(src, p.pos.clone(), p.path_end.clone(), msg);
            d.hint = format!("write a reference, e.g.: {}: {}.<name>", p.path, want);
            self.bad_props.insert(p.pos.clone());
            return;
        }

        // 'required' must not appear inside a '|' alternative.
        let mut reqs: Vec<RequiredConstraint> = Vec::new();
        if let Some(c) = &sv.constraint {
            collect_required_in_or(c, &mut reqs);
        }
        for req in &reqs {
            let msg = "'required' cannot be part of a '|' alternative".to_string();
            let src = r.src.clone();
            let d = self.diags.add(src, req.pos.clone(), req.end.clone(), msg);
            d.hint = format!(
                "combine it with '&' instead: {}: required & <constraint>",
                p.path
            );
            self.bad_props.insert(p.pos.clone());
        }

        if !self.check_property_types(r, p) {
            self.bad_props.insert(p.pos.clone());
            return;
        }

        // The default must satisfy the rule's own constraint.
        let sv = p.scalar().unwrap();
        if let (Some(def), Some(constraint)) = (&sv.default, &sv.constraint) {
            let (ok, fail, _) = check_value(&def.value, constraint);
            if !ok {
                let fail = fail.unwrap();
                let msg = format!(
                    "default value {} violates the constraint '{}' in the same property rule",
                    def.value, fail
                );
                let src = r.src.clone();
                let d = self
                    .diags
                    .add(src, def.value_pos.clone(), def.value_end.clone(), msg);
                d.detail = vec![format!("in rule: {}", r.header())];
                self.bad_props.insert(p.pos.clone());
                return;
            }
        }

        if !self.check_property_sat(r, p) {
            self.bad_props.insert(p.pos.clone());
        }
    }

    /// Verifies that all values in a property rule have the same type.
    fn check_property_types(&mut self, r: &Rc<Rule>, p: &Rc<Property>) -> bool {
        let sv = match p.scalar() {
            Some(s) => s,
            None => return true,
        };
        let mut typed: Vec<(ValueKind, String, Span)> = Vec::new();
        walk_constraint(sv.constraint.as_ref(), &mut |c| {
            if let Constraint::Comparison(cmp) = c {
                typed.push((
                    cmp.value.kind,
                    format!("'{cmp}'"),
                    Span {
                        start: cmp.pos.clone(),
                        end: cmp.end.clone(),
                    },
                ));
            }
        });
        if let Some(def) = &sv.default {
            typed.push((
                def.value.kind,
                format!("the default {}", def.value),
                Span {
                    start: def.value_pos.clone(),
                    end: def.value_end.clone(),
                },
            ));
        }

        let mut ok = true;
        if let Some((first_kind, first_desc, _)) = typed.first().cloned() {
            for (k, desc, span) in typed.iter().skip(1) {
                if first_kind != *k {
                    let msg = format!(
                        "mixed value types for property '{}': {} is a {}, but {} is a {}",
                        p.path, first_desc, first_kind, desc, k
                    );
                    let src = r.src.clone();
                    let d = self
                        .diags
                        .add(src, span.start.clone(), span.end.clone(), msg);
                    d.detail = vec![format!("in rule: {}", r.header())];
                    ok = false;
                }
            }
        }
        ok
    }

    /// Verifies that a property rule's own constraints are satisfiable.
    fn check_property_sat(&mut self, r: &Rc<Rule>, p: &Rc<Property>) -> bool {
        let mut local = ErrorList::new();
        {
            let mut sc = SatChecker::new(p.path.clone(), String::new(), &mut local);
            sc.feed(&RuleProp {
                rule: r.clone(),
                prop: p.clone(),
            });
            sc.check();
        }
        let ok = local.is_empty();
        self.diags.extend(local);
        ok
    }

    /// Runs cross-rule checks on every pair of rules of the same kind whose
    /// selectors could match the same resource.
    fn check_rule_pairs(&mut self, all: &[Rc<Rule>]) {
        for i in 0..all.len() {
            for j in (i + 1)..all.len() {
                let a = &all[i];
                let b = &all[j];
                if a.kind != b.kind
                    || !selectors_can_co_match(&normalize_rule(a), &normalize_rule(b))
                {
                    continue;
                }
                self.check_pair_constraints(a, b);
            }
        }
    }

    fn check_pair_constraints(&mut self, a: &Rc<Rule>, b: &Rc<Rule>) {
        for pa in &a.props {
            for pb in &b.props {
                if pa.path != pb.path {
                    continue;
                }
                // Cross-rule constraint checks only apply to scalar properties.
                if pa.scalar().is_none() || pb.scalar().is_none() {
                    continue;
                }
                let rc_a = RuleProp {
                    rule: a.clone(),
                    prop: pa.clone(),
                };
                let rc_b = RuleProp {
                    rule: b.clone(),
                    prop: pb.clone(),
                };
                if self.bad_props.contains(&pa.pos) || self.bad_props.contains(&pb.pos) {
                    continue;
                }
                let mut local = ErrorList::new();
                {
                    let mut sc = SatChecker::new(pa.path.clone(), String::new(), &mut local);
                    sc.feed(&rc_a);
                    sc.feed(&rc_b);
                    sc.check();
                }
                for mut d in local.0 {
                    d.detail.push("the rules can match the same resource, and constraints from all matching rules merge by intersection".to_string());
                    self.report(d);
                }
                self.check_default_against(&rc_a, &rc_b);
                self.check_default_against(&rc_b, &rc_a);
            }
        }
    }

    /// Flags an explicit default in one rule that violates the constraint of
    /// another rule that can match the same resource.
    fn check_default_against(&mut self, da: &RuleProp, other: &RuleProp) {
        let da_sv = match da.prop.scalar() {
            Some(s) => s,
            None => return,
        };
        let other_sv = match other.prop.scalar() {
            Some(s) => s,
            None => return,
        };
        if da_sv.default.is_none() || other_sv.constraint.is_none() {
            return;
        }
        let mut other_has_default = other_sv.default.is_some();
        if !other_has_default {
            other_has_default = implicit_default(other_sv.constraint.as_ref()).is_some();
        }
        if other_has_default
            && !strictly_implies(&normalize_rule(&da.rule), &normalize_rule(&other.rule))
        {
            return;
        }
        let def = da_sv.default.as_ref().unwrap();
        let other_constraint = other_sv.constraint.as_ref().unwrap();
        let (ok, fail, mismatch) = check_value(&def.value, other_constraint);
        if ok || mismatch.is_some() {
            // type conflicts are reported by the satisfiability check
            return;
        }
        let fail = fail.unwrap();
        let mut d = Diagnostic::new(
            da.rule.src.clone(),
            def.value_pos.clone(),
            def.value_end.clone(),
            format!(
                "default value {} for property '{}' violates the constraint '{}' of another rule that can match the same resource",
                def.value, da.prop.path, fail
            ),
        );
        d.detail = vec![
            format!(
                "  the default is defined at {} in rule: {}",
                def.value_pos,
                da.rule.header()
            ),
            format!(
                "  the constraint is defined at {} in rule: {}",
                fail.span().start,
                other.rule.header()
            ),
        ];
        self.report(d);
    }

    /// Appends a diagnostic, deduplicating identical position+message pairs.
    fn report(&mut self, d: Diagnostic) {
        let key = format!("{}\u{0}{}", d.pos, d.message);
        if self.reported.contains(&key) {
            return;
        }
        self.reported.insert(key);
        self.diags.push(d);
    }
}

use crate::specificity::strictly_implies;

fn sorted_kinds(schema: &HashMap<String, Kind>) -> Vec<String> {
    let mut kinds: Vec<String> = schema.keys().cloned().collect();
    kinds.sort();
    kinds
}

/// Lists the reference properties of a kind for diagnostics.
fn ref_property_hint(rs: &RuleSet, kind: &str) -> String {
    match rs.kind_schema(kind) {
        Some(k) if !k.references.is_empty() => {
            let mut names: Vec<String> = k.references.keys().cloned().collect();
            names.sort();
            format!("reference properties of {}: {}", kind, names.join(", "))
        }
        _ => format!("{kind} has no reference properties"),
    }
}

/// Collects every `required` constraint that appears inside a `|` alternative.
fn collect_required_in_or(c: &Constraint, out: &mut Vec<RequiredConstraint>) {
    if let Constraint::Or(alts) = c {
        for alt in alts {
            collect_required(alt, out);
        }
    }
    match c {
        Constraint::And(terms) => {
            for t in terms {
                collect_required_in_or(t, out);
            }
        }
        Constraint::Or(alts) => {
            for a in alts {
                collect_required_in_or(a, out);
            }
        }
        _ => {}
    }
}

fn collect_required(c: &Constraint, out: &mut Vec<RequiredConstraint>) {
    if let Constraint::Required(r) = c {
        out.push(r.clone());
    }
    match c {
        Constraint::And(terms) => {
            for t in terms {
                collect_required(t, out);
            }
        }
        Constraint::Or(alts) => {
            for a in alts {
                collect_required(a, out);
            }
        }
        _ => {}
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use crate::ast::File;
    use crate::eval::{Kind, Resource};
    use crate::parser::parse_file;
    use crate::testutil::{assert_err_contains, assert_value, eval_ok, parse_set, str_attrs};
    use crate::value::number;

    fn parse_ok(src: &str) -> File {
        let pr = parse_file("policy.encore", src);
        assert!(pr.errors.is_empty(), "unexpected errors:\n{}", pr.errors);
        pr.file
    }

    // The complete example, reused from eval tests.
    const COMPLETE_EXAMPLE: &str = include_str!("testdata/complete_example.ecl");

    #[test]
    fn validate_unknown_kind() {
        let rs = parse_set("for sevice { cpu: default 1 }");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &["unknown resource kind 'sevice'", "did you mean 'service'?"],
        );

        let mut rs = parse_set("for widget { size: default 1 }");
        rs.schema = Some(HashMap::from([("widget".to_string(), Kind::default())]));
        assert!(rs.validate().is_ok());

        let mut rs = parse_set("for anything { size: default 1 }");
        rs.schema = Some(HashMap::new());
        assert!(rs.validate().is_ok());
    }

    #[test]
    fn validate_default_violates_own_constraint() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: <= 4 | default 8\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "default value 8 violates the constraint '<= 4' in the same property rule",
                "for service if env.type == \"production\"",
            ],
        );
    }

    #[test]
    fn validate_default_violates_exact_constraint() {
        let rs = parse_set("for bucket {\n    public_access: false | default true\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &["default value true violates the constraint 'false'"],
        );
    }

    #[test]
    fn validate_duplicate_property() {
        let rs = parse_set("\nfor service {\n    cpu: >= 1\n    cpu: <= 4\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "duplicate property rule for 'cpu' in the same rule",
                "'cpu' was first defined at policy.encore:3:5",
                "combine the constraints with '&'",
            ],
        );
    }

    #[test]
    fn validate_contradictory_selector() {
        let cases: &[(&str, &[&str])] = &[
            (
                "for service if env.type == \"production\" && env.type == \"staging\" { cpu: >= 1 }",
                &["this rule can never match", "'env.type == \"staging\"' contradicts 'env.type == \"production\"'"],
            ),
            (
                "for service if team == \"a\" && team != \"a\" { cpu: >= 1 }",
                &["this rule can never match"],
            ),
            (
                "for service if env.type == \"preview\" && env.type in [\"production\", \"staging\"] { cpu: >= 1 }",
                &["this rule can never match"],
            ),
            (
                "service \"api\" if name == \"web\" { cpu: >= 1 }",
                &["this rule can never match", "'name == \"web\"' contradicts 'name == \"api\"'"],
            ),
        ];
        for (src, want) in cases {
            let rs = parse_set(src);
            assert_err_contains(&rs.validate().unwrap_err(), want);
        }
    }

    #[test]
    fn validate_mixed_types_in_property() {
        let rs = parse_set("for service {\n    cpu: >= 1 & <= 2Gi\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "mixed value types for property 'cpu'",
                "'>= 1' is a number, but '<= 2Gi' is a size",
            ],
        );

        let rs = parse_set("for service {\n    memory: >= 1Gi | default 2\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "mixed value types for property 'memory'",
                "the default 2 is a number",
            ],
        );
    }

    #[test]
    fn validate_impossible_single_rule() {
        let rs = parse_set("for service {\n    cpu: >= 4 & <= 2\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "impossible constraints for property 'cpu'",
                "'>= 4' conflicts with '<= 2'",
            ],
        );
    }

    #[test]
    fn validate_impossible_rule_pair() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: >= 4\n}\nfor service if team == \"payments\" {\n    cpu: <= 2\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "impossible constraints for property 'cpu'",
                "'>= 4' conflicts with '<= 2'",
                "the rules can match the same resource",
            ],
        );
    }

    #[test]
    fn validate_conflicting_exact_pair() {
        let rs = parse_set(
            "\nfor bucket if env.type == \"production\" {\n    public_access: false\n}\nbucket \"uploads\" if env.type == \"production\" {\n    public_access: true\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "impossible constraints for property 'public_access'",
                "'false' conflicts with 'true'",
            ],
        );
    }

    #[test]
    fn validate_disjoint_selectors_not_flagged() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: >= 4\n}\nfor service if env.type == \"preview\" {\n    cpu: <= 2\n}\n",
        );
        assert!(rs.validate().is_ok());

        let rs = parse_set(
            "\nbucket \"uploads\" {\n    public_access: true\n}\nbucket \"internal\" {\n    public_access: false\n}\n",
        );
        assert!(rs.validate().is_ok());
    }

    #[test]
    fn validate_cross_rule_default_violation() {
        let rs = parse_set(
            "\nfor service {\n    cpu: <= 4\n}\nservice \"api\" {\n    cpu: default 8\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "default value 8 for property 'cpu' violates the constraint '<= 4' of another rule that can match the same resource",
                "service \"api\"",
                "for service",
            ],
        );
    }

    #[test]
    fn validate_bool_exclusion_conflict() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    flag: != true\n}\nfor service if team == \"payments\" {\n    flag: != false\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "impossible constraints for property 'flag'",
                "a bool cannot differ from both true and false",
            ],
        );
    }

    #[test]
    fn validate_ambiguous_defaults_not_static() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: default 1\n}\nfor service if team == \"payments\" {\n    cpu: default 2\n}\n",
        );
        assert!(rs.validate().is_ok());
    }

    #[test]
    fn validate_clean_rule_set() {
        let rs = parse_set(COMPLETE_EXAMPLE);
        assert!(rs.validate().is_ok());
    }

    // --- if_test.go ---

    #[test]
    fn parse_if_block_desugar() {
        let src = "\nif env.type == \"production\" {\n    for service {\n        cpu: default 1\n    }\n    if env.name == \"prod-eu\" {\n        service \"api\" {\n            cpu: default 2\n        }\n        sql_cluster \"main\" {\n            engine: \"postgres\"\n        }\n    }\n    for bucket {\n        public_access: false\n    }\n}\nfor service {\n    cpu: default 0.5\n}\n";
        let f = parse_ok(src);
        let headers: Vec<String> = f.rules.iter().map(|r| r.header()).collect();
        assert_eq!(
            headers,
            vec![
                "for service if env.type == \"production\"",
                "service \"api\" if env.type == \"production\" && env.name == \"prod-eu\"",
                "sql_cluster \"main\" if env.type == \"production\" && env.name == \"prod-eu\"",
                "for bucket if env.type == \"production\"",
                "for service",
            ]
        );
    }

    #[test]
    fn eval_if_block_scoping() {
        let rs = parse_set(
            "\nfor service {\n    cpu: default 0.5\n}\nif env.type == \"production\" {\n    for service {\n        cpu: default 1\n    }\n    for service if team == \"payments\" {\n        cpu: default 2\n    }\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
                ..Default::default()
            },
        );
        assert_value(&result.properties.get("cpu").unwrap().value, &number(2.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "platform")]),
                ..Default::default()
            },
        );
        assert_value(&result.properties.get("cpu").unwrap().value, &number(1.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                ..Default::default()
            },
        );
        assert_value(&result.properties.get("cpu").unwrap().value, &number(0.5));
    }

    #[test]
    fn validate_if_block_contradiction() {
        let rs = parse_set(
            "\nif env.type == \"production\" {\n    for service if env.type == \"staging\" {\n        cpu: >= 1\n    }\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "this rule can never match",
                "'env.type == \"staging\"' contradicts 'env.type == \"production\"'",
            ],
        );
    }

    #[test]
    fn validate_if_block_resource_attr() {
        let rs = parse_set(
            "\nif team == \"payments\" {\n    for service {\n        cpu: default 1\n    }\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "'team' is not an environment attribute and cannot be tested in an 'if' block",
                "move resource conditions to each rule's 'if' clause",
            ],
        );

        // A custom environment scope can allow additional attributes.
        let mut rs = parse_set(
            "\nif region == \"eu\" {\n    for service {\n        cpu: default 1\n    }\n}\n",
        );
        rs.env_scope = Some(vec!["env".to_string(), "region".to_string()]);
        assert!(rs.validate().is_ok());
    }

    #[test]
    fn parse_where_block_removed() {
        let pr = parse_file(
            "policy.encore",
            "where env.type == \"production\" {\n    for service { cpu: default 1 }\n}\n",
        );
        assert_err_contains(
            &pr.errors,
            &[
                "'where' blocks are now written as 'if' blocks",
                "if env.type == \"production\" { ... }",
            ],
        );
    }

    #[test]
    fn parse_where_clause_removed() {
        let pr = parse_file(
            "policy.encore",
            "for service where env.type == \"production\" {\n}\n",
        );
        assert_err_contains(
            &pr.errors,
            &[
                "conditions on a rule now use 'if', not 'where'",
                r#"for service if env.type == "production" { ... }"#,
            ],
        );
    }

    #[test]
    fn parse_if_block_errors() {
        let cases: &[(&str, &[&str])] = &[
            (
                "if {\n    for service {\n        cpu: default 1\n    }\n}\n",
                &[
                    "expected a condition after 'if'",
                    "e.g.: if env.type == \"production\" { ... }",
                ],
            ),
            (
                "if env.type == \"production\" {\n    cpu: default 1\n}\n",
                &["property rules must appear inside a rule body, not at this level"],
            ),
            (
                "if env.type == \"production\"\n",
                &["expected '{' to begin the if block, found newline"],
            ),
            (
                "if env.type == \"production\" {\n    for service {\n        cpu: default 1\n    }\n",
                &["expected '}' to close the if block, found end of file"],
            ),
        ];
        for (src, want) in cases {
            let pr = parse_file("policy.encore", src);
            assert!(!pr.errors.is_empty(), "src: {src:?}");
            assert_err_contains(&pr.errors, want);
        }
    }

    #[test]
    fn parse_if_block_recovery() {
        let src = "if env.type == \"production\" {\n    for service {\n        cpu: default 2 | <= 4\n    }\n    for bucket {\n        public_access: false\n    }\n}\n";
        let pr = parse_file("policy.encore", src);
        assert_eq!(pr.errors.len(), 1);
        assert!(pr.errors.0[0]
            .message
            .contains("'default' must be the last clause"));
        assert_eq!(pr.file.rules.len(), 2);
        assert_eq!(
            pr.file.rules[1].header(),
            "for bucket if env.type == \"production\""
        );
    }

    #[test]
    fn parse_version_as_property_name() {
        let src = "version 1\nsql_cluster \"main\" {\n    version: \"16\"\n}\nfor service if version == \"2\" {\n    cpu: default 1\n}\n";
        let f = parse_ok(src);
        assert_eq!(f.version.as_ref().unwrap().num, 1);
        assert_eq!(f.rules[0].props[0].to_string(), "version: \"16\"");
        assert_eq!(f.rules[1].header(), "for service if version == \"2\"");
    }
}
