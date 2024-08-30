use anyhow::Result;
use duration_string::DurationString;
use num_bigint::{BigInt, ToBigInt};
use std::path::{Component, PathBuf};
use swc_ecma_ast as ast;

pub trait LitParser: Sized {
    fn parse_lit(input: &ast::Expr) -> Result<Self>;
}

impl LitParser for String {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => Ok(str.value.to_string()),
            _ => anyhow::bail!("expected string literal, got {:?}", input),
        }
    }
}

impl LitParser for bool {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Bool(b)) => Ok(b.value),
            _ => anyhow::bail!("expected boolean literal, got {:?}", input),
        }
    }
}

impl LitParser for i32 {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        let big = parse_const_bigint(input)?;
        let val: i32 = big
            .try_into()
            .map_err(|_| anyhow::anyhow!("expected number literal, got {:?}", input))?;
        Ok(val)
    }
}

impl LitParser for u32 {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        let big = parse_const_bigint(input)?;
        let val: u32 = big
            .try_into()
            .map_err(|_| anyhow::anyhow!("expected unsigned number literal, got {:?}", input))?;
        Ok(val)
    }
}

impl LitParser for i64 {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        let big = parse_const_bigint(input)?;
        let val: i64 = big
            .try_into()
            .map_err(|_| anyhow::anyhow!("expected number literal, got {:?}", input))?;
        Ok(val)
    }
}

impl LitParser for u64 {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        let big = parse_const_bigint(input)?;
        let val: u64 = big
            .try_into()
            .map_err(|_| anyhow::anyhow!("expected unsigned number literal, got {:?}", input))?;
        Ok(val)
    }
}

impl LitParser for ast::Expr {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        Ok(input.clone())
    }
}

impl<T> LitParser for Option<T>
where
    T: LitParser,
{
    fn parse_lit(input: &ast::Expr) -> Result<Option<T>> {
        let t = T::parse_lit(input)?;
        Ok(Some(t))
    }
}

impl LitParser for std::time::Duration {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => {
                let dur =
                    DurationString::try_from(str.value.to_string()).map_err(anyhow::Error::msg)?;
                Ok(dur.into())
            }
            _ => anyhow::bail!("expected duration string, got {:?}", input),
        }
    }
}

/// Represents a local, relative path (without ".." or a root).
#[derive(Debug, Clone)]
pub struct LocalRelPath(pub PathBuf);

impl LocalRelPath {
    pub fn try_from<S: AsRef<str>>(str: S) -> Result<Self> {
        let str = str.as_ref();
        let path = PathBuf::from(str);
        for c in path.components() {
            match c {
                Component::CurDir => {}
                Component::Normal(_) => {}
                _ => anyhow::bail!("expected a local relative path, got {:?}", str),
            }
        }
        Ok(LocalRelPath(clean_path::clean(path)))
    }
}

impl LitParser for LocalRelPath {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => LocalRelPath::try_from(&str.value),
            _ => anyhow::bail!("expected a local relative path, got {:?}", input),
        }
    }
}

#[derive(Debug)]
pub enum Nullable<T> {
    Present(T),
    Null,
}

impl<T> LitParser for Nullable<T>
where
    T: LitParser,
{
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Null(_)) => Ok(Nullable::Null),
            _ => {
                let t = T::parse_lit(input)?;
                Ok(Nullable::Present(t))
            }
        }
    }
}

impl<T> Clone for Nullable<T>
where
    T: Clone,
{
    fn clone(&self) -> Self {
        match self {
            Nullable::Present(t) => Nullable::Present(t.clone()),
            Nullable::Null => Nullable::Null,
        }
    }
}

fn parse_const_bigint(expr: &ast::Expr) -> Result<BigInt> {
    match expr {
        ast::Expr::Lit(ast::Lit::Num(num)) => {
            let int = num.value as i64;
            if int as f64 != num.value {
                anyhow::bail!("expected integer literal, got float");
            }
            let big = int.to_bigint().ok_or_else(|| {
                anyhow::anyhow!("expected integer literal, got too large integer")
            })?;
            Ok(big)
        }
        ast::Expr::Unary(unary) => match unary.op {
            ast::UnaryOp::Minus => {
                let x = parse_const_bigint(&unary.arg)?;
                Ok(-x)
            }
            ast::UnaryOp::Plus => parse_const_bigint(&unary.arg),
            _ => anyhow::bail!("unsupported unary operator {:?}", unary.op),
        },
        ast::Expr::Bin(bin) => {
            let x = parse_const_bigint(&bin.left)?;
            let y = parse_const_bigint(&bin.right)?;
            match bin.op {
                ast::BinaryOp::Add => Ok(x + y),
                ast::BinaryOp::Sub => Ok(x - y),
                ast::BinaryOp::Mul => Ok(x * y),
                ast::BinaryOp::Mod => Ok(x % y),
                ast::BinaryOp::Div => {
                    // Does it divide evenly?
                    use num_integer::Integer;
                    use num_traits::Zero;
                    let (quo, remainder) = x.div_rem(&y);
                    if remainder.is_zero() {
                        Ok(quo)
                    } else {
                        anyhow::bail!("expected integer division, got {:?}", expr)
                    }
                }
                _ => anyhow::bail!("expected arithmetic operator, got {:?}", bin.op),
            }
        }
        _ => anyhow::bail!("expected integer literal, got {:?}", expr),
    }
}
