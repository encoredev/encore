use litparser::{report_and_continue, ParseResult, ToParseErr};
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;
use swc_ecma_ast::TsTypeParamInstantiation;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::apis::encoding::{describe_auth_handler, AuthHandlerEncoding};
use crate::parser::resources::parseutil::{
    extract_bind_name, iter_references, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::{FilePath, Range};
use crate::span_err::ErrReporter;

use super::encoding::iface_fields;

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

        let service_name = match &pass.service_name {
            Some(name) => Some(name.to_string()),
            None => {
                // TODO handle this in a better way.
                match &module.file_path {
                    FilePath::Real(ref buf) => buf
                        .parent()
                        .and_then(|p| p.file_name())
                        .and_then(|s| s.to_str())
                        .map(|s| s.to_string()),
                    FilePath::Custom(_) => None,
                }
            }
        };

        let names = TrackedNames::new(&[("encore.dev/auth", "authHandler")]);

        'RefLoop: for r in iter_references::<AuthHandlerLiteral>(&module, &names) {
            let r = report_and_continue!(r);
            let Some(service_name) = service_name.as_ref() else {
                module.err("unable to determine service name for file");
                continue;
            };

            let request = pass.type_checker.resolve_type(module.clone(), &r.request);
            let response = pass.type_checker.resolve_type(module.clone(), &r.response);

            let fields = match iface_fields(pass.type_checker, &request) {
                Ok(fields) => fields,
                Err(e) => {
                    e.report();
                    continue;
                }
            };

            for (_, v) in fields {
                if !v.is_custom() {
                    v.range().to_span().err(
                        "authHandler parameter type can only consist of Query and Header fields",
                    );
                    continue 'RefLoop;
                }
            }

            let object = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &ast::Expr::Ident(r.bind_name.clone()));

            let encoding = describe_auth_handler(pass.type_checker.state(), request, response);

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
                resource: ResourceOrPath::Resource(resource),
                object,
                kind: BindKind::Create,
                ident: Some(r.bind_name),
            });
        }
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
    ) -> ParseResult<Option<Self>> {
        for node in path.iter().rev() {
            if let swc_ecma_visit::AstParentNodeRef::CallExpr(
                expr,
                swc_ecma_visit::fields::CallExprField::Callee,
            ) = node
            {
                let doc_comment = module.preceding_comments(expr.span.lo.into());
                let Some(bind_name) = extract_bind_name(path)? else {
                    return Err(expr.parse_err("auth handler must be bound to a variable"));
                };

                let Some(handler) = &expr.args.first() else {
                    return Err(expr.parse_err(
                        "auth handler must have a handler function as its first argument",
                    ));
                };
                let (mut req, mut resp) = parse_auth_handler_signature(&handler.expr)?;

                if req.is_none() {
                    req = extract_type_param(expr.type_args.as_deref(), 0);
                }
                if resp.is_none() {
                    resp = extract_type_param(expr.type_args.as_deref(), 1);
                }

                let Some(req) = req else {
                    return Err(expr
                        .parse_err("auth handler must have an explicitly defined parameter type"));
                };
                let Some(resp) = resp else {
                    return Err(
                        expr.parse_err("auth handler must have an explicitly defined result type")
                    );
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
        }
        Ok(None)
    }
}

fn parse_auth_handler_signature(
    expr: &ast::Expr,
) -> ParseResult<(Option<&ast::TsType>, Option<&ast::TsType>)> {
    let (req_param, type_params, return_type) = match expr {
        ast::Expr::Fn(func) => (
            func.function.params.first().map(|p| &p.pat),
            func.function.type_params.as_deref(),
            func.function.return_type.as_deref(),
        ),
        ast::Expr::Arrow(arrow) => (
            arrow.params.first(),
            arrow.type_params.as_deref(),
            arrow.return_type.as_deref(),
        ),
        _ => return Ok((None, None)),
    };

    if let Some(type_params) = &type_params {
        return Err(type_params.parse_err("auth handler cannot have type parameters"));
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
) -> Option<&ast::TsType> {
    let params = params?;
    let param = params.params.get(idx)?;
    Some(param.as_ref())
}
