use crate::{api::PValue, encore::parser::schema::v1 as schema};
use thiserror::Error;

use super::BasicOrValue;

#[derive(Debug, Clone)]
pub struct Validation {
    pub bov: BasicOrValue,
    pub expr: Expr,
}

impl Validation {
    pub fn validate<'a>(&'a self, val: &'a PValue) -> Result<(), Error<'a>> {
        self.expr.validate(val)
    }
}

#[derive(Debug, Clone)]
pub enum Expr {
    Rule(Rule),
    And(Vec<Expr>),
    Or(Vec<Expr>),
}

impl Expr {
    pub fn validate<'a>(&'a self, val: &'a PValue) -> Result<(), Error<'a>> {
        match self {
            Expr::Rule(rule) => rule.validate(val),
            Expr::And(exprs) => {
                for expr in exprs {
                    expr.validate(val)?;
                }
                Ok(())
            }
            Expr::Or(exprs) => {
                let mut first_err = None;
                for expr in exprs {
                    match expr.validate(val) {
                        Ok(()) => return Ok(()),
                        Err(err) => {
                            if first_err.is_none() {
                                first_err = Some(err);
                            }
                        }
                    }
                }
                match first_err {
                    Some(err) => Err(err),
                    None => Ok(()),
                }
            }
        }
    }
}

#[derive(Debug, Clone)]
pub enum Rule {
    MinLen(u64),
    MaxLen(u64),
    MinVal(f64),
    MaxVal(f64),
    StartsWith(String),
    EndsWith(String),
    MatchesRegexp(regex::Regex),
    Is(Is),
}

#[derive(Debug, Clone)]
pub enum Is {
    Email,
    Url,
}

