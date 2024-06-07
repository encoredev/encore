use crate::parser::respath::Path;
use crate::parser::types::{FieldName, Interface, ResolveState};

pub(super) fn strip_path_params(path: &Path, typ: &mut Interface) {
    if !path.has_dynamic_segments() {
        return;
    }

    let is_path_param = |name: &str| {
        path.dynamic_segments()
            .find(|seg| seg.lit_or_name() == name)
            .is_some()
    };

    // Drop any fields whose type is Path.
    typ.fields.retain(|f| {
        if let FieldName::String(name) = &f.name {
            if is_path_param(name) {
                return false;
            }
        }
        true
    });
}
