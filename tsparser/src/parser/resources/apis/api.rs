use std::str::FromStr;

use anyhow::{anyhow, bail, Context, Ok, Result};
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;
use swc_ecma_ast::TsTypeParamInstantiation;

use litparser::{LitParser, Nullable};
use litparser_derive::LitParser;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::apis::encoding::{
    describe_endpoint, describe_stream_endpoint, EndpointEncoding,
};
use crate::parser::resources::parseutil::{
    extract_bind_name, iter_references, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::respath::Path;
use crate::parser::usageparser::{ResolveUsageData, Usage};
use crate::parser::{FilePath, Range};

#[derive(Debug, Clone)]
pub struct Endpoint {
    pub range: Range,
    pub name: String,
    pub service_name: String,
    pub doc: Option<String>,
    pub expose: bool,
    pub raw: bool,
    pub require_auth: bool,

    /// Body limit in bytes.
    /// None means no limit.
    pub body_limit: Option<u64>,

    pub streaming_request: bool,
    pub streaming_response: bool,

    pub encoding: EndpointEncoding,
}

#[derive(Debug, Clone)]
pub enum Methods {
    All,
    Some(Vec<Method>),
}

impl Methods {
    pub fn to_vec(&self) -> Vec<String> {
        let methods = match self {
            Methods::All => Method::all(),
            Methods::Some(vec) => vec,
        };
        methods.iter().map(|s| s.as_str().to_string()).collect()
    }

    pub fn contains(&self, m: Method) -> bool {
        match self {
            Methods::All => true,
            Methods::Some(vec) => vec.contains(&m),
        }
    }

    pub fn first(&self) -> Option<Method> {
        match self {
            Methods::All => Some(Method::Post),
            Methods::Some(vec) => vec.first().cloned(),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialOrd, Ord, PartialEq, Eq)]
pub enum Method {
    Get,
    Post,
    Patch,
    Put,
    Delete,
    Head,
    Options,
    Trace,
    Connect,
}

impl Method {
    /// Whether the method supports a request body.
    pub fn supports_body(&self) -> bool {
        match self {
            Self::Post | Self::Put | Self::Patch | Self::Connect => true,
            Self::Get | Self::Delete | Self::Head | Self::Options | Self::Trace => false,
        }
    }

    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Connect => "CONNECT",
            Self::Delete => "DELETE",
            Self::Get => "GET",
            Self::Head => "HEAD",
            Self::Options => "OPTIONS",
            Self::Patch => "PATCH",
            Self::Post => "POST",
            Self::Put => "PUT",
            Self::Trace => "TRACE",
        }
    }

    /// List all methods.
    pub fn all() -> &'static [Self] {
        &[
            Self::Get,
            Self::Post,
            Self::Patch,
            Self::Put,
            Self::Delete,
            Self::Head,
            Self::Options,
            Self::Trace,
            // Skip connect for now, since axum doesn't support it.
            // Self::Connect,
        ]
    }
}

impl FromStr for Method {
    type Err = anyhow::Error;
    fn from_str(s: &str) -> Result<Self> {
        Ok(match s {
            "CONNECT" => Self::Connect,
            "DELETE" => Self::Delete,
            "GET" => Self::Get,
            "HEAD" => Self::Head,
            "OPTIONS" => Self::Options,
            "PATCH" => Self::Patch,
            "POST" => Self::Post,
            "PUT" => Self::Put,
            "TRACE" => Self::Trace,
            _ => anyhow::bail!("invalid method: {}", s),
        })
    }
}

