use std::collections::{HashMap, HashSet};
use std::rc::Rc;

use crate::ast::{rule_src, Rule};
use crate::diagnostic::{ErrorList, RelatedInfo};
use crate::eval::{check_value, EvalResult, Evaluator, Resource, RuleSet, ValueSource};
use crate::position::Position;
use crate::value::{go_quote, normalize_dynamic_name, Value, ValueKind};

type ResourceKey = (String, String);

/// A managed infrastructure resource instantiated by a static named block of a
/// managed kind whose selector matches an environment.
pub struct Definition {
    pub kind: String,
    pub name: String,
    /// the (first) named block that instantiated the resource
    pub rule: Rc<Rule>,
}

/// The outcome of evaluating all resources of an environment together.
pub struct EnvResult {
    /// one result per evaluated resource: the input resources in their given
    /// order, followed by resources instantiated by static managed blocks and
    /// dynamic blocks that were not in the input.
    pub results: Vec<EvalResult>,
}

impl EnvResult {
    /// Returns the result for the resource with the given kind and name.
    pub fn get(&self, kind: &str, name: &str) -> Option<&EvalResult> {
        self.results
            .iter()
            .find(|r| r.resource.kind == kind && r.resource.name == name)
    }
}

impl RuleSet {
    /// Returns the managed infrastructure resources instantiated by static
    /// named blocks for an environment with the given selector attributes.
    pub fn definitions(
        &self,
        attrs: &HashMap<String, Value>,
    ) -> Result<Vec<Definition>, ErrorList> {
        let (defs, mut diags) = self.definitions_internal(attrs);
        if !diags.is_empty() {
            diags.sort();
            return Err(diags);
        }
        Ok(defs)
    }

    fn definitions_internal(&self, attrs: &HashMap<String, Value>) -> (Vec<Definition>, ErrorList) {
        let mut defs = Vec::new();
        let mut diags = ErrorList::new();
        let mut seen: HashSet<ResourceKey> = HashSet::new();
        for r in self.rules_iter() {
            if r.name.is_empty() || !self.is_managed(&r.kind) {
                continue;
            }
            let res = Resource {
                kind: r.kind.clone(),
                name: r.name.clone(),
                attrs: attrs.clone(),
                config: HashMap::new(),
            };
            let mut ev = Evaluator::new(self, &res);
            if ev.matches(r) {
                let key = (r.kind.clone(), r.name.clone());
                if !seen.contains(&key) {
                    seen.insert(key);
                    defs.push(Definition {
                        kind: r.kind.clone(),
                        name: r.name.clone(),
                        rule: r.clone(),
                    });
                }
            }
            diags.extend(ev.into_diags());
        }
        (defs, diags)
    }

    /// Evaluates all resources of an environment together.
    pub fn evaluate_env(
        &self,
        env_attrs: &HashMap<String, Value>,
        resources: &[Resource],
    ) -> Result<EnvResult, ErrorList> {
        let mut diags = ErrorList::new();

        // Stage 1: augment input resources with env attributes.
        let mut inputs: Vec<Resource> = Vec::with_capacity(resources.len());
        let mut present: HashSet<ResourceKey> = HashSet::new();
        for res in resources {
            let res = if !env_attrs.is_empty() {
                let mut attrs = env_attrs.clone();
                for (k, v) in &res.attrs {
                    attrs.insert(k.clone(), v.clone());
                }
                let mut cp = res.clone();
                cp.attrs = attrs;
                cp
            } else {
                res.clone()
            };
            present.insert((res.kind.clone(), res.name.clone()));
            inputs.push(res);
        }
        let mut all: Vec<Resource> = inputs.clone();

        // Stage 2: instantiate static managed named blocks.
        let (defs, def_diags) = self.definitions_internal(env_attrs);
        diags.extend(def_diags);
        for def in &defs {
            let key = (def.kind.clone(), def.name.clone());
            if !present.contains(&key) {
                present.insert(key);
                all.push(Resource {
                    kind: def.kind.clone(),
                    name: def.name.clone(),
                    attrs: env_attrs.clone(),
                    config: HashMap::new(),
                });
            }
        }

        // Stage 3: fire dynamic blocks against the input resources.
        let (new_res, overlay, dyn_diags) =
            self.fire_dynamic_blocks(env_attrs, &inputs, &mut present);
        diags.extend(dyn_diags);
        all.extend(new_res);

        // Stage 4: evaluate every resource.
        let mut results: Vec<EvalResult> = Vec::with_capacity(all.len());
        let mut by_key: HashMap<ResourceKey, usize> = HashMap::new();
        for res in &all {
            let (result, rdiags) = self.evaluate_internal(res, &overlay);
            if !rdiags.is_empty() {
                diags.extend(rdiags);
                continue;
            }
            let key = (res.kind.clone(), res.name.clone());
            let idx = results.len();
            results.push(result.unwrap());
            by_key.entry(key).or_insert(idx);
        }

        // Stage 5: resolve and check references.
        let mut ref_diags = ErrorList::new();
        for ri in 0..results.len() {
            let refs = results[ri].references.clone();
            for rr in refs {
                self.check_ref(&results[ri], rr, &by_key, &results, &mut ref_diags);
            }
        }
        diags.extend(ref_diags);

        if !diags.is_empty() {
            diags.sort();
            return Err(diags);
        }
        Ok(EnvResult { results })
    }

