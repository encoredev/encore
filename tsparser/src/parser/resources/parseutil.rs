use std::collections::{HashMap, HashSet};

use anyhow::Result;
use litparser::LitParser;
use swc_common::errors::HANDLER;
use swc_ecma_ast as ast;
use swc_ecma_visit::VisitWithPath;

use crate::parser::module_loader::Module;
use crate::parser::Range;

pub trait ReferenceParser
where
    Self: Sized,
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>>;
}

pub struct NamedClassResource<Config, const NAME_IDX: usize = 0, const CONFIG_IDX: usize = 1> {
    pub range: Range,
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub bind_name: Option<ast::Ident>,
    pub config: Config,
}

impl<Config: LitParser, const NAME_IDX: usize, const CONFIG_IDX: usize> ReferenceParser
    for NamedClassResource<Config, NAME_IDX, CONFIG_IDX>
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        let res= NamedClassResourceOptionalConfig::<Config, NAME_IDX, CONFIG_IDX>::parse_resource_reference(module, path)?;
        match res {
            None => Ok(None),
            Some(res) => {
                let Some(config) = res.config else {
                    HANDLER.with(|handler| {
                        handler.span_err(res.range.to_span(), "missing required config object");
                    });
                    return Ok(None);
                };

                Ok(Some(Self {
                    range: res.range,
                    constructor_args: res.constructor_args,
                    doc_comment: res.doc_comment,
                    resource_name: res.resource_name,
                    bind_name: res.bind_name,
                    config,
                }))
            }
        }
    }
}

pub struct NamedClassResourceOptionalConfig<
    Config,
    const NAME_IDX: usize = 0,
    const CONFIG_IDX: usize = 1,
> {
    pub range: Range,
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub bind_name: Option<ast::Ident>,
    pub config: Option<Config>,
}

impl<Config: LitParser, const NAME_IDX: usize, const CONFIG_IDX: usize> ReferenceParser
    for NamedClassResourceOptionalConfig<Config, NAME_IDX, CONFIG_IDX>
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        for node in path.iter().rev() {
            match node {
                swc_ecma_visit::AstParentNodeRef::NewExpr(
                    expr,
                    swc_ecma_visit::fields::NewExprField::Callee,
                ) => {
                    let Some(args) = &expr.args else {
                        anyhow::bail!("missing constructor arguments")
                    };

                    let bind_name = extract_bind_name(path)?;
                    let resource_name = extract_resource_name(&args, NAME_IDX)?;
                    let doc_comment = module.preceding_comments(expr.span.lo.into());

                    let config = args
                        .get(CONFIG_IDX)
                        .map(|arg| Config::parse_lit(&arg.expr))
                        .transpose()?;

                    return Ok(Some(Self {
                        range: expr.span.into(),
                        constructor_args: args.clone(),
                        resource_name: resource_name.to_string(),
                        doc_comment,
                        bind_name,
                        config,
                    }));
                }

                _ => {}
            }
        }
        Ok(None)
    }
}

pub struct UnnamedClassResource<Config, const CONFIG_IDX: usize = 0> {
    pub range: Range,
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub bind_name: Option<ast::Ident>,
    pub config: Config,
}

impl<Config: LitParser, const CONFIG_IDX: usize> ReferenceParser
    for UnnamedClassResource<Config, CONFIG_IDX>
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        for node in path.iter().rev() {
            match node {
                swc_ecma_visit::AstParentNodeRef::NewExpr(
                    expr,
                    swc_ecma_visit::fields::NewExprField::Callee,
                ) => {
                    let Some(args) = &expr.args else {
                        anyhow::bail!("missing constructor arguments")
                    };

                    let bind_name = extract_bind_name(path)?;
                    let config = Config::parse_lit(&args[CONFIG_IDX].expr)?;
                    let doc_comment = module.preceding_comments(expr.span.lo.into());

                    return Ok(Some(Self {
                        range: expr.span.into(),
                        constructor_args: args.clone(),
                        doc_comment,
                        bind_name,
                        config,
                    }));
                }

                _ => {}
            }
        }
        Ok(None)
    }
}

pub struct NamedStaticMethod<const NAME_IDX: usize = 0> {
    pub range: Range,
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub bind_name: Option<ast::Ident>,
}

impl<const NAME_IDX: usize> ReferenceParser for NamedStaticMethod<NAME_IDX> {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        for (idx, node) in path.iter().rev().enumerate() {
            match node {
                swc_ecma_visit::AstParentNodeRef::MemberExpr(
                    expr,
                    swc_ecma_visit::fields::MemberExprField::Obj,
                ) => {
                    let ast::MemberProp::Ident(method_name) = &expr.prop else {
                        continue;
                    };
                    if method_name.sym != "named" {
                        continue;
                    }

                    let idx = path.len() - idx - 1;

                    // Make sure the parent is a call expression.
                    // The path goes:
                    // CallExpr -> Callee -> Expr -> MemberExpr
                    // So we want idx-3.
                    let Some(parent) = path.get(idx - 3) else {
                        continue;
                    };
                    let swc_ecma_visit::AstParentNodeRef::CallExpr(
                        call,
                        swc_ecma_visit::fields::CallExprField::Callee,
                    ) = parent
                    else {
                        continue;
                    };

                    let bind_name = extract_bind_name(path)?;
                    let resource_name = extract_resource_name(&call.args, NAME_IDX)?;
                    let doc_comment = module.preceding_comments(call.span.lo.into());

                    return Ok(Some(Self {
                        range: call.span.into(),
                        constructor_args: call.args.clone(),
                        resource_name: resource_name.to_string(),
                        doc_comment,
                        bind_name,
                    }));
                }

                _ => {}
            }
        }
        Ok(None)
    }
}

