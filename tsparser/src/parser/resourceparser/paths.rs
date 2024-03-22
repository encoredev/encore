/// A path to a package, e.g. `@foo/bar`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct PkgPath<'a>(pub &'a str);

/// A path to a specific object in a package, e.g. 'Moo' in '@foo/bar'.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct PkgObj<'a>(pub PkgPath<'a>, pub &'a str);
