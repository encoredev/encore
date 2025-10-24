use std::collections::{HashMap, HashSet};
use std::rc::Rc;

use litparser::{LitParser, ParseResult, ToParseErr};
use swc_common::{Span, Spanned};
use swc_ecma_ast::{self as ast, CallExpr, MemberExpr, NewExpr, TsTypeParamInstantiation};
use swc_ecma_visit::VisitWithPath;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::BindName;
use crate::parser::types::{Basic, Interface, Object, Type, TypeChecker};
use crate::parser::Range;
use litparser::Sp;

pub trait ReferenceParser
where
    Self: Sized,
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>>;
}

pub struct NamedClassResource<Config, const NAME_IDX: usize = 0, const CONFIG_IDX: usize = 1> {
    pub range: Range,
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub bind_name: BindName,
    pub config: Config,
    pub expr: ast::NewExpr,
}

impl<Config, const NAME_IDX: usize, const CONFIG_IDX: usize> Spanned
    for NamedClassResource<Config, NAME_IDX, CONFIG_IDX>
{
    fn span(&self) -> Span {
        self.range.to_span()
    }
}

impl<Config: LitParser, const NAME_IDX: usize, const CONFIG_IDX: usize> ReferenceParser
    for NamedClassResource<Config, NAME_IDX, CONFIG_IDX>
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        let res = match NamedClassResourceOptionalConfig::<Config, NAME_IDX, CONFIG_IDX>::parse_resource_reference(module, path)? {
            None => return Ok(None),
            Some(res) => res,
        };
        let Some(config) = res.config else {
            return Err(res
                .range
                .to_span()
                .parse_err("missing required config object"));
        };

        Ok(Some(Self {
            range: res.range,
            constructor_args: res.constructor_args,
            doc_comment: res.doc_comment,
            resource_name: res.resource_name,
            bind_name: res.bind_name,
            config,
            expr: res.expr,
        }))
    }
}

#[derive(Debug)]
pub struct NamedClassResourceOptionalConfig<
    Config,
    const NAME_IDX: usize = 0,
    const CONFIG_IDX: usize = 1,
> {
    pub range: Range,
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub bind_name: BindName,
    pub config: Option<Config>,
    pub expr: ast::NewExpr,
}

impl<Config, const NAME_IDX: usize, const CONFIG_IDX: usize> Spanned
    for NamedClassResourceOptionalConfig<Config, NAME_IDX, CONFIG_IDX>
{
    fn span(&self) -> Span {
        self.range.to_span()
    }
}

impl<Config: LitParser, const NAME_IDX: usize, const CONFIG_IDX: usize> ReferenceParser
    for NamedClassResourceOptionalConfig<Config, NAME_IDX, CONFIG_IDX>
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        for node in path.iter().rev() {
            if let swc_ecma_visit::AstParentNodeRef::NewExpr(
                expr,
                swc_ecma_visit::fields::NewExprField::Callee,
            ) = node
            {
                let Some(args) = &expr.args else {
                    return Err(expr.span.parse_err("missing constructor arguments"));
                };

                let bind_name = match extract_bind_name(path)? {
                    Some(name) => BindName::Named(name),
                    None => {
                        if is_default_export(path, (*expr).into()) {
                            BindName::DefaultExport
                        } else {
                            BindName::Anonymous
                        }
                    }
                };
                let resource_name = extract_resource_name(expr.span, args, NAME_IDX)?;
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
                    expr: (*expr).to_owned(),
                }));
            }
        }
        Ok(None)
    }
}

pub struct UnnamedClassResource<Config, const CONFIG_IDX: usize = 0> {
    pub range: Range,
    #[allow(dead_code)]
    pub constructor_args: Vec<ast::ExprOrSpread>,
    pub doc_comment: Option<String>,
    pub bind_name: BindName,
    pub config: Config,
}

impl<Config, const CONFIG_IDX: usize> Spanned for UnnamedClassResource<Config, CONFIG_IDX> {
    fn span(&self) -> Span {
        self.range.to_span()
    }
}

impl<Config: LitParser, const CONFIG_IDX: usize> ReferenceParser
    for UnnamedClassResource<Config, CONFIG_IDX>
{
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        for node in path.iter().rev() {
            if let swc_ecma_visit::AstParentNodeRef::NewExpr(
                expr,
                swc_ecma_visit::fields::NewExprField::Callee,
            ) = node
            {
                let Some(args) = &expr.args else {
                    return Err(expr.span.parse_err("missing constructor arguments"));
                };
                let Some(config_arg) = args.get(CONFIG_IDX) else {
                    return Err(expr.span.parse_err("missing config object"));
                };

                let bind_name = match extract_bind_name(path)? {
                    Some(name) => BindName::Named(name),
                    None => {
                        if is_default_export(path, (*expr).into()) {
                            BindName::DefaultExport
                        } else {
                            BindName::Anonymous
                        }
                    }
                };

                let config = Config::parse_lit(&config_arg.expr)?;
                let doc_comment = module.preceding_comments(expr.span.lo.into());

                return Ok(Some(Self {
                    range: expr.span.into(),
                    constructor_args: args.clone(),
                    doc_comment,
                    bind_name,
                    config,
                }));
            }
        }
        Ok(None)
    }
}