#[derive(Error, Debug)]
pub enum Error<'a> {
    #[error("length too short (got {got}, expected at least {min})")]
    MinLen { got: usize, min: usize },
    #[error("length too long (got {got}, expected at most {max})")]
    MaxLen { got: usize, max: usize },

    #[error("value must be at least {min} (got {got})")]
    MinVal {
        got: &'a serde_json::Number,
        min: f64,
    },
    #[error("value must be at most {max} (got {got})")]
    MaxVal {
        got: &'a serde_json::Number,
        max: f64,
    },

    #[error("value does not match the regexp {regexp:#?}")]
    MatchesRegexp { regexp: &'a str },

    #[error("value does not start with {want:#?}")]
    StartsWith { want: &'a str },

    #[error("value does not end with {want:#?}")]
    EndsWith { want: &'a str },

    #[error("value is not {expected}")]
    Is { expected: &'a str },

    #[error("unexpected type (got {got}, expected {want})")]
    UnexpectedType { got: &'a PValue, want: &'a str },
}

impl Rule {
    pub fn validate<'a>(&'a self, val: &'a PValue) -> Result<(), Error<'a>> {
        match self {
            Rule::MinLen(min_len) => match val {
                PValue::Array(arr) => {
                    if arr.len() < *min_len as usize {
                        Err(Error::MinLen {
                            got: arr.len(),
                            min: *min_len as usize,
                        })
                    } else {
                        Ok(())
                    }
                }
                PValue::String(str) => {
                    if str.len() < *min_len as usize {
                        Err(Error::MinLen {
                            got: str.len(),
                            min: *min_len as usize,
                        })
                    } else {
                        Ok(())
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string or array",
                }),
            },

            Rule::MaxLen(max_len) => match val {
                PValue::Array(arr) => {
                    if arr.len() > *max_len as usize {
                        Err(Error::MaxLen {
                            got: arr.len(),
                            max: *max_len as usize,
                        })
                    } else {
                        Ok(())
                    }
                }
                PValue::String(str) => {
                    if str.len() > *max_len as usize {
                        Err(Error::MaxLen {
                            got: str.len(),
                            max: *max_len as usize,
                        })
                    } else {
                        Ok(())
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string or array",
                }),
            },

            Rule::MinVal(min_val) => match val {
                PValue::Number(num) => {
                    let bad = if num.is_i64() {
                        num.as_i64().unwrap() < *min_val as i64
                    } else if num.is_u64() {
                        num.as_u64().unwrap() < *min_val as u64
                    } else if num.is_f64() {
                        num.as_f64().unwrap() < *min_val as f64
                    } else {
                        return Err(Error::UnexpectedType {
                            got: val,
                            want: "number",
                        });
                    };
                    if bad {
                        Err(Error::MinVal {
                            got: num,
                            min: *min_val,
                        })
                    } else {
                        Ok(())
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string or array",
                }),
            },

            Rule::MaxVal(max_val) => match val {
                PValue::Number(num) => {
                    let bad = if num.is_i64() {
                        num.as_i64().unwrap() > *max_val as i64
                    } else if num.is_u64() {
                        num.as_u64().unwrap() > *max_val as u64
                    } else if num.is_f64() {
                        num.as_f64().unwrap() > *max_val as f64
                    } else {
                        return Err(Error::UnexpectedType {
                            got: val,
                            want: "number",
                        });
                    };
                    if bad {
                        Err(Error::MaxVal {
                            got: num,
                            max: *max_val,
                        })
                    } else {
                        Ok(())
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string or array",
                }),
            },

            Rule::StartsWith(want) => match val {
                PValue::String(got) => {
                    if got.starts_with(got) {
                        Ok(())
                    } else {
                        Err(Error::StartsWith { want })
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string",
                }),
            },

            Rule::EndsWith(want) => match val {
                PValue::String(got) => {
                    if got.ends_with(got) {
                        Ok(())
                    } else {
                        Err(Error::EndsWith { want })
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string",
                }),
            },

            Rule::MatchesRegexp(re) => match val {
                PValue::String(str) => {
                    if re.is_match(str) {
                        Ok(())
                    } else {
                        Err(Error::MatchesRegexp {
                            regexp: re.as_str(),
                        })
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string",
                }),
            },

            Rule::Is(Is::Email) => match val {
                PValue::String(str) => {
                    let email = email_address::EmailAddress::parse_with_options(
                        str,
                        email_address::Options::default().without_display_text(),
                    );
                    match email {
                        Ok(_) => Ok(()),
                        Err(_) => Err(Error::Is { expected: "email" }),
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string",
                }),
            },

            Rule::Is(Is::Url) => match val {
                PValue::String(str) => {
                    let u = url::Url::parse(str);
                    match u {
                        Ok(_) => Ok(()),
                        Err(_) => Err(Error::Is { expected: "url" }),
                    }
                }

                other => Err(Error::UnexpectedType {
                    got: other,
                    want: "string",
                }),
            },
        }
    }
}

impl TryFrom<&schema::ValidationExpr> for Expr {
    type Error = anyhow::Error;

    fn try_from(expr: &schema::ValidationExpr) -> Result<Self, Self::Error> {
        let Some(expr) = &expr.expr else {
            return Err(anyhow::anyhow!("missing expr"));
        };

        use schema::validation_expr::Expr as PbExpr;

        match expr {
            PbExpr::Rule(rule) => Ok(Expr::Rule(rule.try_into()?)),
            PbExpr::And(expr) => {
                let mut and = Vec::new();
                for expr in &expr.exprs {
                    and.push(expr.try_into()?);
                }
                Ok(Expr::And(and))
            }
            PbExpr::Or(expr) => {
                let mut or = Vec::new();
                for expr in &expr.exprs {
                    or.push(expr.try_into()?);
                }
                Ok(Expr::Or(or))
            }
        }
    }
}

impl TryFrom<&schema::ValidationRule> for Rule {
    type Error = anyhow::Error;

    fn try_from(rule: &schema::ValidationRule) -> Result<Self, Self::Error> {
        let Some(rule) = &rule.rule else {
            return Err(anyhow::anyhow!("missing validation rule"));
        };

        use schema::validation_rule::Is as PbIs;
        use schema::validation_rule::Rule as PbRule;
        match rule {
            PbRule::MinLen(val) => Ok(Rule::MinLen(*val)),
            PbRule::MaxLen(val) => Ok(Rule::MaxLen(*val)),
            PbRule::MinVal(val) => Ok(Rule::MinVal(*val)),
            PbRule::MaxVal(val) => Ok(Rule::MaxVal(*val)),
            PbRule::StartsWith(val) => Ok(Rule::StartsWith(val.clone())),
            PbRule::EndsWith(val) => Ok(Rule::EndsWith(val.clone())),
            PbRule::MatchesRegexp(val) => {
                let re = regex::Regex::new(val)?;
                Ok(Rule::MatchesRegexp(re))
            }
            PbRule::Is(is) => Ok(Rule::Is(match PbIs::try_from(*is)? {
                PbIs::Unknown => anyhow::bail!("unknown 'is' rule"),
                PbIs::Email => Is::Email,
                PbIs::Url => Is::Url,
            })),
        }
    }
}