pub const ENDPOINT_PARSER: ResourceParser = ResourceParser {
    name: "endpoint",
    interesting_pkgs: &[PkgPath("encore.dev/api")],

    run: |pass| {
        let module = pass.module.clone();

        let service_name = match &pass.service_name {
            Some(name) => name.to_string(),
            None => {
                // TODO handle this in a better way.
                match &module.file_path {
                    FilePath::Real(ref buf) => buf
                        .parent()
                        .and_then(|p| p.file_name())
                        .and_then(|s| s.to_str())
                        .map(|s| s.to_string())
                        .ok_or(anyhow::anyhow!(
                            "unable to determine service name for endpoint"
                        ))?,
                    FilePath::Custom(ref str) => {
                        anyhow::bail!("unsupported file path for service: {}", str)
                    }
                }
            }
        };

        let names = TrackedNames::new(&[("encore.dev/api", "api")]);

        for r in iter_references::<APIEndpointLiteral>(&module, &names) {
            let r = r?;
            let path_str = r
                .config
                .path
                .unwrap_or_else(|| format!("/{}.{}", &service_name, r.endpoint_name));

            let path = Path::parse(&path_str, Default::default())?;

            let object = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &ast::Expr::Ident(r.bind_name.clone()));

            let methods = r.config.method.unwrap_or(Methods::Some(vec![Method::Post]));

            let raw = matches!(r.kind, EndpointKind::Raw);

            let mut streaming_request = false;
            let mut streaming_response = false;

            let encoding = match r.kind {
                EndpointKind::Typed { request, response } => {
                    let request = match request {
                        None => None,
                        Some(t) => Some(pass.type_checker.resolve_type(module.clone(), &t)),
                    };
                    let response = match response {
                        None => None,
                        Some(t) => Some(pass.type_checker.resolve_type(module.clone(), &t)),
                    };

                    describe_endpoint(pass.type_checker, methods, path, request, response, raw)?
                }
                EndpointKind::TypedStream {
                    handshake,
                    request,
                    response,
                } => {
                    streaming_request = request.is_stream();
                    streaming_response = response.is_stream();

                    // always register as a get endpoint
                    let methods = Methods::Some(vec![Method::Get]);

                    let request = request
                        .ts_type()
                        .map(|t| pass.type_checker.resolve_type(module.clone(), t));
                    let response = response
                        .ts_type()
                        .map(|t| pass.type_checker.resolve_type(module.clone(), t));
                    let handshake =
                        handshake.map(|t| pass.type_checker.resolve_type(module.clone(), &t));

                    describe_stream_endpoint(
                        pass.type_checker,
                        methods,
                        path,
                        request,
                        response,
                        handshake,
                        raw,
                    )?
                }
                EndpointKind::Raw => {
                    describe_endpoint(pass.type_checker, methods, path, None, None, raw)?
                }
            };

            // Compute the body limit. Null means no limit. No value means 2MiB.
            let body_limit: Option<u64> = match r.config.bodyLimit {
                Some(Nullable::Present(val)) => Some(val),
                Some(Nullable::Null) => None,
                None => Some(2 * 1024 * 1024),
            };

            let resource = Resource::APIEndpoint(Lrc::new(Endpoint {
                range: r.range,
                name: r.endpoint_name,
                service_name: service_name.clone(),
                doc: r.doc_comment,
                expose: r.config.expose.unwrap_or(false),
                require_auth: r.config.auth.unwrap_or(false),
                raw,
                streaming_request,
                streaming_response,
                body_limit,
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
        Ok(())
    },
};

#[derive(Debug)]
pub struct CallEndpointUsage {
    pub range: Range,
    pub endpoint: (String, String),
}

#[derive(Debug)]
pub struct ReferenceEndpointUsage {
    pub range: Range,
    pub endpoint: Lrc<Endpoint>,
}

pub fn resolve_endpoint_usage(_data: &ResolveUsageData, _endpoint: Lrc<Endpoint>) -> Option<Usage> {
    // Endpoints are just normal functions in TS, so no usage to resolve.
    None
    // Ok(match &data.expr.kind {
    //     UsageExprKind::Callee(_) => {
    //         // Considered just a normal function call.
    //     },
    //     UsageExprKind::Other(_other) => Usage::ReferenceEndpoint(ReferenceEndpointUsage {
    //         range: data.expr.range,
    //         endpoint,
    //     }),
    //     _ => anyhow::bail!("invalid endpoint usage"),
    // })
}

#[derive(Debug)]
struct APIEndpointLiteral {
    pub range: Range,
    pub doc_comment: Option<String>,
    pub endpoint_name: String,
    pub bind_name: ast::Ident,
    pub config: EndpointConfig,
    pub kind: EndpointKind,
}

#[derive(Debug)]
enum ParameterType {
    Stream(ast::TsType),
    Single(ast::TsType),
    None,
}

impl ParameterType {
    fn is_stream(&self) -> bool {
        matches!(self, ParameterType::Stream(..))
    }

    fn ts_type(&self) -> Option<&ast::TsType> {
        match self {
            ParameterType::Stream(t) => Some(t),
            ParameterType::Single(t) => Some(t),
            ParameterType::None => None,
        }
    }
}

#[derive(Debug)]
enum EndpointKind {
    Typed {
        request: Option<ast::TsType>,
        response: Option<ast::TsType>,
    },
    TypedStream {
        handshake: Option<ast::TsType>,
        request: ParameterType,
        response: ParameterType,
    },
    Raw,
}

#[derive(LitParser, Debug)]
#[allow(non_snake_case)]
struct EndpointConfig {
    method: Option<Methods>,
    path: Option<String>,
    expose: Option<bool>,
    auth: Option<bool>,
    bodyLimit: Option<Nullable<u64>>,
}

impl ReferenceParser for APIEndpointLiteral {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        for node in path.iter().rev() {
            if let swc_ecma_visit::AstParentNodeRef::CallExpr(
                expr,
                swc_ecma_visit::fields::CallExprField::Callee,
            ) = node
            {
                let doc_comment = module.preceding_comments(expr.span.lo.into());
                let Some(bind_name) = extract_bind_name(path)? else {
                    anyhow::bail!("API Endpoints must be bound to a variable")
                };

                let Some(config) = expr.args.first() else {
                    anyhow::bail!("API Endpoint must have a config object")
                };
                let config = EndpointConfig::parse_lit(config.expr.as_ref())?;

                let Some(handler) = &expr.args.get(1) else {
                    anyhow::bail!("API Endpoint must have a handler function")
                };

                let ast::Callee::Expr(callee) = &expr.callee else {
                    anyhow::bail!("invalid api definition expression")
                };

                // Determine what kind of endpoint it is.
                return Ok(Some(match callee.as_ref() {
                    ast::Expr::Member(member) if member.prop.is_ident_with("raw") => {
                        // Raw endpoint
                        Self {
                            range: expr.span.into(),
                            doc_comment,
                            endpoint_name: bind_name.sym.to_string(),
                            bind_name,
                            config,
                            kind: EndpointKind::Raw,
                        }
                    }

                    ast::Expr::Member(member) if member.prop.is_ident_with("streamBidi") => {
                        let type_params = expr
                            .type_args
                            .as_deref()
                            .context("no type parameters found")?;

                        let (handshake, request, response) = match type_params.params.len() {
                            2 => (
                                None,
                                extract_type_param(Some(type_params), 0)?
                                    .ok_or_else(|| anyhow!("missing type for request"))?,
                                extract_type_param(Some(type_params), 1)?
                                    .ok_or_else(|| anyhow!("missing type for response"))?,
                            ),
                            3 => (
                                extract_type_param(Some(type_params), 0)?,
                                extract_type_param(Some(type_params), 1)?
                                    .ok_or_else(|| anyhow!("missing type for request"))?,
                                extract_type_param(Some(type_params), 2)?
                                    .ok_or_else(|| anyhow!("missing type for response"))?,
                            ),
                            n => bail!("wrong number of type parameters, expected 2 or 3, got {n}"),
                        };

                        // Bidirectional stream
                        Self {
                            range: expr.span.into(),
                            doc_comment,
                            endpoint_name: bind_name.sym.to_string(),
                            bind_name,
                            config,
                            kind: EndpointKind::TypedStream {
                                handshake: handshake.cloned(),
                                request: ParameterType::Stream(request.clone()),
                                response: ParameterType::Stream(response.clone()),
                            },
                        }
                    }
                    ast::Expr::Member(member) if member.prop.is_ident_with("streamIn") => {
                        let type_params = expr
                            .type_args
                            .as_deref()
                            .context("no type parameters found")?;

                        let (handshake, request, response) = match type_params.params.len() {
                            1 => (
                                None,
                                extract_type_param(Some(type_params), 0)?
                                    .ok_or_else(|| anyhow!("missing type for request"))?,
                                None,
                            ),
                            2 => (
                                None,
                                extract_type_param(Some(type_params), 0)?
                                    .ok_or_else(|| anyhow!("missing type for request"))?,
                                Some(
                                    extract_type_param(Some(type_params), 1)?
                                        .ok_or_else(|| anyhow!("missing type for response"))?,
                                ),
                            ),
                            3 => (
                                extract_type_param(Some(type_params), 0)?,
                                extract_type_param(Some(type_params), 1)?
                                    .ok_or_else(|| anyhow!("missing type for request"))?,
                                Some(
                                    extract_type_param(Some(type_params), 2)?
                                        .ok_or_else(|| anyhow!("missing type for response"))?,
                                ),
                            ),
                            n => bail!(
                                "wrong number of type parameters, expected 1, 2 or 3, got {n}"
                            ),
                        };

                        let response = match response {
                            None => ParameterType::None,
                            Some(t) => ParameterType::Single(t.clone()),
                        };

                        // Incoming stream
                        Self {
                            range: expr.span.into(),
                            doc_comment,
                            endpoint_name: bind_name.sym.to_string(),
                            bind_name,
                            config,
                            kind: EndpointKind::TypedStream {
                                handshake: handshake.cloned(),
                                request: ParameterType::Stream(request.clone()),
                                response,
                            },
                        }
                    }
                    ast::Expr::Member(member) if member.prop.is_ident_with("streamOut") => {
                        let type_params = expr
                            .type_args
                            .as_deref()
                            .context("no type parameters found")?;

                        let (handshake, response) = match type_params.params.len() {
                            1 => (
                                None,
                                extract_type_param(Some(type_params), 0)?
                                    .ok_or_else(|| anyhow!("missing type for response"))?,
                            ),
                            2 => (
                                extract_type_param(Some(type_params), 0)?,
                                extract_type_param(Some(type_params), 1)?
                                    .ok_or_else(|| anyhow!("missing type for response"))?,
                            ),
                            n => bail!("wrong number of type parameters, expected 1 or 2, got {n}"),
                        };

                        // Outgoing stream
                        Self {
                            range: expr.span.into(),
                            doc_comment,
                            endpoint_name: bind_name.sym.to_string(),
                            bind_name,
                            config,
                            kind: EndpointKind::TypedStream {
                                handshake: handshake.cloned(),
                                request: ParameterType::None,
                                response: ParameterType::Stream(response.clone()),
                            },
                        }
                    }
                    _ => {
                        // Regular endpoint
                        let (mut req, mut resp) = parse_endpoint_signature(&handler.expr)?;

                        if req.is_none() {
                            req = extract_type_param(expr.type_args.as_deref(), 0)?;
                        }
                        if resp.is_none() {
                            resp = extract_type_param(expr.type_args.as_deref(), 1)?;
                        }

                        Self {
                            range: expr.span.into(),
                            doc_comment,
                            endpoint_name: bind_name.sym.to_string(),
                            bind_name,
                            config,
                            kind: EndpointKind::Typed {
                                request: req.cloned(),
                                response: resp.cloned(),
                            },
                        }
                    }
                }));
            }
        }

        Ok(None)
    }
}

fn parse_endpoint_signature(
    expr: &ast::Expr,
) -> Result<(Option<&ast::TsType>, Option<&ast::TsType>)> {
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

    if type_params.is_some() {
        anyhow::bail!("endpoint handler cannot have type parameters");
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

impl LitParser for Methods {
    fn parse_lit(expr: &ast::Expr) -> Result<Self> {
        Ok(match expr {
            ast::Expr::Lit(ast::Lit::Str(s)) => {
                if s.value.as_ref() == "*" {
                    Self::All
                } else {
                    Self::Some(vec![Method::from_str(s.value.as_ref())?])
                }
            }
            ast::Expr::Array(arr) => {
                let mut methods = Vec::with_capacity(arr.elems.len());
                for ast::ExprOrSpread { expr, .. } in arr.elems.iter().flatten() {
                    if let ast::Expr::Lit(ast::Lit::Str(s)) = expr.as_ref() {
                        if s.value.as_ref() == "*" {
                            if arr.elems.len() > 1 {
                                anyhow::bail!("invalid methods: cannot mix * and other methods");
                            }
                            return Ok(Self::All);
                        }
                        methods.push(Method::from_str(s.value.as_ref())?);
                    }
                }
                methods.sort();
                methods.dedup();
                Self::Some(methods)
            }
            _ => anyhow::bail!("invalid methods: must be string or array of strings"),
        })
    }
}