pub struct NamedStaticMethod<const NAME_IDX: usize = 0> {
    pub range: Range,
    #[allow(dead_code)]
    pub constructor_args: Vec<ast::ExprOrSpread>,
    #[allow(dead_code)]
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub bind_name: BindName,
}

impl<const NAME_IDX: usize> ReferenceParser for NamedStaticMethod<NAME_IDX> {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        for (idx, node) in path.iter().rev().enumerate() {
            if let swc_ecma_visit::AstParentNodeRef::MemberExpr(
                expr,
                swc_ecma_visit::fields::MemberExprField::Obj,
            ) = node
            {
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

                let bind_name = match extract_bind_name(path)? {
                    Some(name) => BindName::Named(name),
                    None => {
                        if is_default_export(path, (*expr).into()) {
                            BindName::DefaultExport
                        } else {
                            BindName::Anonymous
                        }
                    }
                };
                let resource_name = extract_resource_name(call.span, &call.args, NAME_IDX)?;
                let doc_comment = module.preceding_comments(call.span.lo.into());

                return Ok(Some(Self {
                    range: call.span.into(),
                    constructor_args: call.args.clone(),
                    resource_name: resource_name.to_string(),
                    doc_comment,
                    bind_name,
                }));
            }
        }
        Ok(None)
    }
}

/// Extracts the name of a resource.
pub fn extract_resource_name(
    span: swc_common::Span,
    args: &[ast::ExprOrSpread],
    idx: usize,
) -> ParseResult<&str> {
    let Some(val) = args.get(idx) else {
        return Err(span.parse_err(format!("missing resource name as argument[{idx}]")));
    };
    if val.spread.is_none() {
        if let ast::Expr::Lit(ast::Lit::Str(str)) = val.expr.as_ref() {
            return Ok(str.value.as_ref());
        }
    }

    Err(span.parse_err("expected string literal"))
}

pub fn extract_bind_name(path: &swc_ecma_visit::AstNodePath) -> ParseResult<Option<ast::Ident>> {
    for node in path.iter().rev() {
        if let swc_ecma_visit::AstParentNodeRef::VarDecl(
            var,
            swc_ecma_visit::fields::VarDeclField::Decls(idx),
        ) = node
        {
            let Some(decl) = var.decls.get(*idx) else {
                return Err(var
                    .span
                    .parse_err(format!("missing declaration at index {idx}")));
            };
            match &decl.name {
                ast::Pat::Ident(bind_name) => {
                    return Ok(Some(bind_name.id.clone()));
                }
                _ => {
                    return Err(decl.name.parse_err("expected identifier as bind name"));
                }
            }
        }
    }
    Ok(None)
}

pub enum Expr<'a> {
    New(&'a NewExpr),
    Call(&'a CallExpr),
    Member(&'a MemberExpr),
}

impl<'a> From<&'a NewExpr> for Expr<'a> {
    fn from(expr: &'a NewExpr) -> Self {
        Self::New(expr)
    }
}
impl<'a> From<&'a CallExpr> for Expr<'a> {
    fn from(expr: &'a CallExpr) -> Self {
        Self::Call(expr)
    }
}

impl<'a> From<&'a MemberExpr> for Expr<'a> {
    fn from(expr: &'a MemberExpr) -> Self {
        Self::Member(expr)
    }
}

// checks if `expr` is the default export in `path`
pub fn is_default_export(path: &swc_ecma_visit::AstNodePath, expr: Expr) -> bool {
    for node in path.iter().rev() {
        match node {
            swc_ecma_visit::AstParentNodeRef::ExportDefaultExpr(
                swc_ecma_ast::ExportDefaultExpr {
                    expr: exported_expr,
                    ..
                },
                swc_ecma_visit::fields::ExportDefaultExprField::Expr,
            ) => {
                return match expr {
                    Expr::Member(member_expr) => {
                        matches!(**exported_expr, swc_ecma_ast::Expr::Member(ref expr) if expr == member_expr)
                    }
                    Expr::Call(call_expr) => {
                        matches!(**exported_expr, swc_ecma_ast::Expr::Call(ref expr) if expr == call_expr)
                    }
                    Expr::New(new_expr) => {
                        matches!(**exported_expr, swc_ecma_ast::Expr::New(ref expr) if expr == new_expr)
                    }
                }
            }
            _ => continue,
        }
    }
    false
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
                        module_names.insert(namespace.local.to_id(), tracked);
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
) -> impl Iterator<Item = ParseResult<R>> {
    let (local_ids, _module_ids) = collect_import_idents(module, names);
    let mut visitor = <IterReferenceVisitor<'_, R>>::new(module, local_ids);
    module
        .ast
        .visit_with_path(&mut visitor, &mut Default::default());
    visitor.results.into_iter()
}

