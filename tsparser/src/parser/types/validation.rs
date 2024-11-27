use crate::encore::parser::schema::v1 as schema;
use core::hash::{Hash, Hasher};
use serde::Serialize;
use std::ops::Deref;

#[derive(Debug, Clone, Hash, Serialize, PartialEq, Eq)]
pub enum Expr {
    Rule(Rule),
    And(Vec<Expr>),
    Or(Vec<Expr>),
}

#[derive(Debug, Clone, Hash, Serialize, PartialEq, Eq)]
pub enum Rule {
    MinLen(u64),
    MaxLen(u64),
    MinVal(N),
    MaxVal(N),
    StartsWith(String),
    EndsWith(String),
    MatchesRegexp(String),
    Is(Is),
}

#[derive(Debug, Clone, Hash, Serialize, PartialEq, Eq)]
pub enum Is {
    Email,
    Url,
}

impl Rule {
    pub fn merge_and(&self, other: &Self) -> Option<Self> {
        use Rule::*;
        Some(match (self, other) {
            (MinLen(a), MinLen(b)) => MinLen((*a).min(*b)),
            (MaxLen(a), MaxLen(b)) => MaxLen((*a).max(*b)),
            (MinVal(a), MinVal(b)) => MinVal(N((*a).min(**b))),
            (MaxVal(a), MaxVal(b)) => MaxVal(N((*a).max(**b))),
            _ => return None,
        })
    }

    pub fn merge_or(&self, other: &Self) -> Option<Self> {
        use Rule::*;
        Some(match (self, other) {
            (MinLen(a), MinLen(b)) => MinLen((*a).max(*b)),
            (MaxLen(a), MaxLen(b)) => MaxLen((*a).min(*b)),
            (MinVal(a), MinVal(b)) => MinVal(N((*a).max(**b))),
            (MaxVal(a), MaxVal(b)) => MaxVal(N((*a).min(**b))),
            _ => return None,
        })
    }

    pub fn to_pb(&self) -> schema::validation_rule::Rule {
        use schema::validation_rule::Rule as VR;
        match self {
            Rule::MinLen(n) => VR::MinLen(*n),
            Rule::MaxLen(n) => VR::MaxLen(*n),
            Rule::MinVal(n) => VR::MinVal(**n),
            Rule::MaxVal(n) => VR::MaxVal(**n),
            Rule::StartsWith(str) => VR::StartsWith(str.clone()),
            Rule::EndsWith(str) => VR::EndsWith(str.clone()),
            Rule::MatchesRegexp(str) => VR::MatchesRegexp(str.clone()),
            Rule::Is(is) => VR::Is(match is {
                Is::Email => schema::validation_rule::Is::Email,
                Is::Url => schema::validation_rule::Is::Url,
            } as i32),
        }
    }
}

impl Expr {
    pub fn and(self, other: Self) -> Self {
        match (self, other) {
            (Expr::And(mut a), Expr::And(mut b)) => {
                // Can we merge any of the rules into a?
                a.append(&mut b);
                Expr::And(a)
            }
            (Expr::And(mut a), b) => {
                a.push(b);
                Expr::And(a)
            }
            (a, Expr::And(mut b)) => {
                b.insert(0, a);
                Expr::And(b)
            }
            (a, b) => Expr::And(vec![a, b]),
        }
    }

    pub fn or(self, other: Self) -> Self {
        match (self, other) {
            (Expr::Or(mut a), Expr::Or(mut b)) => {
                a.append(&mut b);
                Expr::Or(a)
            }
            (Expr::Or(mut a), b) => {
                a.push(b);
                Expr::Or(a)
            }
            (a, Expr::Or(mut b)) => {
                b.insert(0, a);
                Expr::Or(b)
            }
            (a, b) => Expr::Or(vec![a, b]),
        }
    }

    pub fn rule(rule: Rule) -> Self {
        Expr::Rule(rule)
    }

