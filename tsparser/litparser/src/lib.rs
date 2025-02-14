use duration_string::DurationString;
use num_bigint::{BigInt, ToBigInt};
use std::{
    error::Error,
    fmt::{Debug, Display},
    ops::{Deref, DerefMut},
    path::{Component, PathBuf},
};
use swc_common::{errors::HANDLER, pass::Either, util::take::Take, Span, Spanned};
use swc_ecma_ast as ast;

#[derive(Debug, Clone, Hash)]
pub struct ParseError {
    pub span: Span,
    pub message: String,
}

impl Error for ParseError {}

impl Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl ParseError {
    pub fn report(self) {
        HANDLER.with(|handler| {
            handler.span_err(self.span, &self.message);
        });
    }
}

#[macro_export]
macro_rules! report_and_continue {
    ($e:expr) => {
        match $e {
            Ok(v) => v,
            Err(err) => {
                err.report();
                continue;
            }
        }
    };
}

#[macro_export]
macro_rules! report_and_return {
    ($e:expr) => {
        match $e {
            Ok(v) => v,
            Err(err) => {
                err.report();
                return;
            }
        }
    };
}

pub trait ToParseErr {
    fn parse_err<S: Into<String>>(&self, message: S) -> ParseError;
}

impl<T> ToParseErr for T
where
    T: Spanned,
{
    fn parse_err<S: Into<String>>(&self, message: S) -> ParseError {
        ParseError {
            span: self.span(),
            message: message.into(),
        }
    }
}

pub type ParseResult<T> = Result<T, ParseError>;

pub trait LitParser: Sized {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self>;
}

impl<T> LitParser for Sp<T>
where
    T: LitParser,
{
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        let res = T::parse_lit(input)?;
        Ok(Sp(input.span(), res))
    }
}

impl LitParser for String {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => Ok(str.value.to_string()),
            _ => Err(input.parse_err("expected string literal")),
        }
    }
}

impl LitParser for bool {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Bool(b)) => Ok(b.value),
            _ => Err(input.parse_err("expected boolean literal")),
        }
    }
}

impl LitParser for i32 {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        let big = parse_const_bigint(input)?;
        big.try_into()
            .map_err(|_| input.parse_err("expected number literal"))
    }
}

impl LitParser for u32 {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        let big = parse_const_bigint(input)?;
        big.try_into()
            .map_err(|_| input.parse_err("expected unsigned number literal"))
    }
}

impl LitParser for i64 {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        let big = parse_const_bigint(input)?;
        big.try_into()
            .map_err(|_| input.parse_err("expected number literal"))
    }
}

impl LitParser for u64 {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        let big = parse_const_bigint(input)?;
        big.try_into()
            .map_err(|_| input.parse_err("expected unsigned number literal"))
    }
}

impl LitParser for ast::Expr {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        Ok(input.clone())
    }
}

impl<T> LitParser for Option<T>
where
    T: LitParser,
{
    fn parse_lit(input: &ast::Expr) -> ParseResult<Option<T>> {
        let t = T::parse_lit(input)?;
        Ok(Some(t))
    }
}

impl<L, R> LitParser for Either<L, R>
where
    L: LitParser,
    R: LitParser,
{
    fn parse_lit(input: &ast::Expr) -> ParseResult<Either<L, R>> {
        let res = L::parse_lit(input)
            .map(Either::Left)
            .or_else(|_| R::parse_lit(input).map(Either::Right))?;

        Ok(res)
    }
}

impl LitParser for std::time::Duration {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => {
                let dur = DurationString::try_from(str.value.to_string())
                    .map_err(|e| str.parse_err(e))?;
                Ok(dur.into())
            }
            _ => Err(input.parse_err("expected duration string literal")),
        }
    }
}

impl<T> LitParser for Vec<T>
where
    T: LitParser,
{
    fn parse_lit(input: &swc_ecma_ast::Expr) -> ParseResult<Self> {
        match input {
            ast::Expr::Array(array) => {
                let mut vec = Vec::new();
                for elem in &array.elems {
                    if let Some(expr) = elem {
                        let parsed_elem = T::parse_lit(&expr.expr)?;
                        vec.push(parsed_elem);
                    } else {
                        return Err(array.span.parse_err("expected array element"));
                    }
                }
                Ok(vec)
            }
            _ => Err(input.parse_err("expected array literal")),
        }
    }
}

/// Represents a local, relative path (without ".." or a root).
#[derive(Debug, Clone)]
pub struct LocalRelPath {
    pub span: Span,
    pub buf: PathBuf,
}

impl Spanned for LocalRelPath {
    fn span(&self) -> Span {
        self.span
    }
}

