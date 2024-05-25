use anyhow::Result;

use crate::parser::types::{typ, ResolveState};

pub enum CustomType {
    Header {
        name: Option<String>,
        typ: typ::Type,
    },
    Query {
        name: Option<String>,
        typ: typ::Type,
    },
}

pub fn resolve_custom_type_named(ctx: &ResolveState, named: &typ::Named) -> Result<Option<CustomType>> {
    if !ctx.is_module_path(named.obj.module_id, "encore.dev/api") {
        return Ok(None);
    }

    match &named.obj.name.as_deref() {
        Some("Header") => resolve_header_type(named).map(Some),
        Some("Query") => resolve_query_type(named).map(Some),
        _ => Ok(None),
    }
}

fn resolve_header_type(named: &typ::Named) -> Result<CustomType> {
    let (name, typ) = match (named.type_arguments.get(0), named.type_arguments.get(1)) {
        (None, None) => (None, typ::Type::Basic(typ::Basic::String)),
        (Some(name), None) => (
            Some(resolve_str_lit(name)?),
            typ::Type::Basic(typ::Basic::String),
        ),
        (Some(typ), Some(name)) => (Some(resolve_str_lit(name)?), typ.clone()),
        (None, Some(_)) => unreachable!(),
    };
    Ok(CustomType::Header { name, typ })
}

fn resolve_query_type(named: &typ::Named) -> Result<CustomType> {
    let (name, typ) = match (named.type_arguments.get(0), named.type_arguments.get(1)) {
        (None, None) => (None, typ::Type::Basic(typ::Basic::String)),
        (Some(name), None) => (
            Some(resolve_str_lit(name)?),
            typ::Type::Basic(typ::Basic::String),
        ),
        (Some(typ), Some(name)) => (Some(resolve_str_lit(name)?), typ.clone()),
        (None, Some(_)) => unreachable!(),
    };
    Ok(CustomType::Query { name, typ })
}

fn resolve_str_lit(typ: &typ::Type) -> Result<String> {
    match typ {
        typ::Type::Literal(typ::Literal::String(s)) => Ok(s.clone()),
        _ => anyhow::bail!("expected string literal"),
    }
}