    pub fn simplify(self) -> Self {
        match self {
            Self::And(mut exprs) => {
                let mut i = 0;
                let mut size = exprs.len();
                while i < size {
                    if !matches!(&exprs[i], Expr::Rule(_)) {
                        i += 1;
                        continue;
                    };

                    let j = i + 1;
                    let (a, b) = exprs.split_at_mut(j);
                    let Expr::Rule(i_rule) = &mut a[i] else {
                        panic!("logic error");
                    };

                    let mut b_size = b.len();
                    let mut b_idx = 0;
                    'outer: while b_idx < b_size {
                        if let Expr::Rule(other) = &b[b_idx] {
                            if let Some(merged) = i_rule.merge_and(other) {
                                *i_rule = merged;

                                // Swap this element to the end of b
                                // and update the sizes.
                                b.swap(b_idx, b_size - 1);
                                size -= 1;
                                b_size -= 1;

                                // Don't increment the index since we now have
                                // a new element at the current index.
                                continue 'outer;
                            }
                        }
                        b_idx += 1;
                    }

                    i += 1;
                }

                exprs.truncate(size);
                Self::And(exprs)
            }

            Self::Or(mut exprs) => {
                let mut i = 0;
                let mut size = exprs.len();
                while i < size {
                    if !matches!(&exprs[i], Expr::Rule(_)) {
                        i += 1;
                        continue;
                    };

                    let j = i + 1;
                    let (a, b) = exprs.split_at_mut(j);
                    let Expr::Rule(i_rule) = &mut a[i] else {
                        panic!("logic error");
                    };

                    let mut b_size = b.len();
                    let mut b_idx = 0;
                    'outer: while b_idx < b_size {
                        if let Expr::Rule(other) = &b[b_idx] {
                            if let Some(merged) = i_rule.merge_or(other) {
                                *i_rule = merged;

                                // Swap this element to the end of b
                                // and update the sizes.
                                b.swap(b_idx, b_size - 1);
                                size -= 1;
                                b_size -= 1;

                                // Don't increment the index since we now have
                                // a new element at the current index.
                                continue 'outer;
                            }
                        }
                        b_idx += 1;
                    }

                    i += 1;
                }

                exprs.truncate(size);
                Self::Or(exprs)
            }

            _ => self,
        }
    }

    pub fn to_pb(&self) -> schema::ValidationExpr {
        use schema::validation_expr::Expr as VE;

        schema::ValidationExpr {
            expr: Some(match self {
                Expr::Rule(r) => VE::Rule(schema::ValidationRule {
                    rule: Some(r.to_pb()),
                }),
                Expr::And(exprs) => VE::And(schema::validation_expr::And {
                    exprs: exprs.iter().map(Self::to_pb).collect(),
                }),
                Expr::Or(exprs) => VE::Or(schema::validation_expr::Or {
                    exprs: exprs.iter().map(Self::to_pb).collect(),
                }),
            }),
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize)]
pub struct N(pub f64);

impl Deref for N {
    type Target = f64;

    fn deref(&self) -> &f64 {
        &self.0
    }
}

impl PartialEq for N {
    fn eq(&self, other: &Self) -> bool {
        self.0 == other.0
    }
}

impl Eq for N {}

impl Hash for N {
    fn hash<H: Hasher>(&self, h: &mut H) {
        if self.0 == 0.0f64 {
            // There are 2 zero representations, +0 and -0, which
            // compare equal but have different bits. We use the +0 hash
            // for both so that hash(+0) == hash(-0).
            0.0f64.to_bits().hash(h);
        } else {
            self.0.to_bits().hash(h);
        }
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn test_simplify() {
        use super::*;

        let expr = Expr::Or(vec![
            Expr::Rule(Rule::MinLen(10)),
            Expr::Rule(Rule::MinLen(20)),
            Expr::Rule(Rule::MinLen(30)),
        ]);

        let simplified = expr.simplify();
        assert_eq!(simplified, Expr::Rule(Rule::MinLen(10)));
    }
}
