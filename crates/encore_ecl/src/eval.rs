use std::collections::{HashMap, HashSet};
use std::rc::Rc;
use std::sync::OnceLock;

use crate::ast::{
    CompareOp, Comparison, CondOp, Condition, Constraint, File, ObjectConstraint, Property,
    RefMode, Reference, RequiredConstraint, Rule,
};
use crate::diagnostic::{Diagnostic, ErrorList, RelatedInfo};
use crate::position::Position;
use crate::satisfy::check_satisfiable;
use crate::specificity::{merged_selector, normalize_rule, strictly_implies};
use crate::value::{normalize_dynamic_name, string, values_equal, Value, ValueKind};

/// A collection of parsed files evaluated together as a single policy set.
pub struct RuleSet {
    pub files: Vec<File>,
    /// Describes the known resource kinds. If `None`, the default schema is
    /// used. An explicitly empty (non-`None`) map disables kind checking.
    pub schema: Option<HashMap<String, Kind>>,
    /// The environment-scoped attribute names an `if` block may test, e.g.
    /// "env" (covering env.type, env.name, ...) or "provider". A field is
    /// environment-scoped if it equals one of these or has one as a dotted
    /// prefix. If `None`, [`default_env_scope`] is used; an explicitly empty
    /// (non-`None`) vec disables the environment-scope check.
    pub env_scope: Option<Vec<String>>,
}

/// The schema of a resource kind.
#[derive(Clone, Default)]
pub struct Kind {
    /// Whether the kind is infrastructure managed by Encore: a named or dynamic
    /// block of a managed kind instantiates the resource.
    pub managed: bool,
    /// Maps reference-valued property names of this kind to the resource kind
    /// they refer to, e.g. {"cluster": "sql_cluster"}.
    pub references: HashMap<String, String>,
}

/// The default resource-kind schema used by `validate` and `evaluate_env` when
/// `RuleSet::schema` is `None`.
pub fn default_schema() -> HashMap<String, Kind> {
    let r = |k: &str, v: &str| HashMap::from([(k.to_string(), v.to_string())]);
    HashMap::from([
        (
            "service".to_string(),
            Kind {
                managed: false,
                references: r("instance", "service_instance"),
            },
        ),
        (
            "service_instance".to_string(),
            Kind {
                managed: true,
                references: HashMap::new(),
            },
        ),
        (
            "sql_database".to_string(),
            Kind {
                managed: false,
                references: r("cluster", "sql_cluster"),
            },
        ),
        (
            "sql_cluster".to_string(),
            Kind {
                managed: true,
                references: HashMap::new(),
            },
        ),
        ("bucket".to_string(), Kind::default()),
        ("pubsub_topic".to_string(), Kind::default()),
        ("cache".to_string(), Kind::default()),
        ("secret".to_string(), Kind::default()),
        ("cron_job".to_string(), Kind::default()),
    ])
}

fn default_schema_ref() -> &'static HashMap<String, Kind> {
    static DEFAULT: OnceLock<HashMap<String, Kind>> = OnceLock::new();
    DEFAULT.get_or_init(default_schema)
}

/// The default environment-scope attribute names used by `validate` when
/// `RuleSet::env_scope` is `None`. "env" covers env.type, env.name, and other
/// env.* attributes; "provider" is environment-wide.
pub fn default_env_scope() -> Vec<String> {
    vec!["env".to_string(), "provider".to_string()]
}

impl RuleSet {
    /// Combines parsed files into a rule set.
    pub fn new(files: Vec<File>) -> RuleSet {
        RuleSet {
            files,
            schema: None,
            env_scope: None,
        }
    }

    pub(crate) fn env_scope_names(&self) -> Vec<String> {
        match &self.env_scope {
            Some(s) => s.clone(),
            None => default_env_scope(),
        }
    }

    /// Reports whether a field path is an environment attribute: it equals a
    /// declared environment-scope name or has one as a dotted prefix.
    pub(crate) fn is_env_scoped(&self, field: &str) -> bool {
        self.env_scope_names()
            .iter()
            .any(|e| field == e || field.starts_with(&format!("{e}.")))
    }

    pub(crate) fn rules_iter(&self) -> impl Iterator<Item = &Rc<Rule>> {
        self.files.iter().flat_map(|f| f.rules.iter())
    }

    pub(crate) fn schema_map(&self) -> &HashMap<String, Kind> {
        match &self.schema {
            Some(s) => s,
            None => default_schema_ref(),
        }
    }

    pub(crate) fn kind_schema(&self, kind: &str) -> Option<&Kind> {
        self.schema_map().get(kind)
    }

    pub(crate) fn is_managed(&self, kind: &str) -> bool {
        self.kind_schema(kind).map(|k| k.managed).unwrap_or(false)
    }

    /// Returns the kind that the named property of `kind` refers to, if it is a
    /// declared reference property.
    pub(crate) fn ref_target(&self, kind: &str, prop: &str) -> Option<String> {
        self.kind_schema(kind)
            .and_then(|k| k.references.get(prop))
            .cloned()
    }

    /// Applies the rule set to a resource. On failure it returns an `ErrorList`
    /// describing every problem found.
    pub fn evaluate(&self, res: &Resource) -> Result<EvalResult, ErrorList> {
        let (result, mut diags) = self.evaluate_internal(res, &[]);
        if !diags.is_empty() {
            diags.sort();
            return Err(diags);
        }
        Ok(result.expect("evaluate produced no result without diagnostics"))
    }

