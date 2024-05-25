use std::collections::HashMap;

use anyhow::{Context, Result};

use crate::parser::resources::apis::api::{Method, Methods};
use crate::parser::respath::Path;
use crate::parser::types::custom::{resolve_custom_type_named, CustomType};
use crate::parser::types::{drop_empty_or_void, unwrap_promise, Basic, ResolveState, Interface, InterfaceField, Type, TypeChecker};

/// Describes how an API endpoint can be encoded on the wire.
#[derive(Debug, Clone)]
pub struct EndpointEncoding {
    /// The endpoint's API path.
    pub path: Path,

    /// The methods the endpoint can be called with.
    pub methods: Methods,

    /// The default method to use for calling this endpoint.
    pub default_method: Method,

    pub req: Vec<RequestEncoding>,
    pub resp: ResponseEncoding,

    /// The raw request and schemas, from the source code.
    pub raw_req_schema: Option<Type>,
    pub raw_resp_schema: Option<Type>,
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
    pub auth_param: Type,
    pub auth_data: Type,
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

pub fn describe_endpoint(
    tc: &TypeChecker,
    methods: Methods,
    path: Path,
    req: Option<Type>,
    resp: Option<Type>,
    raw: bool,
) -> Result<EndpointEncoding> {
    let resp = resp
        .map(|t| unwrap_promise(tc.state(), &t).clone())
        .and_then(|t| drop_empty_or_void(t));

    let default_method = default_method(&methods);
    let (req_enc, _req_schema) = describe_req(tc, &methods, &path, &req, raw)?;
    let (resp_enc, _resp_schema) = describe_resp(tc, &methods, &resp)?;

    let path = rewrite_path_types(&req_enc[0], path, raw).context("parse path param types")?;

    Ok(EndpointEncoding {
        path,
        methods,
        default_method,
        req: req_enc,
        resp: resp_enc,
        raw_req_schema: req,
        raw_resp_schema: resp,
    })
}

fn describe_req(
    tc: &TypeChecker,
    methods: &Methods,
    path: &Path,
    req_schema: &Option<Type>,
    raw: bool,
) -> Result<(Vec<RequestEncoding>, Option<FieldMap>)> {
    let Some(req_schema) = req_schema else {
        // We don't have any request schema. This is valid if and only if
        // we have no path parameters or it's a raw endpoint.
        if !path.has_dynamic_segments() || raw {
            return Ok((
                vec![RequestEncoding {
                    methods: methods.clone(),
                    params: vec![],
                }],
                None,
            ));
        } else {
            anyhow::bail!("missing request schema");
        }
    };

    let mut fields = iface_fields(tc, &req_schema)?;
    let path_params = extract_path_params(path, &mut fields)?;

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

    for (loc, methods) in split_by_loc(&methods) {
        let mut params = path_params.clone();
        params.extend(extract_loc_params(&fields, loc));
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
    resp_schema: &Option<Type>,
) -> Result<(ResponseEncoding, Option<FieldMap>)> {
    let Some(resp_schema) = resp_schema else {
        return Ok((ResponseEncoding { params: vec![] }, None));
    };

    let fields = iface_fields(tc, &resp_schema)?;
    let params = extract_loc_params(&fields, ParamLocation::Body);

    let fields = if fields.is_empty() {
        None
    } else {
        Some(fields)
    };

    Ok((ResponseEncoding { params }, fields))
}

pub fn describe_auth_handler(
    ctx: &ResolveState,
    params: Type,
    response: Type,
) -> Result<AuthHandlerEncoding> {
    let response = unwrap_promise(ctx, &response).clone();

    Ok(AuthHandlerEncoding {
        auth_param: params,
        auth_data: response,
    })
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
        Methods::Some(methods) => &methods,
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

    let mut items: Vec<_> = locs.into_iter().map(|(k, v)| (k, v)).collect();
    items.sort();
    items
}

pub type FieldMap = HashMap<String, Field>;

pub struct Field {
    name: String,
    typ: Type,
    optional: bool,
    custom: Option<CustomType>,
}

fn iface_fields<'a>(tc: &'a TypeChecker, typ: &'a Type) -> Result<FieldMap> {
    fn to_fields(state: &ResolveState, iface: &Interface) -> Result<FieldMap> {
        let mut map = HashMap::new();
        for f in &iface.fields {
            map.insert(f.name.clone(), rewrite_custom_type_field(state, f)?);
        }
        Ok(map)
    }

    let typ = unwrap_promise(tc.state(), typ);
    match typ {
        Type::Basic(Basic::Void) => Ok(HashMap::new()),
        Type::Interface(iface) => to_fields(tc.state(), iface),
        Type::Named(named) => {
            let underlying = tc.underlying(named.obj.module_id, typ);
            iface_fields(tc, &underlying)
        }
        _ => anyhow::bail!("expected named interface type, found {:?}", typ),
    }
}

fn extract_path_params(path: &Path, fields: &mut FieldMap) -> Result<Vec<Param>> {
    let mut params = Vec::new();
    let mut index = 0;
    for seg in path.dynamic_segments() {
        let name = seg.lit_or_name();
        let Some(f) = fields.remove(name) else {
            anyhow::bail!("path parameter {:?} not found in request schema", name);
        };
        params.push(Param {
            name: name.to_string(),
            loc: ParamData::Path { index },
            typ: f.typ.clone(),
            optional: f.optional,
        });
        index += 1;
    }

    Ok(params)
}

fn extract_loc_params(fields: &FieldMap, default_loc: ParamLocation) -> Vec<Param> {
    let mut params = Vec::new();
    for f in fields.values() {
        let name = f.name.clone();

        // Determine the location.
        let (loc, loc_name) = match &f.custom {
            Some(CustomType::Header { name, .. }) => (ParamLocation::Header, name.clone()),
            Some(CustomType::Query { name, .. }) => (ParamLocation::Query, name.clone()),
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

            ParamLocation::Path => panic!("path params are not supported as a default loc"),
        };

        params.push(Param {
            name,
            loc: param_data,
            typ: f.typ.clone(),
            optional: f.optional,
        });
    }
    params
}

fn rewrite_path_types(req: &RequestEncoding, path: Path, raw: bool) -> Result<Path> {
    use crate::parser::respath::{Segment, ValueType};
    // Get the path params into a map, keyed by name.
    let path_params = req
        .path()
        .map(|param| (&param.name, param))
        .collect::<HashMap<_, _>>();

    let typ_to_value_type = |typ: &Type| {
        Ok(match typ {
            Type::Basic(Basic::String) => ValueType::String,
            Type::Basic(Basic::Boolean) => ValueType::Bool,
            Type::Basic(Basic::Number | Basic::BigInt) => ValueType::Int,
            typ => anyhow::bail!("unsupported path param type: {:?}", typ),
        })
    };

    let mut segments = Vec::with_capacity(path.segments.len());
    for seg in path.segments.into_iter() {
        let seg = match seg {
            Segment::Param { name, .. } => {
                // Get the value type of the path parameter.
                let value_type = match path_params.get(&name) {
                    Some(param) => typ_to_value_type(&param.typ)?,
                    None => {
                        // Raw endpoints assume path params are strings.
                        if raw {
                            ValueType::String
                        } else {
                            anyhow::bail!("path param {:?} not found in request schema", name);
                        }
                    }
                };

                Segment::Param { name, value_type }
            }
            Segment::Literal(_) | Segment::Wildcard { .. } | Segment::Fallback { .. } => seg,
        };
        segments.push(seg);
    }

    Ok(Path { segments })
}

fn rewrite_custom_type_field(ctx: &ResolveState, field: &InterfaceField) -> Result<Field> {
    let standard_field = Field {
        name: field.name.clone(),
        typ: field.typ.clone(),
        optional: field.optional,
        custom: None,
    };
    let Type::Named(named) = &field.typ else {
        return Ok(standard_field);
    };

    Ok(match resolve_custom_type_named(ctx, named)? {
        None => standard_field,
        Some(CustomType::Header { typ, name }) => Field {
            custom: Some(CustomType::Query {
                typ: typ.clone(),
                name,
            }),
            typ,
            ..standard_field
        },
        Some(CustomType::Query { typ, name }) => Field {
            custom: Some(CustomType::Query {
                typ: typ.clone(),
                name,
            }),
            typ,
            ..standard_field
        },
    })
}
