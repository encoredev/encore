use swc_common::{errors::HANDLER, Spanned};

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