    fn fire_dynamic_blocks(
        &self,
        env_attrs: &HashMap<String, Value>,
        inputs: &[Resource],
        present: &mut HashSet<ResourceKey>,
    ) -> (Vec<Resource>, Vec<Rc<Rule>>, ErrorList) {
        let mut new_res = Vec::new();
        let mut overlay = Vec::new();
        let mut diags = ErrorList::new();
        let mut collisions: HashMap<ResourceKey, String> = HashMap::new();
        let mut cloned: HashSet<(usize, String)> = HashSet::new();

        for parent in self.rules_iter() {
            if parent.blocks.is_empty() {
                continue;
            }
            for input in inputs {
                let mut ev = Evaluator::new(self, input);
                if !ev.matches(parent) {
                    diags.extend(ev.into_diags());
                    continue;
                }
                for b in &parent.blocks {
                    if !ev.where_matches(b) {
                        continue;
                    }
                    let (name, raw, ok) = ev.block_name(b, &mut diags);
                    if !ok {
                        continue;
                    }
                    let key = (b.kind.clone(), name.clone());
                    if let Some(prev) = collisions.get(&key) {
                        if *prev != raw {
                            let src = b.src.clone();
                            let msg = format!(
                                "dynamic block names {} and {} both normalize to {} {}",
                                go_quote(prev),
                                go_quote(&raw),
                                b.kind,
                                go_quote(&name)
                            );
                            let d =
                                diags.add(src, b.dyn_expr_pos.clone(), b.dyn_expr_end.clone(), msg);
                            d.hint = "give the resources distinct names that do not collide after normalization".to_string();
                            continue;
                        }
                    }
                    collisions.insert(key.clone(), raw.clone());

                    if self.is_managed(&b.kind) && !present.contains(&key) {
                        present.insert(key.clone());
                        new_res.push(Resource {
                            kind: b.kind.clone(),
                            name: name.clone(),
                            attrs: env_attrs.clone(),
                            config: HashMap::new(),
                        });
                    }
                    let ck = (Rc::as_ptr(b) as usize, name.clone());
                    if !cloned.contains(&ck) {
                        cloned.insert(ck);
                        overlay.push(as_named(b, &name));
                    }
                }
                diags.extend(ev.into_diags());
            }
        }
        (new_res, overlay, diags)
    }

