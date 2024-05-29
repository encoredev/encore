mod ast_id;
mod binding;
pub mod custom;
mod object;
mod typ;
mod type_resolve;
mod utils;

#[cfg(test)]
mod tests;
mod resolved;

pub use object::{Object, ObjectId, ObjectKind, ResolveState};
pub use typ::{Basic, ClassType, Interface, InterfaceField, Literal, Named, Type, TypeArgId, Generic};
pub use type_resolve::TypeChecker;
pub use utils::*;