    /// The internal form of `evaluate`, returning a best-effort result together
    /// with any diagnostics. `extra` holds synthesized rules (e.g. from dynamic
    /// blocks fired by `evaluate_env`) to match in addition to the rule set's
    /// own rules.
    pub(crate) fn evaluate_internal(
        &self,
        res: &Resource,
        extra: &[Rc<Rule>],
    ) -> (Option<EvalResult>, ErrorList) {
        let mut ev = Evaluator::new(self, res);
        if res.kind.is_empty() {
            ev.diags.add(
                None,
                Position::default(),
                Position::default(),
                "resource kind must not be empty".to_string(),
            );
            return (None, ev.diags);
        }

        let mut matched: Vec<Rc<Rule>> = Vec::new();
        for r in self.rules_iter() {
            if ev.matches(r) {
                matched.push(r.clone());
            }
        }
        for r in extra {
            if ev.matches(r) {
                matched.push(r.clone());
            }
        }

        // Group property rules across matching rules by property path.
        let mut props: HashMap<String, Vec<RuleProp>> = HashMap::new();
        let mut order: Vec<String> = Vec::new();
        for r in &matched {
            for p in &r.props {
                if !props.contains_key(&p.path) {
                    order.push(p.path.clone());
                }
                props.entry(p.path.clone()).or_default().push(RuleProp {
                    rule: r.clone(),
                    prop: p.clone(),
                });
            }
        }

        let mut result = EvalResult {
            resource: res.clone(),
            properties: HashMap::new(),
            matched,
            references: Vec::new(),
        };
        for (path, v) in &res.config {
            result.properties.insert(
                path.clone(),
                ResolvedProperty {
                    path: path.clone(),
                    value: v.clone(),
                    reference: None,
                    source: ValueSource::Explicit,
                    default_rule: None,
                },
            );
        }

        for path in &order {
            let rcs = props.get(path).unwrap().clone();
            if is_ref_path(&rcs) {
                ev.resolve_ref_property(path, &rcs, &mut result);
            } else {
                ev.resolve_scalar_property(path, &rcs, &mut result);
            }
        }

        (Some(result), ev.diags)
    }
}

/// A concrete resource to evaluate rules against.
#[derive(Clone, Default)]
pub struct Resource {
    /// resource kind, e.g. "service"
    pub kind: String,
    /// resource name, e.g. "api"
    pub name: String,
    /// selector attributes keyed by dotted field path, e.g. "env.type".
    /// The "name" field is implied by `name` and need not be included.
    pub attrs: HashMap<String, Value>,
    /// explicitly configured properties, keyed by dotted property path.
    pub config: HashMap<String, Value>,
}

impl Resource {
    pub(crate) fn describe(&self) -> String {
        if !self.name.is_empty() {
            format!("{} {}", self.kind, crate::value::go_quote(&self.name))
        } else {
            self.kind.clone()
        }
    }
}

/// Says where a resolved property value came from.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ValueSource {
    /// the resource configured the value itself
    Explicit,
    /// the value came from a rule's default
    Default,
}

/// The final value of a single property.
#[derive(Clone)]
pub struct ResolvedProperty {
    pub path: String,
    /// scalar value; default for reference properties
    pub value: Value,
    /// non-`None` for reference properties
    pub reference: Option<ResolvedRefValue>,
    pub source: ValueSource,
    /// the rule that provided the default; `None` if explicit
    pub default_rule: Option<Rc<Rule>>,
}

/// The resolved target of a reference-valued property.
#[derive(Clone)]
pub struct ResolvedRefValue {
    pub kind: String,
    pub name: String,
}

/// The outcome of evaluating a rule set against a resource. (Named `EvalResult`
/// rather than `Result` to avoid colliding with [`std::result::Result`].)
pub struct EvalResult {
    /// the resource that was evaluated
    pub resource: Resource,
    /// the final resolved configuration: all explicitly configured properties
    /// plus any applied defaults
    pub properties: HashMap<String, ResolvedProperty>,
    /// the rules that matched the resource, in source order
    pub matched: Vec<Rc<Rule>>,
    /// the reference-valued property constraints of all matching rules, with
    /// the target resolved from the resource's own configuration
    pub references: Vec<ResolvedRef>,
}

/// A reference-valued property of a resource, with its target resolved to a
/// concrete (kind, name).
#[derive(Clone)]
pub struct ResolvedRef {
    /// the property path holding the reference
    pub path: String,
    /// the referenced resource kind
    pub target_kind: String,
    /// resolved target name; "" if unresolved
    pub target_name: String,
    /// nested constraints; `None` for identity entries
    pub object: Option<ObjectConstraint>,
    pub rule: Option<Rc<Rule>>,
    pub prop: Option<Rc<Property>>,
    /// reason the reference could not resolve; "" if resolved
    pub(crate) unresolved: String,
}

/// Pairs a rule with one of its property rules.
#[derive(Clone)]
pub(crate) struct RuleProp {
    pub(crate) rule: Rc<Rule>,
    pub(crate) prop: Rc<Property>,
}

pub(crate) struct Evaluator<'a> {
    pub(crate) rs: &'a RuleSet,
    pub(crate) res: &'a Resource,
    pub(crate) diags: ErrorList,
    reported_conds: HashSet<Position>,
}

fn is_ref_path(rcs: &[RuleProp]) -> bool {
    rcs.iter().any(|rc| rc.prop.ref_value().is_some())
}

