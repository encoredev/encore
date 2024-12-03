use swc_common::{errors::HANDLER, Span, Spanned};

pub trait ErrReporter {
    fn err(&self, msg: &str);
}

impl<T> ErrReporter for T
where
    T: Spanned,
{
    fn err(&self, msg: &str) {
        HANDLER.with(|h| h.span_err(self.span(), msg));
    }
}

#[derive(Debug)]
pub struct SpErr<E> {
    pub span: Span,
    pub error: E,
}

impl<E> SpErr<E>
where
    E: std::error::Error,
{
    pub fn new(span: Span, error: E) -> Self {
        SpErr { span, error }
    }

    pub fn report(&self) {
        HANDLER.with(|handler| handler.span_err(self.span, &self.error.to_string()))
    }
}

impl<E> std::error::Error for SpErr<E>
where
    E: std::error::Error,
{
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        self.error.source()
    }
}

impl<E> std::fmt::Display for SpErr<E>
where
    E: std::fmt::Display,
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        std::fmt::Display::fmt(&self.error, f)
    }
}

pub trait ErrorWithSpanExt: std::error::Error + Sized {
    fn with_span(self, span: Span) -> SpErr<Self> {
        SpErr::new(span, self)
    }
}

impl<T> ErrorWithSpanExt for T where T: std::error::Error {}
