use crate::encore::parser::schema::v1 as schema;
use core::hash::{Hash, Hasher};
use serde::Serialize;
use std::{
    error::Error,
    fmt::{Display, Write},
    ops::Deref,
};

use super::{Basic, Custom, Type};

#[derive(Debug, Clone, Hash, Serialize, PartialEq, Eq)]
pub enum Expr {
    Rule(Rule),
    And(Vec<Expr>),
    Or(Vec<Expr>),
}

impl Display for Expr {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Rule(r) => f.write_fmt(format_args!("{}", r)),
            Self::And(a) => {
                f.write_char('(')?;
                for (i, e) in a.iter().enumerate() {
                    if i > 0 {
                        f.write_str(" & ")?;
                    }
                    f.write_fmt(format_args!("{}", e))?;
                }
                f.write_char(')')
            }
            Self::Or(a) => {
                f.write_char('(')?;
                for (i, e) in a.iter().enumerate() {
                    if i > 0 {
                        f.write_str(" | ")?;
                    }
                    f.write_fmt(format_args!("{}", e))?;
                }
                f.write_char(')')
            }
        }
    }
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

impl Display for Rule {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::MinLen(n) => f.write_fmt(format_args!("MinLen<{}>", n)),
            Self::MaxLen(n) => f.write_fmt(format_args!("MaxLen<{}>", n)),
            Self::MinVal(n) => f.write_fmt(format_args!("Min<{}>", n)),
            Self::MaxVal(n) => f.write_fmt(format_args!("Max<{}>", n)),
            Self::StartsWith(s) => f.write_fmt(format_args!("StartsWith<{:#?}>", s)),
            Self::EndsWith(s) => f.write_fmt(format_args!("EndsWith<{:#?}", s)),
            Self::MatchesRegexp(s) => f.write_fmt(format_args!("MatchesRegexp<{:#?}", s)),
            Self::Is(Is::Email) => f.write_str("IsEmail"),
            Self::Is(Is::Url) => f.write_str("IsURL"),
        }
    }
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
            (MinLen(a), MinLen(b)) => MinLen((*a).max(*b)),
            (MaxLen(a), MaxLen(b)) => MaxLen((*a).min(*b)),
            (MinVal(a), MinVal(b)) => MinVal(N((*a).max(**b))),
            (MaxVal(a), MaxVal(b)) => MaxVal(N((*a).min(**b))),
            _ => return None,
        })
    }

    pub fn merge_or(&self, other: &Self) -> Option<Self> {
        use Rule::*;
        Some(match (self, other) {
            (MinLen(a), MinLen(b)) => MinLen((*a).min(*b)),
            (MaxLen(a), MaxLen(b)) => MaxLen((*a).max(*b)),
            (MinVal(a), MinVal(b)) => MinVal(N((*a).min(**b))),
            (MaxVal(a), MaxVal(b)) => MaxVal(N((*a).max(**b))),
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

    pub fn supports_type(&self, typ: &Type) -> bool {
        // If this is a WireSpec, unwrap it as it is intended to be transparent.
        let typ = match typ {
            Type::Custom(Custom::WireSpec(spec)) => &spec.underlying,
            _ => typ,
        };

        match self {
            Self::MinLen(_) | Self::MaxLen(_) => {
                matches!(typ, Type::Array(_) | Type::Basic(Basic::String))
            }
            Self::MinVal(_) | Self::MaxVal(_) => matches!(typ, Type::Basic(Basic::Number)),
            Self::StartsWith(_)
            | Self::EndsWith(_)
            | Self::MatchesRegexp(_)
            | Self::Is(Is::Email)
            | Self::Is(Is::Url) => {
                matches!(typ, Type::Basic(Basic::String))
            }
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
                if exprs.len() == 1 {
                    exprs.pop().unwrap()
                } else {
                    Self::And(exprs)
                }
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
                if exprs.len() == 1 {
                    exprs.pop().unwrap()
                } else {
                    Self::Or(exprs)
                }
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

    pub fn supports_type<'a>(
        &'a self,
        typ: &'a Type,
    ) -> Result<(), UnsupportedValidationsError<'a>> {
        match self {
            Self::And(exprs) | Self::Or(exprs) => {
                let mut errors = Vec::new();
                for expr in exprs {
                    if let Err(e) = expr.supports_type(typ) {
                        errors.extend(e.0);
                    }
                }
                if errors.is_empty() {
                    Ok(())
                } else {
                    Err(UnsupportedValidationsError(errors))
                }
            }
            Self::Rule(rule) => {
                if !rule.supports_type(typ) {
                    let v = UnsupportedValidation { typ, rule };
                    Err(UnsupportedValidationsError(vec![v]))
                } else {
                    Ok(())
                }
            }
        }
    }
}

#[derive(Debug, Clone)]
pub struct UnsupportedValidation<'a> {
    pub typ: &'a Type,
    pub rule: &'a Rule,
}

#[derive(Debug, Clone)]
pub struct UnsupportedValidationsError<'a>(pub Vec<UnsupportedValidation<'a>>);

impl Display for UnsupportedValidation<'_> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_fmt(format_args!(
            "{} cannot be applied to {}",
            self.rule, self.typ
        ))
    }
}

impl Display for UnsupportedValidationsError<'_> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if self.0.len() == 1 {
            write!(f, "unsupported validation: {}", &self.0[0])
        } else {
            f.write_str("unsupported validation: ")?;
            for (i, rule) in self.0.iter().enumerate() {
                if i > 0 {
                    f.write_str(", ")?;
                }
                write!(f, "{}", rule)?;
            }
            Ok(())
        }
    }
}

impl Error for UnsupportedValidationsError<'_> {}
impl Error for UnsupportedValidation<'_> {}

#[derive(Debug, Clone, Copy, Serialize)]
pub struct N(pub f64);

impl Display for N {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

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

        {
            let expr = Expr::Or(vec![
                Expr::Rule(Rule::MinLen(10)),
                Expr::Rule(Rule::MinLen(30)),
            ]);
            let simplified = expr.simplify();
            assert_eq!(simplified, Expr::Rule(Rule::MinLen(10)));
        }

        {
            let expr = Expr::And(vec![
                Expr::Rule(Rule::MaxVal(N(10.0))),
                Expr::Rule(Rule::MaxVal(N(30.0))),
            ]);
            let simplified = expr.simplify();
            assert_eq!(simplified, Expr::Rule(Rule::MaxVal(N(10.0))));
        }
    }
}
