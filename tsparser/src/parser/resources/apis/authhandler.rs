use std::str::FromStr;

use anyhow::Result;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;
use swc_ecma_ast::TsTypeParamInstantiation;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::apis::encoding::{describe_auth_handler, AuthHandlerEncoding};
use crate::parser::resources::parseutil::{
    extract_bind_name, iter_references, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::types::Type;
use crate::parser::{FilePath, Range};

#[derive(Debug, Clone)]
pub struct AuthHandler {
    pub range: Range,
    pub name: String,
    pub service_name: String,
    pub doc: Option<String>,
    pub encoding: AuthHandlerEncoding,
}

pub const AUTHHANDLER_PARSER: ResourceParser = ResourceParser {
    name: "authhandler",
    interesting_pkgs: &[PkgPath("encore.dev/auth")],

    run: |pass| {
        let module = pass.module.clone();

        // TODO handle this in a better way.
        let service_name = match &module.file_path {
            FilePath::Real(ref buf) => buf
                .parent()
                .and_then(|p| p.file_name())
                .and_then(|s| s.to_str()),
            FilePath::Custom(ref str) => {
                anyhow::bail!("unsupported file path for service: {}", str)
            }
        };
        let Some(service_name) = service_name else {
            return Ok(());
        };

        let names = TrackedNames::new(&[("encore.dev/auth", "authHandler")]);

        for r in iter_references::<AuthHandlerLiteral>(&module, &names) {
            let r = r?;
            let request = pass.type_checker.resolve(module.clone(), &r.request)?;
            let response = pass.type_checker.resolve(module.clone(), &r.response)?;

            let object = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &ast::Expr::Ident(r.bind_name.clone()))?;

            let encoding = describe_auth_handler(pass.type_checker.ctx(), request, response)?;

            let resource = Resource::AuthHandler(Lrc::new(AuthHandler {
                range: r.range,
                name: r.endpoint_name,
                service_name: service_name.to_string(),
                doc: r.doc_comment,
                encoding,
            }));

            pass.add_resource(resource.clone());
            pass.add_bind(BindData {
                range: r.range,
                resource,
                object,
                kind: BindKind::Create,
                ident: Some(r.bind_name),
            });
        }
        Ok(())
    },
};

#[derive(Debug)]
struct AuthHandlerLiteral {
    pub range: Range,
    pub doc_comment: Option<String>,
    pub endpoint_name: String,
    pub bind_name: ast::Ident,
    pub request: ast::TsType,
    pub response: ast::TsType,
}

impl ReferenceParser for AuthHandlerLiteral {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        for node in path.iter().rev() {
            match node {
                swc_ecma_visit::AstParentNodeRef::CallExpr(
                    expr,
                    swc_ecma_visit::fields::CallExprField::Callee,
                ) => {
                    let doc_comment = module.preceding_comments(expr.span.lo.into());
                    let Some(bind_name) = extract_bind_name(path)? else {
                        anyhow::bail!("Auth Handler must be bound to a variable")
                    };

                    let Some(handler) = &expr.args.get(0) else {
                        anyhow::bail!("Auth Handler must have a handler function")
                    };
                    let (mut req, mut resp) = parse_auth_handler_signature(&handler.expr)?;

                    if req.is_none() {
                        req = extract_type_param(expr.type_args.as_deref(), 0)?;
                    }
                    if resp.is_none() {
                        resp = extract_type_param(expr.type_args.as_deref(), 1)?;
                    }

                    let Some(req) = req else {
                        anyhow::bail!(
                            "Auth Handler must have an explicitly defined parameter type"
                        );
                    };
                    let Some(resp) = resp else {
                        anyhow::bail!("Auth Handler must have an explicitly defined result type");
                    };

                    return Ok(Some(Self {
                        range: expr.span.into(),
                        doc_comment,
                        endpoint_name: bind_name.sym.to_string(),
                        bind_name,
                        request: req.clone(),
                        response: resp.clone(),
                    }));
                }

                _ => {}
            }
        }
        Ok(None)
    }
}

fn parse_auth_handler_signature(
    expr: &ast::Expr,
) -> Result<(Option<&ast::TsType>, Option<&ast::TsType>)> {
    let (req_param, type_params, return_type) = match expr {
        ast::Expr::Fn(func) => (
            func.function.params.get(0).map(|p| &p.pat),
            func.function.type_params.as_deref(),
            func.function.return_type.as_deref(),
        ),
        ast::Expr::Arrow(arrow) => (
            arrow.params.get(0),
            arrow.type_params.as_deref(),
            arrow.return_type.as_deref(),
        ),
        _ => return Ok((None, None)),
    };

    if type_params.is_some() {
        anyhow::bail!("auth handler cannot have type parameters");
    }

    let req_type = match req_param {
        None => None,
        Some(param) => match &param {
            ast::Pat::Ident(pat) => pat.type_ann.as_deref(),
            ast::Pat::Array(pat) => pat.type_ann.as_deref(),
            ast::Pat::Rest(pat) => pat.type_ann.as_deref(),
            ast::Pat::Object(pat) => pat.type_ann.as_deref(),

            ast::Pat::Assign(_) | ast::Pat::Invalid(_) | ast::Pat::Expr(_) => None,
        },
    };

    let req = req_type.map(|t| t.type_ann.as_ref());
    let resp = return_type.map(|t| t.type_ann.as_ref());

    Ok((req, resp))
}

fn extract_type_param(
    params: Option<&TsTypeParamInstantiation>,
    idx: usize,
) -> Result<Option<&ast::TsType>> {
    let Some(params) = params else {
        return Ok(None);
    };
    let Some(param) = params.params.get(idx) else {
        return Ok(None);
    };
    Ok(Some(param.as_ref()))
}
