use std::hash::{Hash, Hasher};
use std::rc::Rc;

use swc_ecma_ast as ast;

use crate::parser::module_loader::ModuleId;
use crate::parser::resources::Resource;
use crate::parser::types::Object;
use crate::parser::Range;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct Id(u32);

impl From<u32> for Id {
    fn from(id: u32) -> Self {
        Self(id)
    }
}

#[derive(Debug)]
pub struct BindData {
    pub resource: Resource,
    pub range: Range,

    /// The identifier it is bound to, if any.
    pub ident: Option<ast::Ident>,
    // The object it is bound to, if any.
    pub object: Option<Rc<Object>>,
    pub kind: BindKind,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BindKind {
    Create,
    Reference,
}

#[derive(Debug)]
pub struct Bind {
    pub id: Id,
    pub range: Option<Range>,
    pub resource: Resource,
    pub kind: BindKind,

    /// The module the bind is defined in.
    pub module_id: ModuleId,

    /// The identifier it is bound to, if any.
    /// None means it's an anonymous bind (e.g. `_`).
    pub name: Option<String>,

    /// The object it is bound to, if any.
    pub object: Option<Rc<Object>>,

    /// The identifier it's bound to in the source module.
    /// None means it's an anonymous bind (e.g. `_`).
    /// It's used for computing usage within the module itself,
    /// where we need to know its id.
    pub internal_bound_id: Option<ast::Id>,
}

impl PartialEq<Self> for Bind {
    fn eq(&self, other: &Self) -> bool {
        self.id == other.id
    }
}

impl Hash for Bind {
    fn hash<H: Hasher>(&self, state: &mut H) {
        self.id.hash(state);
    }
}
