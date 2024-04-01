use std::collections::HashMap;

use anyhow::Result;
use swc_common::sync::Lrc;
use swc_common::Spanned;
use swc_ecma_ast as ast;
use swc_ecma_visit::fields::{CallExprField, CalleeField, MemberExprField, NewExprField};
use swc_ecma_visit::{AstNodePath, AstParentNodeRef, VisitAstPath, VisitWithPath};

use crate::parser::module_loader::{Module, ModuleId, ModuleLoader};
use crate::parser::resourceparser::bind::Bind;
use crate::parser::resources::{apis, infra, Resource};
use crate::parser::Range;

#[derive(Debug)]
pub struct UsageExpr {
    pub range: Range,
    pub bind: Lrc<Bind>,
    pub kind: UsageExprKind,
}

#[derive(Debug)]
pub enum UsageExprKind {
    /// A field on a resource being accessed.
    FieldAccess(FieldAccess),

    /// A method on a resource being called.
    MethodCall(MethodCall),

    /// A resource being called as a function.
    Callee(Callee),

    /// A resource being passed as a function argument.
    CallArg(CallArg),

    /// A resource passed as a constructor argument.
    ConstructorArg(ConstructorArg),

    /// Any other resource usage.
    Other(Other),
}

#[derive(Debug)]
pub struct MethodCall {
    pub method: ast::Ident,
    _call: ast::CallExpr,
}

#[derive(Debug)]
pub struct FieldAccess {
    pub field: ast::Ident,
}

#[derive(Debug)]
pub struct Callee {
    _call: ast::CallExpr,
}

#[derive(Debug)]
pub struct CallArg {
    pub arg_idx: usize,
    _call: ast::CallExpr,
}

#[derive(Debug)]
pub struct ConstructorArg {
    pub arg_idx: usize,
    _call: ast::NewExpr,
}

#[derive(Debug)]
pub struct Other {
    _enclosing_expr: ast::Expr,
}

pub struct UsageResolver<'a> {
    module_loader: &'a ModuleLoader<'a>,
    resources: &'a [Resource],
    binds_by_module: HashMap<ModuleId, Vec<Lrc<Bind>>>,
}

impl<'a> UsageResolver<'a> {
    pub fn new(
        module_loader: &'a ModuleLoader<'a>,
        resources: &'a [Resource],
        binds: &[Lrc<Bind>],
    ) -> Self {
        let mut resolver = Self {
            module_loader,
            resources,
            binds_by_module: HashMap::new(),
        };

        for b in binds {
            resolver
                .binds_by_module
                .entry(b.module_id)
                .or_insert_with(|| Vec::new())
                .push(b.clone());
        }

        resolver
    }

    pub fn scan_usage_exprs(&self, module: &Module) -> Result<Vec<UsageExpr>> {
        let external = self.external_binds_to_scan_for(module)?;
        let internal = self.internal_binds_to_scan_for(module);
        let combined: Vec<BindToScan> = external.into_iter().chain(internal.into_iter()).collect();

        let mut visitor = UsageVisitor::new(&combined);
        module
            .ast
            .visit_with_path(&mut visitor, &mut Default::default());

        Ok(visitor.usages)
    }

    /// external_binds_to_scan_for computes the external binds to scan for given a module.
    fn external_binds_to_scan_for(&self, module: &Module) -> Result<Vec<BindToScan>> {
        let mut external = Vec::new();

        for imp in module.imports() {
            // Type-only imports don't contribute to usage.
            if imp.type_only {
                continue;
            }

            // Resolve the module
            let resolved_module = self.module_loader.resolve_import(module, &imp.src.value)?;

            let resolved_binds = self.binds_by_module.get(&resolved_module.id);
            for names in &imp.specifiers {
                match names {
                    ast::ImportSpecifier::Named(named) => {
                        // src_name is the original name of the bind in the module it was defined.
                        let src_name: &str = match &named.imported {
                            Some(ast::ModuleExportName::Ident(id)) => &id.sym.as_ref(),
                            Some(ast::ModuleExportName::Str(id)) => &id.value.as_ref(),
                            None => &named.local.sym.as_ref(),
                        };

                        // found_bind is the matching bind in the resolved module, if any.
                        let found_bind = resolved_binds
                            .into_iter()
                            .flatten()
                            .find(|b| b.name.as_ref().is_some_and(|i| i == src_name));

                        if let Some(bind) = found_bind {
                            external.push(BindToScan {
                                bound_name: named.local.to_id(),
                                selector: None,
                                bind: bind.to_owned(),
                            });
                        }
                    }

                    spec @ ast::ImportSpecifier::Default(_)
                    | spec @ ast::ImportSpecifier::Namespace(_) => {
                        let local_name = match &spec {
                            ast::ImportSpecifier::Default(default) => &default.local,
                            ast::ImportSpecifier::Namespace(ns) => &ns.local,
                            _ => unreachable!(),
                        };

                        for bind in resolved_binds.into_iter().flatten() {
                            if let Some(name) = &bind.name {
                                external.push(BindToScan {
                                    bound_name: local_name.to_id(),
                                    selector: Some(name),
                                    bind: bind.to_owned(),
                                });
                            }
                        }
                    }
                }
            }
        }

        Ok(external)
    }