impl<'a> Evaluator<'a> {
    pub(crate) fn new(rs: &'a RuleSet, res: &'a Resource) -> Evaluator<'a> {
        Evaluator {
            rs,
            res,
            diags: ErrorList::new(),
            reported_conds: HashSet::new(),
        }
    }

    pub(crate) fn into_diags(self) -> ErrorList {
        self.diags
    }

    fn resolve_scalar_property(&mut self, path: &str, rcs: &[RuleProp], result: &mut EvalResult) {
        let have_config = self.res.config.get(path).cloned();
        let mut final_val = have_config.clone();
        let mut have = have_config.is_some();
        let mut def_cand: Option<DefaultCandidate> = None;

        if !have {
            let cands = self.collect_defaults(rcs);
            if !cands.is_empty() {
                let (win, ambiguous) = self.resolve_default(path, &cands);
                if let Some(ambiguous) = ambiguous {
                    // If the constraints themselves are impossible to satisfy,
                    // report that as the root cause instead of the ambiguity.
                    let before = self.diags.len();
                    self.run_check_satisfiable(path, rcs);
                    if self.diags.len() == before {
                        self.diags.push(ambiguous);
                    }
                    return;
                }
                let win = win.unwrap();
                final_val = Some(win.value.clone());
                have = true;
                def_cand = Some(win.clone());
                result.properties.insert(
                    path.to_string(),
                    ResolvedProperty {
                        path: path.to_string(),
                        value: win.value.clone(),
                        reference: None,
                        source: ValueSource::Default,
                        default_rule: Some(win.rule.clone()),
                    },
                );
            }
        }

        if have {
            let final_v = final_val.unwrap();
            for rc in rcs {
                let sv = match rc.prop.scalar() {
                    Some(s) => s,
                    None => continue,
                };
                let constraint = match &sv.constraint {
                    Some(c) => c,
                    None => continue,
                };
                let (ok, fail, mismatch) = check_value(&final_v, constraint);
                if let Some(mismatch) = mismatch {
                    self.type_mismatch(path, &final_v, rc, &mismatch);
                } else if !ok {
                    self.violation(path, &final_v, def_cand.as_ref(), rc, &fail.unwrap());
                }
            }
        } else {
            for rc in rcs {
                if let Some(sv) = rc.prop.scalar() {
                    if let Some(req) = top_level_required(sv.constraint.as_ref()) {
                        self.required_missing(path, rc, &req);
                    }
                }
            }
            self.run_check_satisfiable(path, rcs);
        }
    }

    fn resolve_ref_property(&mut self, path: &str, rcs: &[RuleProp], result: &mut EvalResult) {
        let mut target_kind = self.rs.ref_target(&self.res.kind, path).unwrap_or_default();
        if target_kind.is_empty() {
            for rc in rcs {
                if let Some(rv) = rc.prop.ref_value() {
                    if let Some(r) = &rv.reference {
                        target_kind = r.kind.clone();
                        break;
                    }
                }
            }
        }

        let mut name = String::new();
        let mut have_id = false;
        let mut id_rule: Option<Rc<Rule>> = None;
        let mut id_prop: Option<Rc<Property>> = None;
        let mut id_source = ValueSource::Default;
        let mut unresolved = String::new();

        let cv_str: Option<String> = self
            .res
            .config
            .get(path)
            .filter(|cv| cv.kind == ValueKind::String)
            .map(|cv| cv.str.clone());

        if let Some(s) = cv_str {
            name = s;
            have_id = true;
            id_source = ValueSource::Explicit;
        } else {
            let cands = self.collect_refs(rcs);
            if !cands.is_empty() {
                let (win, ambiguous) = self.resolve_ref(path, &cands);
                if let Some(ambiguous) = ambiguous {
                    self.diags.push(ambiguous);
                } else {
                    let win = win.unwrap();
                    name = win.name.clone();
                    have_id = true;
                    id_rule = Some(win.rule.clone());
                    id_prop = Some(win.prop.clone());
                    unresolved = win.unresolved.clone();
                    if target_kind.is_empty() {
                        target_kind = win.kind.clone();
                    }
                }
            }
        }

        if have_id {
            result.properties.insert(
                path.to_string(),
                ResolvedProperty {
                    path: path.to_string(),
                    value: Value::default(),
                    reference: Some(ResolvedRefValue {
                        kind: target_kind.clone(),
                        name: name.clone(),
                    }),
                    source: id_source,
                    default_rule: id_rule.clone(),
                },
            );
            result.references.push(ResolvedRef {
                path: path.to_string(),
                target_kind: target_kind.clone(),
                target_name: name.clone(),
                object: None,
                rule: id_rule.clone(),
                prop: id_prop.clone(),
                unresolved: unresolved.clone(),
            });
        }

        // Object constraints from every matching rule apply to the target.
        for rc in rcs {
            let rv = match rc.prop.ref_value() {
                Some(rv) => rv,
                None => continue,
            };
            let obj = match &rv.object {
                Some(o) => o,
                None => continue,
            };
            result.references.push(ResolvedRef {
                path: path.to_string(),
                target_kind: target_kind.clone(),
                target_name: name.clone(),
                object: Some(obj.clone()),
                rule: Some(rc.rule.clone()),
                prop: Some(rc.prop.clone()),
                unresolved: unresolved.clone(),
            });
        }
    }

    // --- rule matching ---

    pub(crate) fn matches(&mut self, r: &Rc<Rule>) -> bool {
        // A dynamic block matches no resource directly; it fires via
        // evaluate_env, which synthesizes named rules.
        if !r.dyn_expr.is_empty() {
            return false;
        }
        if r.kind != self.res.kind {
            return false;
        }
        if !r.name.is_empty() && r.name != self.res.name {
            return false;
        }
        for c in &r.wheres {
            if !self.eval_cond(r, c) {
                return false;
            }
        }
        true
    }

    pub(crate) fn lookup_field(&self, field: &str) -> (Value, bool) {
        if field == "name" {
            if self.res.name.is_empty() {
                return (Value::default(), false);
            }
            return (string(self.res.name.clone()), true);
        }
        match self.res.attrs.get(field) {
            Some(v) => (v.clone(), true),
            None => (Value::default(), false),
        }
    }

    pub(crate) fn eval_cond(&mut self, r: &Rc<Rule>, c: &Condition) -> bool {
        let (v, ok) = self.lookup_field(&c.field);
        if c.op == CondOp::Exists {
            return ok;
        }
        if !ok {
            return false;
        }
        match c.op {
            CondOp::Eq | CondOp::Neq => {
                let want = &c.values[0];
                if v.kind != want.kind {
                    self.selector_type_mismatch(r, c, &v, want);
                    return false;
                }
                let eq = values_equal(&v, want);
                if c.op == CondOp::Neq {
                    !eq
                } else {
                    eq
                }
            }
            CondOp::In => {
                let mut kind_ok = false;
                for want in &c.values {
                    if want.kind == v.kind {
                        kind_ok = true;
                        if values_equal(&v, want) {
                            return true;
                        }
                    }
                }
                if !kind_ok {
                    let first = c.values[0].clone();
                    self.selector_type_mismatch(r, c, &v, &first);
                }
                false
            }
            CondOp::Exists => false,
        }
    }

    fn selector_type_mismatch(&mut self, r: &Rc<Rule>, c: &Condition, got: &Value, want: &Value) {
        if self.reported_conds.contains(&c.pos) {
            return;
        }
        self.reported_conds.insert(c.pos.clone());
        let msg = format!(
            "type mismatch in selector condition '{}': attribute '{}' of {} is the {} {}, not a {}",
            c,
            c.field,
            self.res.describe(),
            got.kind,
            got,
            want.kind
        );
        let src = r.src.clone();
        let d = self.diags.add(src, c.pos.clone(), c.end.clone(), msg);
        d.hint = "the rule is treated as not matching this resource".to_string();
    }

    // --- defaults ---

    fn collect_defaults(&self, rcs: &[RuleProp]) -> Vec<DefaultCandidate> {
        let mut cands = Vec::new();
        for rc in rcs {
            let sv = match rc.prop.scalar() {
                Some(s) => s,
                None => continue,
            };
            if let Some(def) = &sv.default {
                cands.push(DefaultCandidate {
                    rule: rc.rule.clone(),
                    prop: rc.prop.clone(),
                    value: def.value.clone(),
                    pos: def.value_pos.clone(),
                    end: def.value_end.clone(),
                    implicit: false,
                });
            } else if let Some(cmp) = implicit_default(sv.constraint.as_ref()) {
                cands.push(DefaultCandidate {
                    rule: rc.rule.clone(),
                    prop: rc.prop.clone(),
                    value: cmp.value.clone(),
                    pos: cmp.pos.clone(),
                    end: cmp.end.clone(),
                    implicit: true,
                });
            }
        }
        cands
    }

    /// Picks the default from the most specific matching rule. Returns a
    /// diagnostic describing the ambiguity if there is no unique winner.
    fn resolve_default(
        &self,
        path: &str,
        cands: &[DefaultCandidate],
    ) -> (Option<DefaultCandidate>, Option<Diagnostic>) {
        for (i, c) in cands.iter().enumerate() {
            let mut winner = true;
            for (j, o) in cands.iter().enumerate() {
                if i == j || values_equal(&c.value, &o.value) {
                    continue;
                }
                if !strictly_implies(&normalize_rule(&c.rule), &normalize_rule(&o.rule)) {
                    winner = false;
                    break;
                }
            }
            if winner {
                return (Some(c.clone()), None);
            }
        }

        let mut d = Diagnostic::new(
            cands[0].rule.src.clone(),
            cands[0].pos.clone(),
            cands[0].end.clone(),
            format!(
                "ambiguous default for property '{}' of {}",
                path,
                self.res.describe()
            ),
        );
        d.detail = vec!["matching rules provide different defaults:".to_string()];
        let mut rules = Vec::new();
        for c in cands {
            rules.push(c.rule.clone());
            d.detail
                .push(format!("  {}: {}", c.rule.pos, c.rule.header()));
            d.detail.push(format!("      {}", c.prop));
        }
        d.detail
            .push("no rule is more specific than all the others".to_string());
        d.hint = format!(
            "add a more specific rule that decides the default, e.g.:\n    for {} if {} {{\n        {}: default {}\n    }}",
            self.res.kind,
            merged_selector(&rules),
            path,
            cands[cands.len() - 1].value
        );
        (None, Some(d))
    }

    // --- references ---

    fn collect_refs(&self, rcs: &[RuleProp]) -> Vec<RefCandidate> {
        let mut cands = Vec::new();
        for rc in rcs {
            let rv = match rc.prop.ref_value() {
                Some(rv) => rv,
                None => continue,
            };
            if let Some(d) = &rv.default {
                cands.push(self.ref_candidate(rc, &d.reference, d.pos.clone(), false));
            } else if let Some(r) = &rv.reference {
                cands.push(self.ref_candidate(rc, r, r.pos.clone(), true));
            }
        }
        cands
    }

    fn ref_candidate(
        &self,
        rc: &RuleProp,
        reference: &Reference,
        pos: Position,
        implicit: bool,
    ) -> RefCandidate {
        let (kind, name, unresolved) = self.resolve_reference(reference);
        RefCandidate {
            rule: rc.rule.clone(),
            prop: rc.prop.clone(),
            kind,
            name,
            unresolved,
            implicit,
            pos,
            end: reference.end.clone(),
        }
    }

    /// Resolves a reference to a concrete (kind, name).
    fn resolve_reference(&self, reference: &Reference) -> (String, String, String) {
        if reference.mode == RefMode::Static {
            return (
                reference.kind.clone(),
                reference.name.clone(),
                String::new(),
            );
        }
        let (v, ok) = self.lookup_field(&reference.expr);
        if !ok {
            return (
                reference.kind.clone(),
                String::new(),
                format!(
                    "attribute '{}' is not set on {}",
                    reference.expr,
                    self.res.describe()
                ),
            );
        }
        if v.kind != ValueKind::String {
            return (
                reference.kind.clone(),
                String::new(),
                format!(
                    "attribute '{}' is the {} {}, not a string",
                    reference.expr, v.kind, v
                ),
            );
        }
        match normalize_dynamic_name(&v.str) {
            Ok(norm) => (reference.kind.clone(), norm, String::new()),
            Err(e) => (reference.kind.clone(), String::new(), e.to_string()),
        }
    }

    fn resolve_ref(
        &self,
        path: &str,
        cands: &[RefCandidate],
    ) -> (Option<RefCandidate>, Option<Diagnostic>) {
        for (i, c) in cands.iter().enumerate() {
            let mut winner = true;
            for (j, o) in cands.iter().enumerate() {
                if i == j || ref_cand_equal(c, o) {
                    continue;
                }
                if !strictly_implies(&normalize_rule(&c.rule), &normalize_rule(&o.rule)) {
                    winner = false;
                    break;
                }
            }
            if winner {
                return (Some(c.clone()), None);
            }
        }

        let mut d = Diagnostic::new(
            cands[0].rule.src.clone(),
            cands[0].pos.clone(),
            cands[0].end.clone(),
            format!(
                "ambiguous reference for property '{}' of {}",
                path,
                self.res.describe()
            ),
        );
        d.detail = vec!["matching rules point the reference at different resources:".to_string()];
        for c in cands {
            d.detail
                .push(format!("  {}: {}", c.rule.pos, c.rule.header()));
            d.detail.push(format!("      {}", c.prop));
        }
        d.detail
            .push("no rule is more specific than all the others".to_string());
        (None, Some(d))
    }

    // --- diagnostics ---

    fn violation(
        &mut self,
        path: &str,
        val: &Value,
        def_cand: Option<&DefaultCandidate>,
        rc: &RuleProp,
        fail: &Constraint,
    ) {
        let span = fail.span();
        let src = rc.rule.src.clone();
        let detail = vec![
            format!("the constraint is defined at {} in rule:", rc.rule.pos),
            format!("  {}", rc.rule.header()),
            format!("      {}", rc.prop),
        ];
        if let Some(dc) = def_cand {
            let related = RelatedInfo {
                pos: dc.pos.clone(),
                message: format!(
                    "the default is defined at {} in rule: {}",
                    dc.pos,
                    dc.rule.header()
                ),
            };
            let msg = format!(
                "{}: default value {} for property '{}' violates constraint '{}'",
                self.res.describe(),
                val,
                path,
                fail
            );
            let d = self.diags.add(src, span.start, span.end, msg);
            d.related.push(related);
            d.detail = detail;
        } else {
            let msg = format!(
                "{}: property '{}' value {} violates constraint '{}'",
                self.res.describe(),
                path,
                val,
                fail
            );
            let d = self.diags.add(src, span.start, span.end, msg);
            d.detail = detail;
        }
    }

    fn type_mismatch(&mut self, path: &str, val: &Value, rc: &RuleProp, cmp: &Comparison) {
        let src = rc.rule.src.clone();
        let msg = format!(
            "{}: property '{}' has {} value {}, but the constraint '{}' expects a {}",
            self.res.describe(),
            path,
            val.kind,
            val,
            cmp,
            cmp.value.kind
        );
        let detail = vec![
            format!("the constraint is defined at {} in rule:", rc.rule.pos),
            format!("  {}", rc.rule.header()),
            format!("      {}", rc.prop),
        ];
        let d = self.diags.add(src, cmp.pos.clone(), cmp.end.clone(), msg);
        d.detail = detail;
    }

    fn required_missing(&mut self, path: &str, rc: &RuleProp, req: &RequiredConstraint) {
        let src = rc.rule.src.clone();
        let msg = format!(
            "{}: property '{}' is required but not set, and no default applies",
            self.res.describe(),
            path
        );
        let detail = vec![
            format!("required by rule at {}:", rc.rule.pos),
            format!("  {}", rc.rule.header()),
            format!("      {}", rc.prop),
        ];
        let d = self.diags.add(src, req.pos.clone(), req.end.clone(), msg);
        d.detail = detail;
        d.hint = format!(
            "set '{}' on the resource, or add 'default <value>' to a matching rule",
            path
        );
    }

    fn run_check_satisfiable(&mut self, path: &str, rcs: &[RuleProp]) {
        let resource = self.res.describe();
        check_satisfiable(path, &resource, &mut self.diags, rcs);
    }
}

