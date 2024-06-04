mod ast_id;
mod binding;
pub mod custom;
mod object;
mod typ;
mod type_resolve;
mod utils;

mod resolved;
#[cfg(test)]
mod tests;

pub use object::{Object, ObjectId, ObjectKind, ResolveState};
pub use typ::{
    Basic, ClassType, FieldName, Generic, Interface, InterfaceField, Literal, Named, Type,
    TypeArgId,
};
pub use type_resolve::TypeChecker;
pub use utils::*;
