use crate::parser::types::{Basic, ResolveState, Type};

pub fn unwrap_promise<'a>(state: &ResolveState, typ: &'a Type) -> &'a Type {
    if let Type::Named(named) = &typ {
        if named.obj.name.as_deref() == Some("Promise") && state.is_universe(named.obj.module_id) {
            if let Some(t) = named.type_arguments.get(0) {
                return t;
            }
        }
    }
    typ
}

pub fn drop_empty_or_void(typ: Type) -> Option<Type> {
    match typ {
        Type::Interface(iface) => {
            if iface.fields.is_empty() && iface.index.is_none() {
                None
            } else {
                Some(Type::Interface(iface))
            }
        }
        Type::Basic(Basic::Void) => None,
        _ => Some(typ),
    }
}