/// A candidate default value for a scalar property.
#[derive(Clone)]
struct DefaultCandidate {
    rule: Rc<Rule>,
    prop: Rc<Property>,
    value: Value,
    pos: Position,
    end: Position,
    #[allow(dead_code)]
    implicit: bool,
}

/// A candidate identity for a reference-valued property.
#[derive(Clone)]
struct RefCandidate {
    rule: Rc<Rule>,
    prop: Rc<Property>,
    kind: String,
    name: String,
    unresolved: String,
    #[allow(dead_code)]
    implicit: bool,
    pos: Position,
    end: Position,
}

fn ref_cand_equal(a: &RefCandidate, b: &RefCandidate) -> bool {
    a.kind == b.kind && a.name == b.name
}

// --- constraint checking ---

/// Checks a value against a constraint expression. Returns whether the
/// constraint is satisfied; if not, `fail` is the smallest subexpression that
/// failed. If the value's type does not match the constraint at all, `mismatch`
/// is the offending comparison.
pub(crate) fn check_value(
    v: &Value,
    c: &Constraint,
) -> (bool, Option<Constraint>, Option<Comparison>) {
    match c {
        Constraint::Required(_) => (true, None, None),
        Constraint::Comparison(t) => {
            if v.kind != t.value.kind {
                return (false, Some(c.clone()), Some(t.clone()));
            }
            if compare_values(v, &t.value, t.op) {
                (true, None, None)
            } else {
                (false, Some(c.clone()), None)
            }
        }
        Constraint::And(terms) => {
            for term in terms {
                let (ok, fail, mismatch) = check_value(v, term);
                if !ok {
                    return (false, fail, mismatch);
                }
            }
            (true, None, None)
        }
        Constraint::Or(alts) => {
            let mut first_mismatch: Option<Comparison> = None;
            let mut all_mismatch = true;
            for alt in alts {
                let (ok, _fail, m) = check_value(v, alt);
                if let Some(m) = m {
                    if first_mismatch.is_none() {
                        first_mismatch = Some(m);
                    }
                    continue;
                }
                all_mismatch = false;
                if ok {
                    return (true, None, None);
                }
            }
            if all_mismatch {
                return (false, Some(c.clone()), first_mismatch);
            }
            (false, Some(c.clone()), None)
        }
        _ => (true, None, None),
    }
}

