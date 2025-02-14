use std::collections::HashMap;

use litparser::{ParseResult, Sp, ToParseErr};
use swc_common::Span;
use thiserror::Error;

use crate::parser::resources::apis::api::{Method, Methods};
use crate::parser::respath::Path;
use crate::parser::types::{
    drop_empty_or_void, unwrap_promise, unwrap_validated, validation, Basic, Custom, FieldName,
    Interface, InterfaceField, ResolveState, Type, TypeChecker, WireLocation, WireSpec,
};
use crate::parser::Range;
use crate::span_err::{ErrorWithSpanExt, SpErr};

/// Describes how an API endpoint can be encoded on the wire.
#[derive(Debug, Clone)]
pub struct EndpointEncoding {
    /// The endpoint's definition span
    pub span: Span,

    /// The endpoint's API path.
    pub path: Path,

    /// The methods the endpoint can be called with.
    pub methods: Methods,

    /// The default method to use for calling this endpoint.
    pub default_method: Method,

    pub req: Vec<RequestEncoding>,
    pub resp: ResponseEncoding,

    /// Schema for the websocket handshake, if stream.
    pub handshake: Option<RequestEncoding>,

    /// The raw request and schemas, from the source code.
    pub raw_handshake_schema: Option<Sp<Type>>,
    pub raw_req_schema: Option<Sp<Type>>,
    pub raw_resp_schema: Option<Sp<Type>>,
}

impl EndpointEncoding {
    pub fn default_request_encoding(&self) -> &RequestEncoding {
        &self.req[0]
    }
}

#[derive(Debug, Clone, Copy, PartialOrd, Ord, PartialEq, Eq, Hash)]
pub enum ParamLocation {
    Path,
    Header,
    Query,
    Body,
    Cookie,
}

#[derive(Debug, Clone)]
pub enum ParamData {
    Path { index: usize },
    Header { header: String },
    Query { query: String },
    Body,
    Cookie,
}

#[derive(Debug, Clone)]
pub struct Param {
    pub name: String,
    pub loc: ParamData,
    pub typ: Type,
    pub optional: bool,
    pub range: Range,
}

#[derive(Debug, Clone)]
pub struct RequestEncoding {
    /// The method(s) this encoding is for.
    pub methods: Methods,

    /// Parsed params.
    pub params: Vec<Param>,
}

#[derive(Debug, Clone)]
pub struct ResponseEncoding {
    /// Parsed params.
    pub params: Vec<Param>,
}

#[derive(Debug, Clone)]
pub struct AuthHandlerEncoding {
    pub auth_param: Sp<Type>,
    pub auth_data: Sp<Type>,
}

pub struct RequestParamsByLoc<'a> {
    pub path: Vec<&'a Param>,
    pub header: Vec<&'a Param>,
    pub query: Vec<&'a Param>,
    pub body: Vec<&'a Param>,
    pub cookie: Vec<&'a Param>,
}

impl RequestEncoding {
    pub fn by_loc(&self) -> RequestParamsByLoc {
        let mut by_loc = RequestParamsByLoc {
            path: vec![],
            header: vec![],
            query: vec![],
            body: vec![],
            cookie: vec![],
        };
        for p in &self.params {
            match p.loc {
                ParamData::Path { .. } => by_loc.path.push(p),
                ParamData::Header { .. } => by_loc.header.push(p),
                ParamData::Query { .. } => by_loc.query.push(p),
                ParamData::Body => by_loc.body.push(p),
                ParamData::Cookie => by_loc.cookie.push(p),
            }
        }
        by_loc
    }

    pub fn path(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Path { .. }))
    }

    pub fn header(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Header { .. }))
    }

    pub fn query(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Query { .. }))
    }

    pub fn body(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Body))
    }

    pub fn cookie(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Cookie))
    }
}

impl ResponseEncoding {
    pub fn cookie(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Cookie))
    }

    pub fn header(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Header { .. }))
    }

    pub fn body(&self) -> impl Iterator<Item = &Param> {
        self.params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::Body))
    }
}