    /// Checks one resolved reference against the environment.
    fn check_ref(
        &self,
        result: &EvalResult,
        rr: crate::eval::ResolvedRef,
        by_key: &HashMap<ResourceKey, usize>,
        results: &[EvalResult],
        diags: &mut ErrorList,
    ) {
        let res = &result.resource;
        let src = rule_src(rr.rule.as_ref());
        let (ref_pos, ref_end) = match &rr.prop {
            Some(p) => (p.pos.clone(), p.path_end.clone()),
            None => (Position::default(), Position::default()),
        };
        let rule_detail: Vec<String> = match &rr.rule {
            Some(rule) => vec![
                format!("the constraint is defined at {} in rule:", rule.pos),
                format!("  {}", rule.header()),
            ],
            None => Vec::new(),
        };

        if !rr.unresolved.is_empty() {
            let msg = format!(
                "{}: cannot resolve the reference for property '{}': {}",
                res.describe(),
                rr.path,
                rr.unresolved
            );
            let d = diags.add(src, ref_pos, ref_end, msg);
            d.detail = rule_detail;
            return;
        }
        if rr.target_name.is_empty() {
            let kind = if rr.target_kind.is_empty() {
                "resource"
            } else {
                rr.target_kind.as_str()
            };
            let msg = format!(
                "{}: property '{}' is not set, but a constraint applies to the referenced {}",
                res.describe(),
                rr.path,
                kind
            );
            let d = diags.add(src, ref_pos, ref_end, msg);
            d.detail = rule_detail;
            d.hint = format!(
                "set '{}' on the resource or add a default to a matching rule",
                rr.path
            );
            return;
        }
        let target = match by_key.get(&(rr.target_kind.clone(), rr.target_name.clone())) {
            Some(&idx) => &results[idx],
            None => {
                let msg = format!(
                    "{}: property '{}' references {} {}, but no such resource exists in the environment",
                    res.describe(), rr.path, rr.target_kind, go_quote(&rr.target_name)
                );
                let d = diags.add(src, ref_pos, ref_end, msg);
                d.detail = rule_detail;
                if self.is_managed(&rr.target_kind) {
                    d.hint = format!(
                        "instantiate it with: {} {} {{ ... }}",
                        rr.target_kind,
                        go_quote(&rr.target_name)
                    );
                } else {
                    d.hint = format!(
                        "no {} named {} exists in the application",
                        rr.target_kind,
                        go_quote(&rr.target_name)
                    );
                }
                return;
            }
        };

        let obj = match &rr.object {
            Some(o) => o,
            None => return, // identity-only entry
        };

        let target_desc = format!("{} {}", rr.target_kind, go_quote(&rr.target_name));
        for p in &obj.props {
            let tv = match target.properties.get(&p.path) {
                Some(tv) => tv,
                None => {
                    let mut detail = rule_detail.clone();
                    detail.push(format!("      {}: {{ {} }}", rr.path, p));
                    let msg = format!(
                        "{}: the referenced {} does not set property '{}', which the constraint needs",
                        res.describe(), target_desc, p.path
                    );
                    let d = diags.add(src.clone(), p.pos.clone(), p.path_end.clone(), msg);
                    d.detail = detail;
                    d.hint = format!(
                        "set '{}' on the {}, or give it a default in a rule for that kind",
                        p.path, rr.target_kind
                    );
                    continue;
                }
            };
            let sv = match p.scalar() {
                Some(s) => s,
                None => continue,
            };
            if sv.constraint.is_none() || tv.reference.is_some() {
                continue;
            }
            let constraint = sv.constraint.as_ref().unwrap();
            let (ok, fail, mismatch) = check_value(&tv.value, constraint);
            if let Some(mismatch) = mismatch {
                let msg = format!(
                    "{}: property '{}' of the referenced {} has {} value {}, but the constraint '{}' expects a {}",
                    res.describe(), p.path, target_desc, tv.value.kind, tv.value, mismatch, mismatch.value.kind
                );
                let d = diags.add(src.clone(), mismatch.pos.clone(), mismatch.end.clone(), msg);
                d.detail = rule_detail.clone();
            } else if !ok {
                let fail = fail.unwrap();
                let span = fail.span();
                let msg = format!(
                    "{}: the referenced {} has '{}' = {}, violating the constraint '{}'",
                    res.describe(),
                    target_desc,
                    p.path,
                    tv.value,
                    fail
                );
                let d = diags.add(src.clone(), span.start, span.end, msg);
                d.detail = rule_detail.clone();
                if tv.source == ValueSource::Default {
                    if let Some(dr) = &tv.default_rule {
                        d.related.push(RelatedInfo {
                            pos: dr.pos.clone(),
                            message: format!(
                                "the value comes from a default in rule at {}: {}",
                                dr.pos,
                                dr.header()
                            ),
                        });
                    }
                }
            }
        }
    }
}

impl<'a> Evaluator<'a> {
    /// Reports whether a nested block's own selector matches the resource the
    /// enclosing rule iterates over.
    fn where_matches(&mut self, b: &Rc<Rule>) -> bool {
        for c in &b.wheres {
            if !self.eval_cond(b, c) {
                return false;
            }
        }
        true
    }

    /// Resolves the resource name a nested block instantiates for the resource
    /// the evaluator iterates over, returning the normalized name and the
    /// source value.
    fn block_name(&self, b: &Rc<Rule>, diags: &mut ErrorList) -> (String, String, bool) {
        if b.dyn_expr.is_empty() {
            return (b.name.clone(), b.name.clone(), true); // static nested block
        }
        let (v, found) = self.lookup_field(&b.dyn_expr);
        if !found {
            let src = b.src.clone();
            let msg = format!(
                "{}: dynamic block attribute '{}' is not set",
                self.res.describe(),
                b.dyn_expr
            );
            let d = diags.add(src, b.dyn_expr_pos.clone(), b.dyn_expr_end.clone(), msg);
            d.hint =
                "the block is only instantiated for resources if the attribute is set".to_string();
            return (String::new(), String::new(), false);
        }
        if v.kind != ValueKind::String {
            let src = b.src.clone();
            let msg = format!(
                "{}: dynamic block attribute '{}' is the {} {}, not a string",
                self.res.describe(),
                b.dyn_expr,
                v.kind,
                v
            );
            diags.add(src, b.dyn_expr_pos.clone(), b.dyn_expr_end.clone(), msg);
            return (String::new(), String::new(), false);
        }
        match normalize_dynamic_name(&v.str) {
            Ok(norm) => (norm, v.str.clone(), true),
            Err(e) => {
                let src = b.src.clone();
                let msg = format!("{}: {}", self.res.describe(), e);
                diags.add(src, b.dyn_expr_pos.clone(), b.dyn_expr_end.clone(), msg);
                (String::new(), String::new(), false)
            }
        }
    }
}

