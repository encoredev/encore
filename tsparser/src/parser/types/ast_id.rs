use std::hash::Hash;
use swc_ecma_ast as ast;

/// AstId is a convenience wrapper around ast::Id that also tracks the name of the identifier,
/// for debugging purposes. It can be swapped out for ast::Id later.
#[derive(Debug)]
pub struct AstId(ast::Id, String);

impl AstId {
    pub fn new(id: ast::Id, name: String) -> Self {
        Self(id, name)
    }
}

impl std::fmt::Display for AstId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.1)
    }
}

impl From<ast::Id> for AstId {
    fn from(id: ast::Id) -> Self {
        Self(id, "unknown".into())
    }
}

impl From<&ast::Ident> for AstId {
    fn from(ident: &ast::Ident) -> Self {
        Self(ident.to_id(), ident.sym.as_ref().to_string())
    }
}

impl Hash for AstId {
    fn hash<H: std::hash::Hasher>(&self, state: &mut H) {
        self.0.hash(state);
    }
}

impl PartialEq for AstId {
    fn eq(&self, other: &Self) -> bool {
        self.0.eq(&other.0)
    }
}
impl Eq for AstId {}

impl PartialOrd for AstId {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        self.0.partial_cmp(&other.0)
    }
}
impl Ord for AstId {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        self.0.cmp(&other.0)
    }
}