pub fn describe_stream_endpoint(
    def_span: Span,
    tc: &TypeChecker,
    methods: Methods,
    path: Path,
    req: Option<Sp<Type>>,
    resp: Option<Sp<Type>>,
    handshake: Option<Sp<Type>>,
) -> ParseResult<EndpointEncoding> {
    let resp = if let Some(resp) = resp {
        let (span, resp) = resp.split();
        drop_empty_or_void(unwrap_promise(tc.state(), &resp).clone()).map(|t| Sp::new(span, t))
    } else {
        None
    };

    let default_method = default_method(&methods);

    let (handshake_enc, _req_schema) = describe_req(
        def_span,
        tc,
        &Methods::Some(vec![Method::Get]),
        Some(&path),
        &handshake,
        false,
    )?;

    let handshake_enc = match handshake_enc.as_slice() {
        [] => None,
        [ref enc] => Some(enc.clone()),
        _ => return Err(def_span.parse_err("unexpected handshake encoding")),
    };

    let (req_enc, _req_schema) = if handshake_enc.is_some() {
        describe_req(def_span, tc, &methods, None, &req, false)?
    } else {
        describe_req(def_span, tc, &methods, Some(&path), &req, false)?
    };

    let (resp_enc, _resp_schema) = describe_resp(tc, &methods, &resp)?;

    let path = if let Some(ref enc) = handshake_enc {
        rewrite_path_types(enc, path, false)?
    } else {
        path
    };

    Ok(EndpointEncoding {
        span: def_span,
        path,
        methods,
        default_method,
        req: req_enc,
        resp: resp_enc,
        handshake: handshake_enc,
        raw_handshake_schema: handshake,
        raw_req_schema: req,
        raw_resp_schema: resp,
    })
}

pub fn describe_endpoint(
    def_span: Span,
    tc: &TypeChecker,
    methods: Methods,
    path: Path,
    req: Option<Sp<Type>>,
    resp: Option<Sp<Type>>,
    raw: bool,
) -> ParseResult<EndpointEncoding> {
    let resp = if let Some(resp) = resp {
        let (span, resp) = resp.split();
        drop_empty_or_void(unwrap_promise(tc.state(), &resp).clone()).map(|t| Sp::new(span, t))
    } else {
        None
    };

    let default_method = default_method(&methods);

    let (req_enc, _req_schema) = describe_req(def_span, tc, &methods, Some(&path), &req, raw)?;
    let (resp_enc, _resp_schema) = describe_resp(tc, &methods, &resp)?;

    let path = rewrite_path_types(&req_enc[0], path, raw)?;

    Ok(EndpointEncoding {
        span: def_span,
        path,
        methods,
        default_method,
        req: req_enc,
        resp: resp_enc,
        handshake: None,
        raw_handshake_schema: None,
        raw_req_schema: req,
        raw_resp_schema: resp,
    })
}

pub fn describe_static_assets(def_span: Span, methods: Methods, path: Path) -> EndpointEncoding {
    EndpointEncoding {
        span: def_span,
        path,
        methods: methods.clone(),
        default_method: Method::Get,
        req: vec![RequestEncoding {
            methods,
            params: vec![],
        }],
        resp: ResponseEncoding { params: vec![] },
        handshake: None,
        raw_handshake_schema: None,
        raw_req_schema: None,
        raw_resp_schema: None,
    }
}

fn describe_req(
    def_span: Span,
    tc: &TypeChecker,
    methods: &Methods,
    path: Option<&Path>,
    req_schema: &Option<Sp<Type>>,
    raw: bool,
) -> ParseResult<(Vec<RequestEncoding>, Option<FieldMap>)> {
    let Some(req_schema) = req_schema else {
        // We don't have any request schema. This is valid if and only if
        // we have no path parameters or it's a raw endpoint.
        if path.is_none() || !path.unwrap().has_dynamic_segments() || raw {
            return Ok((
                vec![RequestEncoding {
                    methods: methods.clone(),
                    params: vec![],
                }],
                None,
            ));
        } else {
            return Err(
                def_span.parse_err("request schema must be defined when having path parameters")
            );
        }
    };

    let mut fields =
        iface_fields(tc, req_schema).map_err(|err| err.span.parse_err(err.error.to_string()))?;
    let path_params = if let Some(path) = path {
        extract_path_params(path, &mut fields)?
    } else {
        vec![]
    };

    // If there are no fields remaining, we can use this encoding for all methods.
    if fields.is_empty() {
        return Ok((
            vec![RequestEncoding {
                methods: methods.clone(),
                params: path_params,
            }],
            None,
        ));
    }

    // Otherwise, the fields should be grouped by location depending on the method.
    let mut encodings = Vec::new();

    for (loc, methods) in split_by_loc(methods) {
        let mut params = path_params.clone();
        params.extend(extract_loc_params(&fields, loc)?);
        encodings.push(RequestEncoding {
            methods: Methods::Some(methods),
            params,
        });
    }

    Ok((encodings, Some(fields)))
}

