use anyhow::Result;

use crate::parser::types::{typ, Basic, Literal, ResolveState, Type};

pub enum CustomType {
    Header { name: Option<String>, typ: Type },
    Query { name: Option<String>, typ: Type },
}

pub fn resolve_custom_type_named(
    ctx: &ResolveState,
    named: &typ::Named,
) -> Result<Option<CustomType>> {
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
    let (typ, name) = match (named.type_arguments.get(0), named.type_arguments.get(1)) {
        (None, None) => (Type::Basic(Basic::String), None),

        (Some(first), None) => {
            // If we only have a single argument, check its type.
            // If it's a string literal it's the name, otherwise it's the type.
            match first {
                Type::Literal(Literal::String(lit)) => {
                    (Type::Basic(Basic::String), Some(lit.to_string()))
                }
                _ => (first.clone(), None),
            }
        }

        (Some(typ), Some(name)) => (typ.clone(), Some(resolve_str_lit(name)?)),
        (None, Some(_)) => unreachable!(),
    };
    Ok(CustomType::Header { typ, name })
}

fn resolve_query_type(named: &typ::Named) -> Result<CustomType> {
    let (typ, name) = match (named.type_arguments.get(0), named.type_arguments.get(1)) {
        (None, None) => (Type::Basic(Basic::String), None),

        (Some(first), None) => {
            // If we only have a single argument, check its type.
            // If it's a string literal it's the name, otherwise it's the type.
            match first {
                Type::Literal(Literal::String(lit)) => {
                    (Type::Basic(Basic::String), Some(lit.to_string()))
                }
                _ => (first.clone(), None),
            }
        }

        (Some(typ), Some(name)) => (typ.clone(), Some(resolve_str_lit(name)?)),
        (None, Some(_)) => unreachable!(),
    };

    Ok(CustomType::Query { typ, name })
}

fn resolve_str_lit(typ: &Type) -> Result<String> {
    match typ {
        Type::Literal(Literal::String(s)) => Ok(s.clone()),
        _ => anyhow::bail!("expected string literal"),
    }
}