pub(crate) fn compare_values(a: &Value, b: &Value, op: CompareOp) -> bool {
    match a.kind {
        ValueKind::Number | ValueKind::Size | ValueKind::Duration => {
            let cmp = if a.num < b.num {
                -1
            } else if a.num > b.num {
                1
            } else {
                0
            };
            op.holds(cmp)
        }
        ValueKind::Bool => match op {
            CompareOp::Eq => a.bool == b.bool,
            CompareOp::Neq => a.bool != b.bool,
            _ => false,
        },
        ValueKind::String => match op {
            CompareOp::Eq => a.str == b.str,
            CompareOp::Neq => a.str != b.str,
            _ => false,
        },
    }
}

/// Returns the `required` constraint if it appears as a top-level conjunct.
fn top_level_required(c: Option<&Constraint>) -> Option<RequiredConstraint> {
    match c {
        Some(Constraint::Required(t)) => Some(t.clone()),
        Some(Constraint::And(terms)) => {
            for term in terms {
                if let Constraint::Required(req) = term {
                    return Some(req.clone());
                }
            }
            None
        }
        _ => None,
    }
}

/// Returns the exact value comparison that acts as an implicit default for the
/// constraint, if any.
pub(crate) fn implicit_default(c: Option<&Constraint>) -> Option<Comparison> {
    match c {
        Some(Constraint::Comparison(t)) => {
            if t.op == CompareOp::Eq {
                Some(t.clone())
            } else {
                None
            }
        }
        Some(Constraint::And(terms)) => {
            let mut eq: Option<Comparison> = None;
            for term in terms {
                match term {
                    Constraint::Required(_) => {}
                    Constraint::Comparison(tt) => {
                        if tt.op != CompareOp::Eq
                            || (eq.is_some()
                                && !values_equal(&eq.as_ref().unwrap().value, &tt.value))
                        {
                            return None;
                        }
                        if eq.is_none() {
                            eq = Some(tt.clone());
                        }
                    }
                    _ => return None,
                }
            }
            eq
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testutil::{
        assert_err_contains, assert_value, eval_err, eval_ok, parse_set, str_attrs,
    };
    use crate::value::{boolean, must_parse_quantity, number, string};

    fn cfg(pairs: &[(&str, Value)]) -> HashMap<String, Value> {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.clone()))
            .collect()
    }

    fn val(result: &EvalResult, path: &str) -> Value {
        result.properties.get(path).unwrap().value.clone()
    }

    #[test]
    fn eval_kind_wide_default() {
        let rs = parse_set(
            "\nfor service {\n    cpu: >= 0.25 & <= 8 | default 0.5\n}\nfor service if env.type == \"production\" {\n    cpu: >= 1 & <= 4 | default 1\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 2);
        let rp = result.properties.get("cpu").unwrap();
        assert_value(&rp.value, &number(1.0));
        assert_eq!(rp.source, ValueSource::Default);
        assert_eq!(
            rp.default_rule.as_ref().unwrap().header(),
            "for service if env.type == \"production\""
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: str_attrs(&[("env.type", "development")]),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 1);
        assert_value(&val(&result, "cpu"), &number(0.5));
    }

    #[test]
    fn eval_named_rule_default() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: >= 1 & <= 4 | default 1\n}\nservice \"api\" if env.type == \"production\" {\n    cpu: default 2\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(2.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "worker".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
    }

    #[test]
    fn eval_ambiguous_default() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: default 1\n}\nfor service if team == \"payments\" {\n    cpu: default 2\n}\n",
        );
        let err = eval_err(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
                ..Default::default()
            },
        );
        assert_err_contains(
            &err,
            &[
                "ambiguous default for property 'cpu' of service \"api\"",
                "matching rules provide different defaults:",
                "policy.encore:2:1: for service if env.type == \"production\"",
                "cpu: default 1",
                "policy.encore:5:1: for service if team == \"payments\"",
                "cpu: default 2",
                "no rule is more specific than all the others",
                "for service if env.type == \"production\" && team == \"payments\"",
            ],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "platform")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
    }

    #[test]
    fn eval_same_default_not_ambiguous() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: default 1\n}\nfor service if team == \"payments\" {\n    cpu: default 1\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
    }

    #[test]
    fn eval_constraints_merge_by_intersection() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: <= 4\n}\nfor service if team == \"payments\" {\n    cpu: >= 2\n}\n",
        );
        let res = |cpu: f64| Resource {
            kind: "service".into(),
            name: "api".into(),
            attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
            config: cfg(&[("cpu", number(cpu))]),
        };

        let result = eval_ok(&rs, &res(3.0));
        assert_eq!(
            result.properties.get("cpu").unwrap().source,
            ValueSource::Explicit
        );

        assert_err_contains(
            &eval_err(&rs, &res(1.0)),
            &[
                "service \"api\": property 'cpu' value 1 violates constraint '>= 2'",
                "for service if team == \"payments\"",
            ],
        );
        assert_err_contains(
            &eval_err(&rs, &res(8.0)),
            &[
                "service \"api\": property 'cpu' value 8 violates constraint '<= 4'",
                "for service if env.type == \"production\"",
            ],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
                ..Default::default()
            },
        );
        assert!(!result.properties.contains_key("cpu"));
    }

    #[test]
    fn eval_explicit_beats_default_but_not_constraints() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: >= 1 & <= 4 | default 1\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production")]);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: attrs(),
                config: cfg(&[("cpu", number(3.0))]),
                ..Default::default()
            },
        );
        let rp = result.properties.get("cpu").unwrap();
        assert_value(&rp.value, &number(3.0));
        assert_eq!(rp.source, ValueSource::Explicit);
        assert!(rp.default_rule.is_none());

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    attrs: attrs(),
                    config: cfg(&[("cpu", number(8.0))]),
                    ..Default::default()
                },
            ),
            &["property 'cpu' value 8 violates constraint '<= 4'"],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: attrs(),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
    }

    #[test]
    fn eval_exact_value_acts_as_default() {
        let rs = parse_set("\nfor bucket {\n    public_access: false\n    versioning: true\n}\n");
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "bucket".into(),
                name: "uploads".into(),
                ..Default::default()
            },
        );
        let rp = result.properties.get("public_access").unwrap();
        assert_value(&rp.value, &boolean(false));
        assert_eq!(rp.source, ValueSource::Default);
        assert_value(&val(&result, "versioning"), &boolean(true));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "bucket".into(),
                name: "uploads".into(),
                config: cfg(&[("public_access", boolean(false))]),
                ..Default::default()
            },
        );
        assert_eq!(
            result.properties.get("public_access").unwrap().source,
            ValueSource::Explicit
        );

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "bucket".into(),
                    name: "uploads".into(),
                    config: cfg(&[("public_access", boolean(true))]),
                    ..Default::default()
                },
            ),
            &["bucket \"uploads\": property 'public_access' value true violates constraint 'false'"],
        );
    }

    #[test]
    fn eval_conflicting_exact_values() {
        let rs = parse_set(
            "\nfor bucket if env.type == \"production\" {\n    public_access: false\n}\nbucket \"uploads\" {\n    public_access: true\n}\n",
        );
        let err = eval_err(
            &rs,
            &Resource {
                kind: "bucket".into(),
                name: "uploads".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert_err_contains(
            &err,
            &[
                "impossible constraints for property 'public_access' of bucket \"uploads\"",
                "'false' conflicts with 'true'",
                "it cannot equal both false and true",
                "for bucket if env.type == \"production\"",
                "bucket \"uploads\"",
            ],
        );
        assert!(!err.to_string().contains("ambiguous"));
    }

    #[test]
    fn eval_exact_plus_range_conflict() {
        let rs = parse_set("\nfor service {\n    cpu: <= 4\n}\nservice \"api\" {\n    cpu: 8\n}\n");
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    ..Default::default()
                },
            ),
            &[
                "service \"api\": default value 8 for property 'cpu' violates constraint '<= 4'",
                "the default is defined at policy.encore:6:10 in rule: service \"api\"",
            ],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "worker".into(),
                ..Default::default()
            },
        );
        assert!(!result.properties.contains_key("cpu"));
    }

    #[test]
    fn eval_default_must_satisfy_other_rules_constraints() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: >= 2\n}\nfor service if team == \"payments\" {\n    cpu: default 1\n}\n",
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
            &[
                "default value 1 for property 'cpu' violates constraint '>= 2'",
                "the default is defined at",
            ],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "development"), ("team", "payments")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
    }

    #[test]
    fn eval_specificity_chain() {
        let rs = parse_set(
            "\nfor service {\n    cpu: default 0.5\n}\nfor service if env.type == \"production\" {\n    cpu: default 1\n}\nfor service if env.type == \"production\" && team == \"payments\" {\n    cpu: default 2\n}\nservice \"api\" if env.type == \"production\" && team == \"payments\" {\n    cpu: default 3\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production"), ("team", "payments")]);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: attrs(),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(3.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "worker".into(),
                attrs: attrs(),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(2.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "worker".into(),
                attrs: str_attrs(&[("env.type", "production"), ("team", "platform")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "worker".into(),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(0.5));
    }

    #[test]
    fn eval_membership_specificity() {
        let rs = parse_set(
            "\nfor service if team in [\"payments\", \"billing\"] {\n    cpu: default 1\n}\nfor service if team == \"payments\" {\n    cpu: default 2\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("team", "payments")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(2.0));

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("team", "billing")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
    }

    #[test]
    fn eval_named_rule_equals_name_selector() {
        let rs = parse_set(
            "\nservice \"api\" {\n    cpu: default 1\n}\nfor service if name == \"api\" {\n    cpu: default 2\n}\n",
        );
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    ..Default::default()
                },
            ),
            &["ambiguous default for property 'cpu'"],
        );
    }

    #[test]
    fn eval_required() {
        let rs = parse_set(
            "\nfor sql_database if env.type == \"production\" {\n    backup_retention: required & >= 30d\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production")]);

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "sql_database".into(),
                    name: "main".into(),
                    attrs: attrs(),
                    ..Default::default()
                },
            ),
            &[
                "sql_database \"main\": property 'backup_retention' is required but not set",
                "set 'backup_retention' on the resource, or add 'default <value>' to a matching rule",
            ],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "sql_database".into(),
                name: "main".into(),
                attrs: attrs(),
                config: cfg(&[("backup_retention", must_parse_quantity("45d"))]),
            },
        );
        assert_value(
            &val(&result, "backup_retention"),
            &must_parse_quantity("45d"),
        );

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "sql_database".into(),
                    name: "main".into(),
                    attrs: attrs(),
                    config: cfg(&[("backup_retention", must_parse_quantity("7d"))]),
                },
            ),
            &["property 'backup_retention' value 7d violates constraint '>= 30d'"],
        );
    }

    #[test]
    fn eval_required_satisfied_by_default() {
        let rs = parse_set(
            "\nfor sql_database if env.type == \"production\" {\n    backup_retention: required & >= 30d\n}\nfor sql_database if env.type == \"production\" {\n    backup_retention: default 30d\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "sql_database".into(),
                name: "main".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        let rp = result.properties.get("backup_retention").unwrap();
        assert_value(&rp.value, &must_parse_quantity("30d"));
        assert_eq!(rp.source, ValueSource::Default);
    }

    #[test]
    fn eval_disjunctions_intersect() {
        let rs = parse_set(
            "\nfor service {\n    region: \"europe-west1\" | \"europe-north1\" | \"us-central1\"\n}\nfor service if env.type == \"production\" {\n    region: \"europe-west1\" | \"europe-north1\"\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production")]);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: attrs(),
                config: cfg(&[("region", string("europe-west1"))]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "region"), &string("europe-west1"));

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    attrs: attrs(),
                    config: cfg(&[("region", string("us-central1"))]),
                },
            ),
            &["property 'region' value \"us-central1\" violates constraint '\"europe-west1\" | \"europe-north1\"'"],
        );
    }

    #[test]
    fn eval_empty_disjunction_intersection() {
        let rs = parse_set(
            "\nfor service {\n    region: \"a\" | \"b\"\n}\nfor service if env.type == \"production\" {\n    region: \"c\" | \"d\"\n}\n",
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
                "impossible constraints for property 'region'",
                "no value satisfies all the allowed-value constraints",
                "'\"a\" | \"b\"' at policy.encore",
                "'\"c\" | \"d\"' at policy.encore",
            ],
        );
    }

    #[test]
    fn eval_impossible_range() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: >= 4\n}\nfor service if team == \"payments\" {\n    cpu: <= 2\n}\n",
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
            &[
                "impossible constraints for property 'cpu'",
                "'>= 4' conflicts with '<= 2'",
                "a rule cannot weaken another rule's constraints",
            ],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                config: cfg(&[("cpu", number(4.0))]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(4.0));
    }

    #[test]
    fn eval_strict_bounds() {
        let rs = parse_set(
            "\nfor service {\n    cpu: > 2\n}\nfor service if env.type == \"production\" {\n    cpu: <= 2\n}\n",
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
            &["'> 2' conflicts with '<= 2'"],
        );

        let rs = parse_set(
            "\nfor service {\n    cpu: >= 2\n}\nfor service if env.type == \"production\" {\n    cpu: <= 2\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert!(!result.properties.contains_key("cpu"));
    }

    #[test]
    fn eval_sizes_and_durations() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    memory: >= 1Gi & <= 8Gi | default 2Gi\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production")]);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: attrs(),
                ..Default::default()
            },
        );
        let rp = result.properties.get("memory").unwrap();
        assert_value(&rp.value, &must_parse_quantity("2Gi"));
        assert_eq!(rp.value.to_string(), "2Gi");

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: attrs(),
                config: cfg(&[("memory", must_parse_quantity("2048Mi"))]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "memory"), &must_parse_quantity("2Gi"));

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    attrs: attrs(),
                    config: cfg(&[("memory", must_parse_quantity("512Mi"))]),
                },
            ),
            &["property 'memory' value 512Mi violates constraint '>= 1Gi'"],
        );
    }

    #[test]
    fn eval_selector_operators() {
        let rs = parse_set(
            "\nfor bucket if tags.data exists {\n    backup_retention: default 7d\n}\nfor service if env.type != \"preview\" {\n    cpu: default 1\n}\nfor service if env.type in [\"production\", \"staging\"] {\n    memory: default 1Gi\n}\n",
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "bucket".into(),
                attrs: str_attrs(&[("tags.data", "customer")]),
                ..Default::default()
            },
        );
        assert_value(
            &val(&result, "backup_retention"),
            &must_parse_quantity("7d"),
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "bucket".into(),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 0);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "cpu"), &number(1.0));
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "preview")]),
                ..Default::default()
            },
        );
        assert!(!result.properties.contains_key("cpu"));
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 0);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "staging")]),
                ..Default::default()
            },
        );
        assert!(result.properties.contains_key("memory"));
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[("env.type", "preview")]),
                ..Default::default()
            },
        );
        assert!(!result.properties.contains_key("memory"));
    }

    #[test]
    fn eval_selector_type_mismatch() {
        let rs = parse_set("\nfor service if replicas == 3 {\n    cpu: default 1\n}\n");
        let err = eval_err(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: cfg(&[("replicas", string("three"))]),
                ..Default::default()
            },
        );
        assert_err_contains(
            &err,
            &[
                "type mismatch in selector condition 'replicas == 3'",
                "attribute 'replicas' of service \"api\" is the string \"three\", not a number",
            ],
        );
    }

    #[test]
    fn eval_property_type_mismatch() {
        let rs = parse_set("\nfor service {\n    cpu: >= 1\n}\n");
        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    config: cfg(&[("cpu", string("high"))]),
                    ..Default::default()
                },
            ),
            &["service \"api\": property 'cpu' has string value \"high\", but the constraint '>= 1' expects a number"],
        );
    }

    #[test]
    fn eval_conflicting_constraint_types_across_rules() {
        let rs = parse_set(
            "\nfor service {\n    limit: >= 1\n}\nfor service if env.type == \"production\" {\n    limit: >= 1Gi\n}\n",
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
                "conflicting types for property 'limit'",
                "'>= 1' is a number constraint, but '>= 1Gi' compares against a size",
            ],
        );
    }

    #[test]
    fn eval_unrelated_config_passes_through() {
        let rs = parse_set("\nfor service {\n    cpu: <= 4\n}\n");
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                config: cfg(&[("custom.setting", string("anything"))]),
                ..Default::default()
            },
        );
        let rp = result.properties.get("custom.setting").unwrap();
        assert_value(&rp.value, &string("anything"));
        assert_eq!(rp.source, ValueSource::Explicit);
    }

    #[test]
    fn eval_no_matching_rules() {
        let rs = parse_set("\nfor bucket {\n    public_access: false\n}\n");
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 0);
        assert!(result.properties.is_empty());
    }

    #[test]
    fn eval_provider_scoped_properties() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" && provider == \"gcp\" && implementation == \"cloud_run\" {\n    provider.gcp.cloud_run.cpu_always_allocated: true\n    provider.gcp.cloud_run.min_instances: >= 1 | default 1\n}\n",
        );
        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[
                    ("env.type", "production"),
                    ("provider", "gcp"),
                    ("implementation", "cloud_run"),
                ]),
                ..Default::default()
            },
        );
        assert_value(
            &val(&result, "provider.gcp.cloud_run.cpu_always_allocated"),
            &boolean(true),
        );
        assert_value(
            &val(&result, "provider.gcp.cloud_run.min_instances"),
            &number(1.0),
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                attrs: str_attrs(&[
                    ("env.type", "production"),
                    ("provider", "gcp"),
                    ("implementation", "gke"),
                ]),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 0);
    }

    #[test]
    fn eval_empty_resource_kind() {
        let rs = parse_set("for service { cpu: default 1 }");
        let err = match rs.evaluate(&Resource::default()) {
            Err(e) => e,
            Ok(_) => panic!("expected error"),
        };
        assert_err_contains(&err, &["resource kind must not be empty"]);
    }

    const COMPLETE_EXAMPLE: &str = r#"
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
"#;

    #[test]
    fn eval_complete_example() {
        let rs = parse_set(COMPLETE_EXAMPLE);
        assert!(rs.validate().is_ok());

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                name: "api".into(),
                attrs: str_attrs(&[
                    ("env.type", "production"),
                    ("team", "payments"),
                    ("provider", "gcp"),
                    ("implementation", "cloud_run"),
                ]),
                config: cfg(&[("instances.max", number(10.0))]),
            },
        );
        assert_eq!(result.matched.len(), 5);

        assert_value(&val(&result, "cpu"), &number(3.0));
        assert_value(&val(&result, "memory"), &must_parse_quantity("2Gi"));
        assert_value(&val(&result, "instances.min"), &number(1.0));
        assert_value(&val(&result, "instances.max"), &number(10.0));
        assert_value(
            &val(&result, "provider.gcp.cloud_run.cpu_always_allocated"),
            &boolean(true),
        );
        assert_value(
            &val(&result, "provider.gcp.cloud_run.min_instances"),
            &number(1.0),
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "sql_database".into(),
                name: "main".into(),
                attrs: str_attrs(&[("env.type", "production")]),
                ..Default::default()
            },
        );
        assert_value(
            &val(&result, "backup_retention"),
            &must_parse_quantity("90d"),
        );
        assert_value(&val(&result, "point_in_time_recovery"), &boolean(true));
        assert_value(&val(&result, "deletion_protection"), &boolean(true));

        assert_err_contains(
            &eval_err(
                &rs,
                &Resource {
                    kind: "sql_database".into(),
                    name: "main".into(),
                    attrs: str_attrs(&[("env.type", "production")]),
                    config: cfg(&[("backup_retention", must_parse_quantity("60d"))]),
                },
            ),
            &["property 'backup_retention' value 60d violates constraint '>= 90d'"],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "bucket".into(),
                name: "uploads".into(),
                attrs: str_attrs(&[("tags.data", "customer")]),
                ..Default::default()
            },
        );
        assert_value(&val(&result, "public_access"), &boolean(false));
        assert_value(&val(&result, "versioning"), &boolean(true));
        assert_value(
            &val(&result, "backup_retention"),
            &must_parse_quantity("30d"),
        );
    }
}
