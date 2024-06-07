use std::fmt::Display;

use napi::{Either, Env, JsFunction, JsObject, JsUnknown};

pub trait PromiseHandler: Clone + Send + Sync + 'static {
    type Output: Send + 'static;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output;
    fn reject(&self, env: Env, val: napi::JsUnknown) -> Self::Output;
    fn error(&self, env: Env, err: napi::Error) -> Self::Output;
}

pub fn await_promise<T, H>(
    env: Env,
    result: JsUnknown,
    tx: tokio::sync::mpsc::UnboundedSender<T>,
    handler: H,
) where
    H: PromiseHandler<Output = T>,
    T: Send + 'static,
{
    // An inner function to enable using ? for error handling.
    let outer_handler = handler.clone();
    let outer_tx = tx.clone();
    let inner = move || -> napi::Result<()> {
        // If the result is a promise, wait for it to resolve, and send the result to the channel.
        // Otherwise, send the result immediately.
        if result.is_promise()? {
            let result: JsObject = result.try_into()?;
            let then: JsFunction = result.get_named_property("then")?;

            let cb = {
                let handler = handler.clone();
                let tx = tx.clone();
                env.create_function_from_closure("callback", move |ctx| {
                    let res = match ctx.try_get::<JsUnknown>(0) {
                        Ok(Either::A(success)) => handler.resolve(env, Some(success)),
                        Ok(Either::B(_)) => handler.resolve(env, None),
                        Err(err) => handler.error(env, err),
                    };

                    _ = tx.send(res);
                    ctx.env.get_undefined()
                })?
            };

            let eb = {
                let handler = handler.clone();
                env.create_function_from_closure("error_callback", move |ctx| {
                    let res = match ctx.get(0) {
                        Ok(exception) => handler.reject(env, exception),
                        Err(err) => handler.error(env, err),
                    };

                    _ = tx.send(res);
                    ctx.env.get_undefined()
                })?
            };

            then.call(Some(&result), &[cb, eb])?;
        } else {
            let res = handler.resolve(env, Some(result));
            _ = tx.send(res);
        }

        Ok(())
    };

    inner().unwrap_or_else(move |err| {
        let res = outer_handler.error(env, err);
        _ = outer_tx.send(res);
    });
}

#[derive(Debug)]
pub struct JSError {
    pub message: String,
}

impl std::error::Error for JSError {}

impl Display for JSError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}
