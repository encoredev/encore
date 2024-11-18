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

impl Rule {
    pub fn merge_and(&self, other: &Self) -> Option<Self> {
        use Rule::*;
        Some(match (self, other) {
            (MinLen(a), MinLen(b)) => MinLen((*a).min(*b)),
            (MaxLen(a), MaxLen(b)) => MaxLen((*a).max(*b)),
            (MinVal(a), MinVal(b)) => MinVal((*a).min(*b)),
            (MaxVal(a), MaxVal(b)) => MaxVal((*a).max(*b)),
            _ => return None,
        })
    }

    pub fn merge_or(&self, other: &Self) -> Option<Self> {
        use Rule::*;
        Some(match (self, other) {
            (MinLen(a), MinLen(b)) => MinLen((*a).max(*b)),
            (MaxLen(a), MaxLen(b)) => MaxLen((*a).min(*b)),
            (MinVal(a), MinVal(b)) => MinVal((*a).max(*b)),
            (MaxVal(a), MaxVal(b)) => MaxVal((*a).min(*b)),
            _ => return None,
        })
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

    #[allow(clippy::should_implement_trait)]
    pub fn not(self) -> Self {
        if let Expr::Not(inner) = self {
            *inner
        } else {
            Expr::Not(Box::new(self))
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
                while i < exprs.len() {
                    if !matches!(&exprs[i], Expr::Rule(_)) {
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
                while i < exprs.len() {
                    if !matches!(&exprs[i], Expr::Rule(_)) {
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
}