    /// internal_binds_to_scan_for computes the internal binds to scan for given a module.
    fn internal_binds_to_scan_for(&self, module: &Module) -> Vec<BindToScan> {
        let mut internal = Vec::new();

        if let Some(module_binds) = self.binds_by_module.get(&module.id) {
            for b in module_binds {
                if let Some(id) = &b.internal_bound_id {
                    internal.push(BindToScan {
                        bound_name: id.to_owned(),
                        selector: None,
                        bind: b.to_owned(),
                    });
                }
            }
        }

        internal
    }
}

#[derive(Debug)]
pub enum Usage {
    CallEndpoint(apis::api::CallEndpointUsage),
    ReferenceEndpoint(apis::api::ReferenceEndpointUsage),
    PublishTopic(infra::pubsub_topic::PublishUsage),
}

pub struct ResolveUsageData<'a> {
    pub expr: &'a UsageExpr,
    pub resources: &'a [Resource],
}

impl UsageResolver<'_> {
    pub fn resolve_usage(&self, exprs: &[UsageExpr]) -> Result<Vec<Usage>> {
        let mut usages = Vec::new();
        for expr in exprs {
            let data = ResolveUsageData {
                resources: self.resources,
                expr,
            };
            match &expr.bind.resource {
                Resource::APIEndpoint(ep) => {
                    let usage = apis::api::resolve_endpoint_usage(&data, ep.clone())?;
                    usages.push(usage);
                }
                Resource::ServiceClient(client) => {
                    apis::service_client::resolve_service_client_usage(&data, client.clone())?
                        .map(|u| usages.push(u));
                }
                Resource::PubSubTopic(topic) => {
                    infra::pubsub_topic::resolve_topic_usage(&data, topic.clone())?
                        .map(|u| usages.push(u));
                }
                _ => {}
            }
        }

        Ok(usages)
    }
}

#[derive(Debug, PartialEq)]
struct BindToScan<'a> {
    /// The bound name within the module being parsed.
    /// If [selector] is None it's the id of the bind itself (`import { Name } from 'module'`).
    /// Otherwise it's the id of the module (`import module from 'module'`), and the specific
    /// bind's name is found in [selector].
    bound_name: ast::Id,

    /// The selector within the bound name, in the case of module imports.
    selector: Option<&'a str>,

    /// The bind itself.
    bind: Lrc<Bind>,
}

struct UsageVisitor<'a> {
    binds: HashMap<ast::Id, &'a BindToScan<'a>>,
    usages: Vec<UsageExpr>,
}

impl<'a> UsageVisitor<'a> {
    pub fn new(binds: &'a [BindToScan]) -> Self {
        let mut map = HashMap::with_capacity(binds.len());
        for b in binds {
            map.insert(b.bound_name.clone(), b);
        }

        Self {
            binds: map,
            usages: Vec::new(),
        }
    }

    /// Report whether the given id represents the bind definition itself.
    fn is_bind_def(&self, bind: &Bind, id: &ast::Ident) -> bool {
        bind.range.map_or(false, |r| r.contains(&id.span.into()))
    }

    /// Report whether the given path represents an import declaration.
    fn is_import_def(&self, path: &AstNodePath) -> bool {
        for k in path.kinds().iter() {
            if let swc_ecma_visit::AstParentKind::ImportDecl(_) = k {
                return true;
            }
        }
        return false;
    }

