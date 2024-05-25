use crate::parser::types::{ResolveState, Interface, Type};

pub(super) fn strip_path_params(ctx: &ResolveState, typ: &mut Interface) {
    // Drop any fields whose type is Path.
    typ.fields.retain(|f| {
        if let Type::Named(named) = &f.typ {
            let obj = &named.obj;
            if obj.name.as_deref() == Some("Path")
                && ctx.is_module_path(obj.module_id, "encore.dev/api")
            {
                return false;
            }
        }
        true
    });
}