struct IterReferenceVisitor<'a, R> {
    module: &'a Module,
    local_ids: HashSet<ast::Id>,
    results: Vec<ParseResult<R>>,
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

impl<R: ReferenceParser> swc_ecma_visit::VisitAstPath for IterReferenceVisitor<'_, R> {
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

pub fn extract_type_param(
    params: Option<&TsTypeParamInstantiation>,
    idx: usize,
) -> Option<&ast::TsType> {
    let params = params?;
    let param = params.params.get(idx)?;
    Some(param.as_ref())
}

pub fn resolve_object_for_bind_name(
    type_checker: &TypeChecker,
    module: Rc<Module>,
    bind_name: &BindName,
) -> Option<Rc<Object>> {
    match bind_name {
        BindName::Anonymous => None,
        BindName::DefaultExport => type_checker.resolve_default_export(module.clone()),
        BindName::Named(ref id) => {
            type_checker.resolve_obj(module.clone(), &ast::Expr::Ident(id.clone()))
        }
    }
}

/// Returns the interface if the type resolves to an Interface, otherwise returns None.
pub fn resolve_interface(tc: &TypeChecker, typ: &Sp<Type>) -> Option<Interface> {
    use crate::parser::types::unwrap_promise;

    let span = typ.span();
    let typ = unwrap_promise(tc.state(), typ);
    match typ {
        Type::Basic(Basic::Void) => None,
        Type::Interface(iface) => Some(iface.clone()),
        Type::Named(named) => {
            let underlying = tc.underlying(named.obj.module_id, typ);
            resolve_interface(tc, &Sp::new(span, underlying))
        }
        _ => None,
    }
}

/// Validates that a resource name follows snake_case naming conventions.
///
/// Snake case names must:
/// - Be between 1 and 63 characters long
/// - Start with a lowercase letter
/// - End with a lowercase letter or number
/// - Only contain lowercase letters, numbers, and underscores
/// - Not start with the reserved prefix (if provided)
///
/// Returns `Ok(())` if valid, or an error string if invalid.
pub fn validate_snake_case_name(name: &str, reserved_prefix: Option<&str>) -> Result<(), String> {
    const MAX_LENGTH: usize = 63;

    // Check length
    if name.is_empty() || name.len() > MAX_LENGTH {
        return Err(format!(
            "name must be between 1 and {} characters long (got {})",
            MAX_LENGTH,
            name.len()
        ));
    }

    // Check snake_case format: ^[a-z]([_a-z0-9]*[a-z0-9])?$
    let mut chars = name.chars();

    // First character must be a lowercase letter
    let first = chars.next().unwrap();
    if !first.is_ascii_lowercase() {
        return Err(format!(
            "name must start with a lowercase letter (got '{}')",
            first
        ));
    }

    // If there's only one character, it's valid
    if name.len() == 1 {
        // Check reserved prefix
        if let Some(prefix) = reserved_prefix {
            if name.starts_with(prefix) {
                return Err(format!(
                    "name must not start with reserved prefix '{}' (got '{}')",
                    prefix, name
                ));
            }
        }
        return Ok(());
    }

    // Last character must be lowercase letter or digit
    let last = name.chars().last().unwrap();
    if !last.is_ascii_lowercase() && !last.is_ascii_digit() {
        return Err(format!(
            "name must end with a lowercase letter or digit (got '{}')",
            last
        ));
    }

    // Middle characters must be lowercase letters, digits, or underscores
    for (i, c) in name.chars().enumerate() {
        if !c.is_ascii_lowercase() && !c.is_ascii_digit() && c != '_' {
            return Err(format!(
                "name must only contain lowercase letters, numbers, and underscores (got '{}' at position {})",
                c, i
            ));
        }
    }

    // Check reserved prefix
    if let Some(prefix) = reserved_prefix {
        if name.starts_with(prefix) {
            return Err(format!(
                "name must not start with reserved prefix '{}' (got '{}')",
                prefix, name
            ));
        }
    }

    Ok(())
}