    fn classify_usage(&self, bind: Lrc<Bind>, path: &AstNodePath) -> Option<UsageExpr> {
        let idx = path.len() - 1;
        let parent = path.get(idx - 1);
        let grandparent = path.get(idx - 2);

        return match parent {
            Some(AstParentNodeRef::MemberExpr(sel, MemberExprField::Obj)) => {
                // We have a member expression, where the object ("foo" in foo.bar) is the bind.
                // Ensure "bar" is a static identifier and not a private field or a computed property.
                match &sel.prop {
                    ast::MemberProp::PrivateName(_) => {
                        // self.errs.emit(
                        //     private.span.into(),
                        //     "cannot use private member for resource",
                        //     Error,
                        // );
                        None
                    }
                    ast::MemberProp::Computed(_) => {
                        // self.errs.emit(
                        //     computed.span.into(),
                        //     "cannot use computed member for resource",
                        //     Error,
                        // );
                        None
                    }
                    ast::MemberProp::Ident(id) => {
                        // bind.SomeField or bind.SomeField()

                        let call_ref = path.get(idx - 4);
                        if let Some(AstParentNodeRef::CallExpr(call, CallExprField::Callee)) =
                            call_ref
                        {
                            Some(UsageExpr {
                                range: call.span.into(),
                                bind: bind.clone(),
                                kind: UsageExprKind::MethodCall(MethodCall {
                                    _call: (*call).to_owned(),
                                    method: id.to_owned(),
                                }),
                            })
                        } else {
                            Some(UsageExpr {
                                range: sel.span.into(),
                                bind: bind.clone(),
                                kind: UsageExprKind::FieldAccess(FieldAccess {
                                    field: id.to_owned(),
                                }),
                            })
                        }
                    }
                }
            }

            Some(AstParentNodeRef::Callee(_, CalleeField::Expr)) => {
                // This bind is being called as a function.
                if let Some(AstParentNodeRef::CallExpr(call, _)) = grandparent {
                    Some(UsageExpr {
                        range: call.span.into(),
                        bind: bind.clone(),
                        kind: UsageExprKind::Callee(Callee {
                            _call: (*call).to_owned(),
                        }),
                    })
                } else {
                    // TODO emit error (parent of Callee should always be a Call)
                    None
                }
            }

            _ => {
                match grandparent {
                    // The bind is being passed as an argument to a function.
                    Some(AstParentNodeRef::CallExpr(call, CallExprField::Args(idx))) => {
                        Some(UsageExpr {
                            range: call.span.into(),
                            bind: bind.clone(),
                            kind: UsageExprKind::CallArg(CallArg {
                                _call: (*call).to_owned(),
                                arg_idx: *idx,
                            }),
                        })
                    }

                    Some(AstParentNodeRef::NewExpr(new, NewExprField::Args(idx))) => {
                        Some(UsageExpr {
                            range: new.span.into(),
                            bind: bind.clone(),
                            kind: UsageExprKind::ConstructorArg(ConstructorArg {
                                _call: (*new).to_owned(),
                                arg_idx: *idx,
                            }),
                        })
                    }

                    // Some other expression.
                    _ => {
                        // Find the largest enclosing expression.
                        let enclosing = path.iter().find_map(|node| match node {
                            AstParentNodeRef::Expr(expr, _) => Some(*expr),
                            _ => None,
                        });
                        if let Some(enclosing) = enclosing {
                            Some(UsageExpr {
                                range: enclosing.span().into(),
                                bind: bind.clone(),
                                kind: UsageExprKind::Other(Other {
                                    _enclosing_expr: enclosing.to_owned(),
                                }),
                            })
                        } else {
                            None
                        }
                    }
                }
            }
        };
    }
}