/// Clones a dynamic (or static nested) block into a named rule for the resource
/// it instantiates, so the existing matching, specificity, and merge logic
/// applies. The block's own selector is dropped.
fn as_named(b: &Rc<Rule>, name: &str) -> Rc<Rule> {
    Rc::new(Rule {
        pos: b.pos.clone(),
        kind: b.kind.clone(),
        kind_pos: b.kind_pos.clone(),
        kind_end: b.kind_end.clone(),
        name: name.to_string(),
        name_pos: b.name_pos.clone(),
        dyn_expr: String::new(),
        dyn_expr_pos: Position::default(),
        dyn_expr_end: Position::default(),
        wheres: Vec::new(),
        props: b.props.clone(),
        blocks: b.blocks.clone(),
        src: b.src.clone(),
    })
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use super::{EnvResult, RuleSet};
    use crate::ast::{File, RefMode};
    use crate::diagnostic::ErrorList;
    use crate::eval::{Resource, ValueSource};
    use crate::parser::parse_file;
    use crate::testutil::{assert_err_contains, assert_value, eval_ok, parse_set, str_attrs};
    use crate::value::{boolean, must_parse_quantity, number, string, Value};

    fn parse_ok(src: &str) -> File {
        let pr = parse_file("p.encore", src);
        assert!(pr.errors.is_empty(), "unexpected errors:\n{}", pr.errors);
        pr.file
    }

    fn cfg(pairs: &[(&str, Value)]) -> HashMap<String, Value> {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.clone()))
            .collect()
    }

    fn res(kind: &str, name: &str) -> Resource {
        Resource {
            kind: kind.into(),
            name: name.into(),
            ..Default::default()
        }
    }

    fn env_err(rs: &RuleSet, attrs: &HashMap<String, Value>, resources: &[Resource]) -> ErrorList {
        match rs.evaluate_env(attrs, resources) {
            Ok(_) => panic!("expected env error, got ok"),
            Err(e) => e,
        }
    }

    fn env_ok(rs: &RuleSet, attrs: &HashMap<String, Value>, resources: &[Resource]) -> EnvResult {
        match rs.evaluate_env(attrs, resources) {
            Ok(r) => r,
            Err(e) => panic!("unexpected env error:\n{e}"),
        }
    }

    #[test]
    fn parse_named_managed_block() {
        let src = "\nsql_cluster \"main\" if env.type == \"production\" {\n    engine: \"postgres\"\n    version: \"16\"\n    cpu: >= 2 & <= 16 | default 4\n}\n";
        let f = parse_ok(src);
        assert_eq!(f.rules.len(), 1);
        let r = &f.rules[0];
        assert_eq!(r.kind, "sql_cluster");
        assert_eq!(r.name, "main");
        assert_eq!(
            r.header(),
            "sql_cluster \"main\" if env.type == \"production\""
        );
        assert_eq!(r.props.len(), 3);
    }

    #[test]
    fn parse_define_removed() {
        let pr = parse_file(
            "p.encore",
            "define sql_cluster \"main\" {\n    engine: \"postgres\"\n}\n",
        );
        assert_err_contains(
            &pr.errors,
            &[
                "the 'define' keyword has been removed",
                "declare managed resources directly, e.g.: sql_cluster \"main\" { ... }",
            ],
        );
    }

    #[test]
    fn parse_object_constraint() {
        let src = "\nfor sql_database if tags.data == \"customer\" {\n    cluster: {\n        backup_retention: >= 30d\n        point_in_time_recovery: true\n    }\n}\n";
        let f = parse_ok(src);
        let r = &f.rules[0];
        assert_eq!(r.props.len(), 1);
        let rv = r.props[0].ref_value().unwrap();
        assert!(rv.reference.is_none());
        let obj = rv.object.as_ref().unwrap();
        assert_eq!(obj.props.len(), 2);
        assert_eq!(obj.props[0].to_string(), "backup_retention: >= 30d");
        assert_eq!(obj.props[1].to_string(), "point_in_time_recovery: true");
    }

    #[test]
    fn parse_reference_values() {
        let src = "\nsql_database \"audit\" {\n    cluster: sql_cluster.audit & {\n        backup_retention: >= 90d\n    }\n}\nfor sql_database {\n    cluster: default sql_cluster.main\n}\nfor sql_database if tags.domain exists {\n    cluster: default sql_cluster[tags.domain]\n}\n";
        let f = parse_ok(src);

        let audit = f.rules[0].props[0].ref_value().unwrap();
        let r = audit.reference.as_ref().unwrap();
        assert_eq!(r.mode, RefMode::Static);
        assert_eq!(r.kind, "sql_cluster");
        assert_eq!(r.name, "audit");
        assert!(audit.object.is_some());

        let def = f.rules[1].props[0].ref_value().unwrap();
        assert_eq!(
            def.default.as_ref().unwrap().reference.to_string(),
            "sql_cluster.main"
        );

        let dyn_ = f.rules[2].props[0].ref_value().unwrap();
        let dref = &dyn_.default.as_ref().unwrap().reference;
        assert_eq!(dref.mode, RefMode::Dynamic);
        assert_eq!(dref.to_string(), "sql_cluster[tags.domain]");
    }

    #[test]
    fn parse_require_removed() {
        let pr = parse_file(
            "p.encore",
            "for sql_database {\n    require cluster {\n        backup_retention: >= 30d\n    }\n}\n",
        );
        assert_err_contains(
            &pr.errors,
            &[
                "the 'require' block has been removed",
                "constrain a referenced resource with nested object syntax",
            ],
        );
    }

    #[test]
    fn parse_object_constraint_errors() {
        let cases: &[(&str, &[&str])] = &[
            (
                "for sql_database {\n    cluster: {\n        backup_retention: >= 30d | default 30d\n    }\n}\n",
                &["'default' is not allowed inside an object constraint"],
            ),
            (
                "for sql_database {\n    cluster: sql_cluster.main & >= 5\n}\n",
                &["a reference cannot be combined with scalar constraints"],
            ),
            (
                "for sql_database {\n    cluster: sql_cluster.main & sql_cluster.audit\n}\n",
                &["a property cannot have more than one reference"],
            ),
        ];
        for (src, want) in cases {
            let pr = parse_file("policy.encore", src);
            assert!(!pr.errors.is_empty(), "src: {src:?}");
            assert_err_contains(&pr.errors, want);
        }
    }

    #[test]
    fn validate_object_constraint() {
        let rs = parse_set("for sql_database {\n    cluster: >= 5\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &["property 'cluster' is a reference and cannot take a scalar constraint"],
        );

        let rs = parse_set("for sql_database {\n    cluster: default service_instance.main\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &["property 'cluster' references service_instance, but it must reference a sql_cluster"],
        );

        let rs = parse_set("for service {\n    cpu: sql_cluster.main\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &["property 'cpu' of service is not a reference property"],
        );

        let rs = parse_set(
            "for sql_database {\n    cluster: {\n        cpu: >= 1\n        cpu: <= 4\n    }\n}\n",
        );
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &["duplicate property 'cpu' in the same object constraint"],
        );

        let rs =
            parse_set("for sql_database {\n    cluster: {\n        cpu: >= 4 & <= 2\n    }\n}\n");
        assert_err_contains(
            &rs.validate().unwrap_err(),
            &[
                "impossible constraints for property 'cpu'",
                "'>= 4' conflicts with '<= 2'",
            ],
        );
    }

    #[test]
    fn eval_managed_block_merging() {
        let rs = parse_set(
            "\nsql_cluster \"main\" if env.type == \"production\" {\n    cpu: >= 2 & <= 16 | default 4\n}\nfor sql_cluster {\n    cpu: <= 8\n}\n",
        );
        let attrs = || str_attrs(&[("env.type", "production")]);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "sql_cluster".into(),
                name: "main".into(),
                attrs: attrs(),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 2);
        assert_value(&result.properties.get("cpu").unwrap().value, &number(4.0));

        assert_err_contains(
            &crate::testutil::eval_err(
                &rs,
                &Resource {
                    kind: "sql_cluster".into(),
                    name: "main".into(),
                    attrs: attrs(),
                    config: cfg(&[("cpu", number(12.0))]),
                },
            ),
            &["property 'cpu' value 12 violates constraint '<= 8'"],
        );

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "sql_cluster".into(),
                name: "other".into(),
                attrs: attrs(),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 1);
    }

    #[test]
    fn definitions() {
        let rs = parse_set(
            "\nsql_cluster \"main\" if env.type == \"production\" {\n    engine: \"postgres\"\n}\nsql_cluster \"audit\" if env.type == \"production\" && tags.compliance exists {\n    engine: \"postgres\"\n}\nsql_cluster \"main\" if env.type == \"production\" && provider == \"gcp\" {\n    cpu: default 8\n}\n",
        );

        let defs = rs
            .definitions(&str_attrs(&[("env.type", "production")]))
            .unwrap();
        assert_eq!(defs.len(), 1);
        assert_eq!(defs[0].kind, "sql_cluster");
        assert_eq!(defs[0].name, "main");
        assert_eq!(
            defs[0].rule.header(),
            "sql_cluster \"main\" if env.type == \"production\""
        );

        let defs = rs
            .definitions(&str_attrs(&[
                ("env.type", "production"),
                ("tags.compliance", "pci"),
            ]))
            .unwrap();
        assert_eq!(defs.len(), 2);

        let defs = rs
            .definitions(&str_attrs(&[("env.type", "development")]))
            .unwrap();
        assert_eq!(defs.len(), 0);

        let rs = parse_set("service \"api\" { cpu: default 2 }");
        let defs = rs.definitions(&HashMap::new()).unwrap();
        assert_eq!(defs.len(), 0);
    }

    #[test]
    fn eval_references_resolved() {
        let rs = parse_set(
            "\nfor sql_database {\n    cluster: sql_cluster.main & {\n        backup_retention: >= 30d\n    }\n}\n",
        );
        let result = eval_ok(&rs, &res("sql_database", "orders"));
        let cluster = result.properties.get("cluster").unwrap();
        let r = cluster.reference.as_ref().unwrap();
        assert_eq!(r.kind, "sql_cluster");
        assert_eq!(r.name, "main");
        assert_eq!(result.references.len(), 2);
    }

    const REALISTIC_EXAMPLE: &str = include_str!("testdata/realistic_example.ecl");

    #[test]
    fn evaluate_env_realistic_example() {
        let rs = parse_set(REALISTIC_EXAMPLE);
        assert!(rs.validate().is_ok());

        let env_attrs = str_attrs(&[("env.type", "production")]);
        let er = env_ok(
            &rs,
            &env_attrs,
            &[
                res("service", "api"),
                res("sql_database", "orders"),
                res("sql_database", "audit"),
                Resource {
                    kind: "sql_database".into(),
                    name: "users".into(),
                    attrs: str_attrs(&[("tags.data", "customer")]),
                    ..Default::default()
                },
                res("bucket", "uploads"),
            ],
        );
        assert_eq!(er.results.len(), 7);

        let main = er.get("sql_cluster", "main").unwrap();
        assert_value(
            &main.properties.get("engine").unwrap().value,
            &string("postgres"),
        );
        assert_value(
            &main.properties.get("version").unwrap().value,
            &string("16"),
        );
        assert_value(&main.properties.get("cpu").unwrap().value, &number(4.0));
        assert_value(
            &main.properties.get("backup_retention").unwrap().value,
            &must_parse_quantity("30d"),
        );
        assert_value(
            &main.properties.get("high_availability").unwrap().value,
            &boolean(true),
        );

        let audit = er.get("sql_cluster", "audit").unwrap();
        assert_value(
            &audit.properties.get("storage").unwrap().value,
            &must_parse_quantity("1Ti"),
        );
        assert_value(
            &audit.properties.get("backup_retention").unwrap().value,
            &must_parse_quantity("90d"),
        );

        assert_eq!(
            er.get("sql_database", "orders")
                .unwrap()
                .properties
                .get("cluster")
                .unwrap()
                .reference
                .as_ref()
                .unwrap()
                .name,
            "main"
        );
        assert_eq!(
            er.get("sql_database", "audit")
                .unwrap()
                .properties
                .get("cluster")
                .unwrap()
                .reference
                .as_ref()
                .unwrap()
                .name,
            "audit"
        );
        assert_eq!(
            er.get("sql_database", "users")
                .unwrap()
                .properties
                .get("cluster")
                .unwrap()
                .reference
                .as_ref()
                .unwrap()
                .name,
            "main"
        );

        let api = er.get("service", "api").unwrap();
        assert_value(&api.properties.get("cpu").unwrap().value, &number(1.0));
        assert_value(
            &api.properties.get("instances.min").unwrap().value,
            &number(1.0),
        );
        assert_value(
            &er.get("bucket", "uploads")
                .unwrap()
                .properties
                .get("public_access")
                .unwrap()
                .value,
            &boolean(false),
        );

        assert!(er.get("sql_cluster", "nope").is_none());
    }

    #[test]
    fn evaluate_env_object_constraint_violation() {
        let rs = parse_set(
            "\nsql_cluster \"main\" if env.type == \"production\" {\n    backup_retention: >= 7d | default 7d\n}\nfor sql_database if env.type == \"production\" {\n    cluster: sql_cluster.main & {\n        backup_retention: >= 30d\n    }\n}\n",
        );
        let attrs = str_attrs(&[("env.type", "production")]);
        assert_err_contains(
            &env_err(&rs, &attrs, &[res("sql_database", "orders")]),
            &[
                "sql_database \"orders\": the referenced sql_cluster \"main\" has 'backup_retention' = 7d, violating the constraint '>= 30d'",
                "the constraint is defined at",
                "the value comes from a default in rule at",
            ],
        );

        let er = env_ok(
            &rs,
            &attrs,
            &[
                res("sql_database", "orders"),
                Resource {
                    kind: "sql_cluster".into(),
                    name: "main".into(),
                    config: cfg(&[("backup_retention", must_parse_quantity("45d"))]),
                    ..Default::default()
                },
            ],
        );
        assert_value(
            &er.get("sql_cluster", "main")
                .unwrap()
                .properties
                .get("backup_retention")
                .unwrap()
                .value,
            &must_parse_quantity("45d"),
        );
    }

    #[test]
    fn evaluate_env_missing_target() {
        let rs = parse_set(
            "\nsql_database \"audit\" {\n    cluster: sql_cluster.audit & {\n        backup_retention: >= 90d\n    }\n}\n",
        );
        assert_err_contains(
            &env_err(&rs, &HashMap::new(), &[res("sql_database", "audit")]),
            &[
                "sql_database \"audit\": property 'cluster' references sql_cluster \"audit\", but no such resource exists in the environment",
                "instantiate it with: sql_cluster \"audit\" { ... }",
            ],
        );
    }

    #[test]
    fn evaluate_env_unset_reference() {
        let rs = parse_set(
            "\nfor sql_database {\n    cluster: {\n        backup_retention: >= 30d\n    }\n}\n",
        );
        assert_err_contains(
            &env_err(&rs, &HashMap::new(), &[res("sql_database", "orders")]),
            &[
                "sql_database \"orders\": property 'cluster' is not set, but a constraint applies to the referenced sql_cluster",
                "set 'cluster' on the resource or add a default to a matching rule",
            ],
        );
    }

    #[test]
    fn evaluate_env_target_missing_property() {
        let rs = parse_set(
            "\nsql_cluster \"main\" {\n    engine: \"postgres\"\n}\nfor sql_database {\n    cluster: sql_cluster.main & {\n        point_in_time_recovery: true\n    }\n}\n",
        );
        assert_err_contains(
            &env_err(&rs, &HashMap::new(), &[res("sql_database", "orders")]),
            &[
                "sql_database \"orders\": the referenced sql_cluster \"main\" does not set property 'point_in_time_recovery', which the constraint needs",
                "set 'point_in_time_recovery' on the sql_cluster",
            ],
        );
    }

    #[test]
    fn evaluate_env_input_overrides_definition() {
        let rs = parse_set(
            "\nsql_cluster \"main\" if env.type == \"production\" {\n    cpu: >= 2 & <= 16 | default 4\n}\n",
        );
        let env_attrs = str_attrs(&[("env.type", "production")]);

        let er = env_ok(
            &rs,
            &env_attrs,
            &[Resource {
                kind: "sql_cluster".into(),
                name: "main".into(),
                config: cfg(&[("cpu", number(8.0))]),
                ..Default::default()
            }],
        );
        assert_eq!(er.results.len(), 1);
        let rp = er
            .get("sql_cluster", "main")
            .unwrap()
            .properties
            .get("cpu")
            .unwrap();
        assert_value(&rp.value, &number(8.0));
        assert_eq!(rp.source, ValueSource::Explicit);

        assert_err_contains(
            &env_err(
                &rs,
                &env_attrs,
                &[Resource {
                    kind: "sql_cluster".into(),
                    name: "main".into(),
                    config: cfg(&[("cpu", number(32.0))]),
                    ..Default::default()
                }],
            ),
            &["property 'cpu' value 32 violates constraint '<= 16'"],
        );
    }

    #[test]
    fn evaluate_env_object_constraints_merge_across_rules() {
        let rs = parse_set(
            "\nsql_cluster \"main\" {\n    backup_retention: >= 7d | default 35d\n    high_availability: false\n}\nfor sql_database {\n    cluster: sql_cluster.main & {\n        backup_retention: >= 30d\n    }\n}\nfor sql_database if tags.data == \"customer\" {\n    cluster: {\n        high_availability: true\n    }\n}\n",
        );
        let err = env_err(
            &rs,
            &HashMap::new(),
            &[Resource {
                kind: "sql_database".into(),
                name: "users".into(),
                attrs: str_attrs(&[("tags.data", "customer")]),
                ..Default::default()
            }],
        );
        assert_err_contains(
            &err,
            &["sql_database \"users\": the referenced sql_cluster \"main\" has 'high_availability' = false, violating the constraint 'true'"],
        );
        assert!(!err.to_string().contains("backup_retention"));

        env_ok(&rs, &HashMap::new(), &[res("sql_database", "orders")]);
    }

    #[test]
    fn evaluate_env_merges_env_attrs() {
        let rs = parse_set(
            "\nfor service if env.type == \"production\" {\n    cpu: default 1\n}\nfor service if env.type == \"production\" && team == \"payments\" {\n    cpu: default 2\n}\n",
        );
        let er = env_ok(
            &rs,
            &str_attrs(&[("env.type", "production")]),
            &[
                Resource {
                    kind: "service".into(),
                    name: "api".into(),
                    attrs: str_attrs(&[("team", "payments")]),
                    ..Default::default()
                },
                res("service", "worker"),
            ],
        );
        assert_value(
            &er.get("service", "api")
                .unwrap()
                .properties
                .get("cpu")
                .unwrap()
                .value,
            &number(2.0),
        );
        assert_value(
            &er.get("service", "worker")
                .unwrap()
                .properties
                .get("cpu")
                .unwrap()
                .value,
            &number(1.0),
        );
    }

    #[test]
    fn evaluate_env_dynamic_blocks() {
        let rs = parse_set(
            "\nfor service if tags.domain exists {\n    instance: default service_instance[tags.domain]\n    service_instance tags.domain {\n        cpu: >= 1 & <= 8 | default 2\n        memory: >= 1Gi & <= 16Gi | default 4Gi\n    }\n}\n",
        );
        assert!(rs.validate().is_ok());

        let er = env_ok(
            &rs,
            &str_attrs(&[("env.type", "production")]),
            &[
                Resource {
                    kind: "service".into(),
                    name: "billing".into(),
                    attrs: str_attrs(&[("tags.domain", "Billing")]),
                    ..Default::default()
                },
                Resource {
                    kind: "service".into(),
                    name: "invoices".into(),
                    attrs: str_attrs(&[("tags.domain", "Billing")]),
                    ..Default::default()
                },
                Resource {
                    kind: "service".into(),
                    name: "search".into(),
                    attrs: str_attrs(&[("tags.domain", "search")]),
                    ..Default::default()
                },
            ],
        );

        assert!(er.get("service_instance", "billing").is_some());
        assert!(er.get("service_instance", "search").is_some());
        assert_eq!(er.results.len(), 5);

        assert_value(
            &er.get("service_instance", "billing")
                .unwrap()
                .properties
                .get("cpu")
                .unwrap()
                .value,
            &number(2.0),
        );

        assert_eq!(
            er.get("service", "billing")
                .unwrap()
                .properties
                .get("instance")
                .unwrap()
                .reference
                .as_ref()
                .unwrap()
                .name,
            "billing"
        );
        assert_eq!(
            er.get("service", "invoices")
                .unwrap()
                .properties
                .get("instance")
                .unwrap()
                .reference
                .as_ref()
                .unwrap()
                .name,
            "billing"
        );
        assert_eq!(
            er.get("service", "search")
                .unwrap()
                .properties
                .get("instance")
                .unwrap()
                .reference
                .as_ref()
                .unwrap()
                .name,
            "search"
        );
    }

    #[test]
    fn evaluate_env_dynamic_name_collision() {
        let rs = parse_set(
            "\nfor service if tags.domain exists {\n    service_instance tags.domain {\n        cpu: default 2\n    }\n}\n",
        );
        assert_err_contains(
            &env_err(
                &rs,
                &HashMap::new(),
                &[
                    Resource {
                        kind: "service".into(),
                        name: "a".into(),
                        attrs: str_attrs(&[("tags.domain", "Billing API")]),
                        ..Default::default()
                    },
                    Resource {
                        kind: "service".into(),
                        name: "b".into(),
                        attrs: str_attrs(&[("tags.domain", "billing-api")]),
                        ..Default::default()
                    },
                ],
            ),
            &[
                "both normalize to service_instance \"billing-api\"",
                "give the resources distinct names that do not collide after normalization",
            ],
        );
    }
}