impl LocalRelPath {
    pub fn try_from<S: AsRef<str>>(sp: Span, str: S) -> ParseResult<Self> {
        let str = str.as_ref();
        let path = PathBuf::from(str);
        for c in path.components() {
            match c {
                Component::CurDir => {}
                Component::Normal(_) => {}
                _ => return Err(sp.parse_err("expected a local relative path")),
            }
        }
        Ok(LocalRelPath {
            span: sp,
            buf: clean_path::clean(path),
        })
    }
}

impl LitParser for LocalRelPath {
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => LocalRelPath::try_from(str.span, &str.value),
            _ => Err(input.parse_err("expected a local relative path")),
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
    fn parse_lit(input: &ast::Expr) -> ParseResult<Self> {
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

fn parse_const_bigint(expr: &ast::Expr) -> ParseResult<BigInt> {
    match expr {
        ast::Expr::Lit(ast::Lit::Num(num)) => {
            let int = num.value as i64;
            if int as f64 != num.value {
                return Err(num.parse_err("expected integer literal"));
            }
            let Some(big) = int.to_bigint() else {
                return Err(num.parse_err("integer too large"));
            };
            Ok(big)
        }
        ast::Expr::Unary(unary) => match unary.op {
            ast::UnaryOp::Minus => {
                let x = parse_const_bigint(&unary.arg)?;
                Ok(-x)
            }
            ast::UnaryOp::Plus => parse_const_bigint(&unary.arg),
            _ => Err(unary.parse_err(format!("unsupported unary operator {:?}", unary.op))),
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
                        Err(bin.parse_err("expected integer division"))
                    }
                }
                _ => Err(bin.parse_err(format!("expected arithmetic operator, got {:?}", bin.op))),
            }
        }
        _ => Err(expr.parse_err("expected integer literal")),
    }
}

pub struct Sp<T>(Span, T);

impl<T> Clone for Sp<T>
where
    T: Clone,
{
    fn clone(&self) -> Self {
        Sp(self.0, self.1.clone())
    }
}

impl<T> Sp<T> {
    pub fn new(sp: Span, val: T) -> Self {
        Self(sp, val)
    }

    pub fn with_dummy(val: T) -> Self {
        Self::new(Span::dummy(), val)
    }

    pub fn with<U>(&self, val: U) -> Sp<U> {
        Sp::new(self.0, val)
    }

    pub fn split(self) -> (Span, T) {
        (self.0, self.1)
    }

    pub fn span(&self) -> Span {
        self.0
    }

    pub fn take(self) -> T {
        self.1
    }

    pub fn map<F, U>(self, f: F) -> Sp<U>
    where
        F: FnOnce(T) -> U,
    {
        Sp(self.0, f(self.1))
    }

    pub fn get(&self) -> &T {
        &self.1
    }

    pub fn as_deref(&self) -> &T::Target
    where
        T: Deref,
    {
        self.1.deref()
    }
}

impl<T, E> Sp<Result<T, E>> {
    pub fn transpose(self) -> Result<Sp<T>, E> {
        match self.1 {
            Ok(inner) => Ok(Sp(self.0, inner)),
            Err(err) => Err(err),
        }
    }
}

impl<T> AsRef<T> for Sp<T> {
    fn as_ref(&self) -> &T {
        &self.1
    }
}

impl<T> AsMut<T> for Sp<T> {
    fn as_mut(&mut self) -> &mut T {
        &mut self.1
    }
}

impl<T> Deref for Sp<T> {
    type Target = T;

    fn deref(&self) -> &Self::Target {
        &self.1
    }
}

impl<T> DerefMut for Sp<T> {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.1
    }
}

impl<T> PartialEq for Sp<T>
where
    T: PartialEq,
{
    fn eq(&self, other: &Self) -> bool {
        self.1 == other.1
    }
}

impl<T> Eq for Sp<T> where T: Eq {}

impl<T> PartialOrd for Sp<T>
where
    T: PartialOrd,
{
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        self.1.partial_cmp(&other.1)
    }
}

impl<T> Ord for Sp<T>
where
    T: Ord,
{
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        self.1.cmp(&other.1)
    }
}

impl<T> Copy for Sp<T> where T: Copy {}

impl<T> Debug for Sp<T>
where
    T: Debug,
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        self.1.fmt(f)
    }
}

impl<T> Display for Sp<T>
where
    T: Display,
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        self.1.fmt(f)
    }
}

impl<T> From<T> for Sp<T>
where
    T: Spanned,
{
    fn from(value: T) -> Self {
        Self(value.span(), value)
    }
}

impl<T> Spanned for Sp<T> {
    fn span(&self) -> Span {
        self.0
    }
}
