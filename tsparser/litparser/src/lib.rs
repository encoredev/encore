use anyhow::Result;
use duration_string::DurationString;
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
        match input {
            ast::Expr::Lit(ast::Lit::Num(num)) => {
                let int = num.value as i32;
                if int as f64 != num.value {
                    anyhow::bail!("expected integer literal, got {:?}", input)
                }
                Ok(int)
            }
            _ => anyhow::bail!("expected number literal, got {:?}", input),
        }
    }
}

impl LitParser for u32 {
    fn parse_lit(input: &ast::Expr) -> Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Num(num)) => {
                if num.value < 0.0 {
                    anyhow::bail!("expected non-negative integer literal, got {:?}", input)
                }
                let int = num.value as u32;
                if int as f64 != num.value {
                    anyhow::bail!("expected integer literal, got {:?}", input)
                }
                Ok(int)
            }
            _ => anyhow::bail!("expected number literal, got {:?}", input),
        }
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