impl VisitAstPath for UsageVisitor<'_> {
    fn visit_ident<'ast: 'r, 'r>(&mut self, n: &'ast ast::Ident, path: &mut AstNodePath<'r>) {
        if let Some(b) = self.binds.get(&n.to_id()) {
            // If this ident represents the bind's definition itself, ignore it.
            if self.is_bind_def(&b.bind, n) {
                return;
            }

            // If this ident is part of an import declaration we don't consider that a usage.
            if self.is_import_def(path) {
                return;
            }

            if let Some(u) = self.classify_usage(b.bind.clone(), path) {
                self.usages.push(u);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use assert_fs::fixture::PathChild;
    use assert_fs::TempDir;
    use std::path::PathBuf;

    use assert_matches::assert_matches;
    use swc_common::{Globals, GLOBALS};

    use crate::parser::parser::ParseContext;
    use crate::parser::resourceparser::bind::BindKind;
    use crate::parser::resources::apis::api::{Endpoint, Method, Methods};
    use crate::parser::resources::apis::encoding::{
        EndpointEncoding, RequestEncoding, ResponseEncoding,
    };
    use crate::parser::resources::Resource;
    use crate::parser::respath::Path;
    use crate::testutil::testresolve::TestResolver;
    use crate::testutil::JS_RUNTIME_PATH;

    use super::*;

    #[test]
    fn test_scan_external_binds() {
        let globals = Globals::new();
        GLOBALS.set(&globals, || {
            let ar = txtar::from_str(
                "
-- foo.ts --
import { Bar } from './bar.ts';
-- bar.ts --
export const Bar = 5;
        ",
            );

            let base = PathBuf::from("/dummy");
            let resolver = Box::new(TestResolver::new(&base, &ar));
            let tmp = TempDir::new().unwrap();
            let app_root = tmp.child("app_root").to_path_buf();
            let pc = ParseContext::with_resolver(app_root, &JS_RUNTIME_PATH, resolver).unwrap();
            let mods = pc.loader.load_archive(&base, &ar).unwrap();

            let foo_mod = mods.get(&"/dummy/foo.ts".into()).unwrap();
            let bar_mod = mods.get(&"/dummy/bar.ts".into()).unwrap();

            let res = Resource::APIEndpoint(Lrc::new(Endpoint {
                range: Default::default(),
                service_name: "svc".into(),
                name: "Bar".into(),
                doc: None,
                expose: true,
                require_auth: false,
                encoding: EndpointEncoding {
                    default_method: Method::Post,
                    methods: Methods::Some(vec![Method::Post]),
                    req: vec![RequestEncoding {
                        methods: Methods::Some(vec![Method::Post]),
                        params: vec![],
                    }],
                    resp: ResponseEncoding { params: vec![] },
                    path: Path::parse("/svc.Bar", Default::default()).unwrap(),
                    raw_req_schema: None,
                    raw_resp_schema: None,
                },
            }));

            let bar_binds = vec![Lrc::new(Bind {
                kind: BindKind::Create,
                object: None,
                id: 1.into(),
                range: None,
                name: Some("Bar".into()),
                resource: res.clone(),
                internal_bound_id: None,
                module_id: bar_mod.id,
            })];

            let resources = [res];
            let ur = UsageResolver::new(&pc.loader, &resources, &bar_binds);

            let result = ur.external_binds_to_scan_for(foo_mod).unwrap();
            assert_eq!(result.len(), 1);
            assert_eq!(result[0].bind, bar_binds[0]);
        });
    }

    #[test]
    fn test_scan_usage() {
        let globals = Globals::new();
        GLOBALS.set(&globals, || {
            let ar = txtar::from_str(
                "
-- foo.ts --
import { Bar } from './bar.ts';

Bar.field;      // FieldAccess
Bar.method();   // MethodCall
Bar();          // Callee
foo(x, Bar)     // CallArg
new Class(Bar); // ConstructorArg
let foo = Bar;  // Other
-- bar.ts --
export const Bar = 5;
            ",
            );

            let base = PathBuf::from("/dummy");
            let resolver = Box::new(TestResolver::new(&base, &ar));
            let tmp = TempDir::new().unwrap();
            let app_root = tmp.child("app_root").to_path_buf();
            let pc = ParseContext::with_resolver(app_root, &JS_RUNTIME_PATH, resolver).unwrap();
            let mods = pc.loader.load_archive(&base, &ar).unwrap();

            let foo_mod = mods.get(&"/dummy/foo.ts".into()).unwrap();
            let bar_mod = mods.get(&"/dummy/bar.ts".into()).unwrap();

            let res = Resource::APIEndpoint(Lrc::new(Endpoint {
                range: Default::default(),
                name: "Bar".to_string(),
                service_name: "svc".to_string(),
                doc: None,
                expose: true,
                require_auth: false,
                encoding: EndpointEncoding {
                    default_method: Method::Post,
                    methods: Methods::Some(vec![Method::Post]),
                    req: vec![RequestEncoding {
                        methods: Methods::Some(vec![Method::Post]),
                        params: vec![],
                    }],
                    resp: ResponseEncoding {
                        params: vec![],
                    },
                    path: Path::parse("/svc.Bar", Default::default()).unwrap(),
                    raw_req_schema: None,
                    raw_resp_schema: None,
                },
            }));
            let bar_binds = vec![Lrc::new(Bind {
                kind: BindKind::Create,
                object: None,
                id: 1.into(),
                range: None,
                name: Some("Bar".into()),
                resource: res.clone(),
                internal_bound_id: None,
                module_id: bar_mod.id,
            })];

            let resources = [res];
            let ur = UsageResolver::new(&pc.loader, &resources, &bar_binds);

            let usages = ur.scan_usage_exprs(foo_mod).unwrap();
            assert_eq!(usages.len(), 6);

            assert_matches!(&usages[0].kind, UsageExprKind::FieldAccess(field) if field.field.as_ref() == "field");
            assert_matches!(&usages[1].kind, UsageExprKind::MethodCall(method) if method.method.as_ref() == "method");
            assert_matches!(&usages[2].kind, UsageExprKind::Callee(_));
            assert_matches!(&usages[3].kind, UsageExprKind::CallArg(arg) if arg.arg_idx == 1);
            assert_matches!(&usages[4].kind, UsageExprKind::ConstructorArg(arg) if arg.arg_idx == 0);
            assert_matches!(&usages[5].kind, UsageExprKind::Other(_));
        });
    }
}