fn describe_resp(
    tc: &TypeChecker,
    _methods: &Methods,
    resp_schema: &Option<Sp<Type>>,
) -> ParseResult<(ResponseEncoding, Option<FieldMap>)> {
    let Some(resp_schema) = resp_schema else {
        return Ok((ResponseEncoding { params: vec![] }, None));
    };

    let fields =
        iface_fields(tc, resp_schema).map_err(|err| err.span.parse_err(err.error.to_string()))?;
    let params = extract_loc_params(&fields, ParamLocation::Body)?;

    let fields = if fields.is_empty() {
        None
    } else {
        Some(fields)
    };

    Ok((ResponseEncoding { params }, fields))
}

pub fn describe_auth_handler(
    ctx: &ResolveState,
    params: Sp<Type>,
    response: Sp<Type>,
) -> AuthHandlerEncoding {
    let (span, response) = response.split();
    let response = unwrap_promise(ctx, &response).clone();

    AuthHandlerEncoding {
        auth_param: params,
        auth_data: Sp::new(span, response),
    }
}

fn default_method(methods: &Methods) -> Method {
    match methods {
        Methods::All => Method::Post,
        Methods::Some(methods) => {
            if methods.contains(&Method::Post) {
                Method::Post
            } else {
                methods[0]
            }
        }
    }
}

fn split_by_loc(methods: &Methods) -> Vec<(ParamLocation, Vec<Method>)> {
    let methods = match methods {
        Methods::All => Method::all(),
        Methods::Some(methods) => methods,
    };

    let mut locs = HashMap::new();
    for m in methods {
        let loc = if m.supports_body() {
            ParamLocation::Body
        } else {
            ParamLocation::Query
        };
        locs.entry(loc).or_insert(Vec::new()).push(*m);
    }

    let mut items: Vec<_> = locs.into_iter().collect();
    items.sort();
    items
}

pub type FieldMap = HashMap<String, Field>;

pub struct Field {
    name: String,
    typ: Type,
    optional: bool,
    custom: Option<WireSpec>,
    range: Range,
}

impl Field {
    pub fn is_custom(&self) -> bool {
        self.custom.is_some()
    }

    pub fn range(&self) -> Range {
        self.range
    }
}

#[derive(Error, Debug)]
pub enum Error {
    #[error("expected named interface type, found {0}")]
    ExpectedNamedInterfaceType(String),
    #[error("invalid custom type field")]
    InvalidCustomType(#[source] anyhow::Error),
}

pub(crate) fn iface_fields<'a>(
    tc: &'a TypeChecker,
    typ: &'a Sp<Type>,
) -> Result<FieldMap, SpErr<Error>> {
    fn to_fields(iface: &Interface) -> Result<FieldMap, SpErr<Error>> {
        let mut map = HashMap::new();
        for f in &iface.fields {
            if let FieldName::String(name) = &f.name {
                map.insert(name.clone(), rewrite_custom_type_field(f, name));
            }
        }
        Ok(map)
    }

    let span = typ.span();
    let typ = unwrap_promise(tc.state(), typ);
    match typ {
        Type::Basic(Basic::Void) => Ok(HashMap::new()),
        Type::Interface(iface) => to_fields(iface),
        Type::Named(named) => {
            let underlying = Sp::new(span, tc.underlying(named.obj.module_id, typ));
            iface_fields(tc, &underlying)
        }
        _ => Err(Error::ExpectedNamedInterfaceType(format!("{typ:?}")).with_span(span)),
    }
}

fn extract_path_params(path: &Path, fields: &mut FieldMap) -> ParseResult<Vec<Param>> {
    let mut params = Vec::new();
    for (index, seg) in path.dynamic_segments().enumerate() {
        let name = seg.lit_or_name();
        let Some(f) = fields.remove(name) else {
            return Err(seg.parse_err("path parameter not found in request schema"));
        };
        params.push(Param {
            name: name.to_string(),
            loc: ParamData::Path { index },
            typ: f.typ.clone(),
            optional: f.optional,
            range: f.range,
        });
    }

    Ok(params)
}