pub fn extract_resource_name(args: &[ast::ExprOrSpread], idx: usize) -> Result<&str> {
    let val = args.get(idx).ok_or(anyhow::anyhow!(
        "missing resource name as argument[{}]",
        idx
    ))?;
    if val.spread.is_none() {
        if let ast::Expr::Lit(ast::Lit::Str(str)) = val.expr.as_ref() {
            return Ok(str.value.as_ref());
        }
    }

    Err(anyhow::anyhow!(
        "expected string literal as argument[{}]",
        idx
    ))
}

pub fn extract_bind_name(path: &swc_ecma_visit::AstNodePath) -> Result<Option<ast::Ident>> {
    for node in path.iter().rev() {
        match node {
            swc_ecma_visit::AstParentNodeRef::VarDecl(
                var,
                swc_ecma_visit::fields::VarDeclField::Decls(idx),
            ) => {
                let decl = var
                    .decls
                    .get(*idx)
                    .ok_or(anyhow::anyhow!("missing declaration at index {}", idx))?;
                match &decl.name {
                    ast::Pat::Ident(bind_name) => {
                        return Ok(Some(bind_name.id.clone()));
                    }
                    _ => anyhow::bail!("expected identifier as bind name"),
                }
            }
            _ => {}
        }
    }
    Ok(None)
}

pub struct TrackedNames<'a>(HashMap<&'a str, Vec<&'a str>>);

impl<'a> TrackedNames<'a> {
    pub fn new(names: &'a [(&'a str, &'a str)]) -> Self {
        let mut modules = HashMap::new();
        for &(module, name) in names {
            modules.entry(module).or_insert_with(Vec::new).push(name);
        }

        Self(modules)
    }

    pub fn get(&self, module: &str) -> Option<&[&str]> {
        self.0.get(module).map(|v| &v[..])
    }
}

/// Collect the idents matching the given names to track.
fn collect_import_idents<'a>(
    module: &'a Module,
    tracked_names: &'a TrackedNames<'a>,
) -> (HashSet<ast::Id>, HashMap<ast::Id, &'a [&'a str]>) {
    let mut local_names = HashSet::new();
    let mut module_names = HashMap::new();

    for it in &module.ast.body {
        if let ast::ModuleItem::ModuleDecl(ast::ModuleDecl::Import(import)) = it {
            // Is the module in question one we care about?
            let Some(tracked) = tracked_names.get(import.src.value.as_ref()) else {
                continue;
            };

            // Iterate over the specifiers and determine the local idents.
            for spec in &import.specifiers {
                match spec {
                    ast::ImportSpecifier::Named(named) => {
                        // We are importing specific names from the module.
                        // Determine if the name is one we care about.
                        let is_relevant = tracked.iter().any(|t| match named.imported {
                            Some(ast::ModuleExportName::Ident(ref i)) => i.sym.as_ref() == *t,
                            Some(ast::ModuleExportName::Str(_)) => false,
                            None => named.local.sym.as_ref() == *t,
                        });

                        if is_relevant {
                            // The name is one we care about, so add it to the set.
                            local_names.insert(named.local.to_id());
                        }
                    }

                    ast::ImportSpecifier::Default(_) => {
                        // We are importing the default export from the module.

                        // Do we want to handle this? If so we need to identify the
                        // default import when calling this function.
                        // For now, do nothing.
                    }
                    ast::ImportSpecifier::Namespace(namespace) => {
                        // We're importing the module as a namespace ("import * as foo from 'foo'").
                        module_names.insert(namespace.local.to_id(), &tracked[..]);
                    }
                }
            }
        }
    }

    (local_names, module_names)
}

pub fn iter_references<R: ReferenceParser>(
    module: &Module,
    names: &TrackedNames,
) -> impl Iterator<Item = Result<R>> {
    let (local_ids, _module_ids) = collect_import_idents(&module, names);
    let mut visitor = <IterReferenceVisitor<'_, R>>::new(module, local_ids);
    module
        .ast
        .visit_with_path(&mut visitor, &mut Default::default());
    visitor.results.into_iter()
}

struct IterReferenceVisitor<'a, R> {
    module: &'a Module,
    local_ids: HashSet<ast::Id>,
    results: Vec<Result<R>>,
}

impl<'a, R> IterReferenceVisitor<'a, R> {
    fn new(module: &'a Module, local_ids: HashSet<ast::Id>) -> Self {
        Self {
            module,
            local_ids,
            results: Vec::new(),
        }
    }
}

impl<'a, R: ReferenceParser> swc_ecma_visit::VisitAstPath for IterReferenceVisitor<'a, R> {
    fn visit_ident<'ast: 'r, 'r>(
        &mut self,
        n: &'ast ast::Ident,
        path: &mut swc_ecma_visit::AstNodePath<'r>,
    ) {
        if !self.local_ids.contains(&n.to_id()) {
            return;
        };
        // TODO check for module_ids

        // If this is part of an import declaration, ignore it.
        if path
            .kinds()
            .iter()
            .any(|p| matches!(p, swc_ecma_visit::AstParentKind::ImportDecl(_)))
        {
            return;
        }

        match R::parse_resource_reference(self.module, path) {
            Ok(None) => {} // do nothing
            Ok(Some(r)) => self.results.push(Ok(r)),
            Err(e) => self.results.push(Err(e)),
        }
    }
}
