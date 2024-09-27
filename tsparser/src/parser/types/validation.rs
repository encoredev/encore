use serde::Serialize;

#[derive(Debug, Clone, Hash, Serialize)]
pub enum Expr {
    Rule(Rule),
    And(Vec<Expr>),
    Or(Vec<Expr>),
    Not(Box<Expr>),
}

#[derive(Debug, Clone, Hash, Serialize)]
pub enum Rule {
    MinLen(u64),
    MaxLen(u64),
    MinVal(i64),
    MaxVal(i64),
}