fn extract_loc_params(fields: &FieldMap, default_loc: ParamLocation) -> ParseResult<Vec<Param>> {
    let mut params = Vec::new();
    for f in fields.values() {
        let name = f.name.clone();

        // Determine the location.
        let (loc, loc_name) = match &f.custom {
            Some(spec) => (
                match spec.location {
                    WireLocation::Header => ParamLocation::Header,
                    WireLocation::Query => ParamLocation::Query,
                    WireLocation::PubSubAttr => ParamLocation::Body,
                },
                spec.name_override.clone(),
            ),
            None => (default_loc, None),
        };

        let param_data: ParamData = match loc {
            ParamLocation::Query => ParamData::Query {
                query: loc_name.unwrap_or_else(|| f.name.clone()),
            },
            ParamLocation::Body => ParamData::Body,
            ParamLocation::Cookie => ParamData::Cookie,
            ParamLocation::Header => ParamData::Header {
                header: loc_name.unwrap_or_else(|| f.name.clone()),
            },

            ParamLocation::Path => {
                return Err(f
                    .range
                    .to_span()
                    .parse_err("path params are not supported as a default loc"))
            }
        };

        params.push(Param {
            name,
            loc: param_data,
            typ: f.typ.clone(),
            optional: f.optional,
            range: f.range,
        });
    }
    Ok(params)
}

fn rewrite_path_types(req: &RequestEncoding, path: Path, raw: bool) -> ParseResult<Path> {
    use crate::parser::respath::{Segment, ValueType};
    // Get the path params into a map, keyed by name.
    let path_params = req
        .path()
        .map(|param| (param.name.as_str(), param))
        .collect::<HashMap<_, _>>();

    fn typ_to_value_type(param: &Param) -> ParseResult<(ValueType, Option<validation::Expr>)> {
        // Unwrap any validation expression before we check the type.
        let (typ, expr) = unwrap_validated(&param.typ);
        if let Some(expr) = &expr {
            if let Err(err) = expr.supports_type(typ) {
                return Err(param.range.parse_err(err.to_string()));
            }
        }

        match typ {
            Type::Basic(Basic::String) => Ok((ValueType::String, expr.cloned())),
            Type::Basic(Basic::Boolean) => Ok((ValueType::Bool, expr.cloned())),
            Type::Basic(Basic::Number | Basic::BigInt) => Ok((ValueType::Int, expr.cloned())),
            typ => Err(param
                .range
                .to_span()
                .parse_err(format!("unsupported path parameter type: {:?}", typ))),
        }
    }

    let resolve_value_type = |span: Span, name: &str| {
        match path_params.get(name) {
            Some(param) => typ_to_value_type(param),
            None => {
                // Raw endpoints assume path params are strings.
                if raw {
                    Ok((ValueType::String, None))
                } else {
                    Err(span.parse_err("path parameter not found in request schema"))
                }
            }
        }
    };

    let mut segments = Vec::with_capacity(path.segments.len());
    for seg in path.segments.into_iter() {
        let (seg_span, seg) = seg.split();
        let seg = match seg {
            Segment::Literal(_) => seg,
            Segment::Param { name, .. } => {
                let (value_type, validation) = resolve_value_type(seg_span, &name)?;
                Segment::Param {
                    name,
                    value_type,
                    validation,
                }
            }
            Segment::Wildcard { name, .. } => {
                let (_, validation) = resolve_value_type(seg_span, &name)?;
                Segment::Wildcard { name, validation }
            }
            Segment::Fallback { name, .. } => {
                let (_, validation) = resolve_value_type(seg_span, &name)?;
                Segment::Fallback { name, validation }
            }
        };
        segments.push(Sp::new(seg_span, seg));
    }

    Ok(Path {
        span: path.span,
        segments,
    })
}

pub fn resolve_wire_spec(typ: &Type) -> Option<&WireSpec> {
    match typ {
        Type::Custom(Custom::WireSpec(spec)) => Some(spec),
        Type::Validated(v) => resolve_wire_spec(&v.typ),
        Type::Optional(opt) => resolve_wire_spec(&opt.0),
        _ => None,
    }
}

fn rewrite_custom_type_field(field: &InterfaceField, field_name: &str) -> Field {
    let mut standard_field = Field {
        name: field_name.to_string(),
        typ: field.typ.clone(),
        optional: field.optional,
        custom: None,
        range: field.range,
    };

    if let Some(spec) = resolve_wire_spec(&field.typ) {
        standard_field.custom = Some(spec.clone());
    };

    standard_field
}
