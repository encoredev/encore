use std::fmt::Display;

use encore_runtime_core::api::ErrCode;
use napi::{bindgen_prelude::spawn, Either, Env, JsFunction, JsObject, JsUnknown};

pub trait FromJsUnknown {
    fn try_from_js_unknown(env: &Env, value: JsUnknown) -> napi::Result<Self>
    where
        Self: Sized;
}

impl<D> FromJsUnknown for D
where
    D: serde::de::DeserializeOwned,
{
    fn try_from_js_unknown(env: &Env, value: JsUnknown) -> napi::Result<Self>
    where
        Self: Sized,
    {
        env.from_js_value(value)
    }
}

pub trait FromNapiError {
    fn from_napi_error(error: napi::Error) -> Self
    where
        Self: Sized;
}

impl FromNapiError for encore_runtime_core::api::Error {
    fn from_napi_error(error: napi::Error) -> Self {
        Self {
            code: ErrCode::Internal,
            message: ErrCode::Internal.default_public_message().into(),
            internal_message: Some(error.to_string()),
            stack: None,
        }
    }
}

pub trait PromiseHandler: Clone + Send + Sync + 'static {
    type Output: Send + 'static;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output;
    fn reject(&self, env: Env, val: napi::JsUnknown) -> Self::Output;
    fn error(&self, env: Env, err: napi::Error) -> Self::Output;
}

pub fn await_promise<T, H>(
    env: Env,
    result: JsUnknown,
    tx: tokio::sync::mpsc::Sender<T>,
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
                let cb = env.create_function_from_closure("callback", move |ctx| {
                    let handler = handler.clone();
                    let res = match ctx.try_get::<JsUnknown>(0) {
                        Ok(Either::A(success)) => handler.resolve(env, Some(success)),
                        Ok(Either::B(_)) => handler.resolve(env, None),
                        Err(err) => handler.error(env, err),
                    };

                    let tx = tx.clone();
                    spawn(async move {
                        _ = tx.send(res).await;
                    });

                    ctx.env.get_undefined()
                })?;
                cb
            };

            let eb = {
                let handler = handler.clone();
                let tx = tx.clone();
                env.create_function_from_closure("error_callback", move |ctx| {
                    let res = match ctx.get(0) {
                        Ok(exception) => handler.reject(env, exception),
                        Err(err) => handler.error(env, err),
                    };

                    let tx = tx.clone();
                    spawn(async move {
                        _ = tx.send(res).await;
                    });
                    ctx.env.get_undefined()
                })?
            };

            then.call(Some(&result), &[cb, eb])?;
        } else {
            let res = handler.resolve(env, Some(result));
            let tx = tx.clone();
            spawn(async move {
                _ = tx.send(res).await;
            });
        }

        Ok(())
    };

    let tx = outer_tx.clone();
    inner().unwrap_or_else(|err| {
        let res = outer_handler.error(env, err);
        spawn(async move {
            _ = tx.send(res).await;
        });
    });
}

#[derive(Debug)]
pub struct JSError {
    pub message: String,
}

impl FromJsUnknown for JSError {
    fn try_from_js_unknown(env: &Env, value: JsUnknown) -> napi::Result<Self>
    where
        Self: Sized,
    {
        let value: JsObject = value.try_into()?;
        let message: JsUnknown = value.get_named_property("message")?;
        let message = message.coerce_to_string()?;
        let message: String = env.from_js_value(message)?;
        Ok(Self { message })
    }
}

impl FromNapiError for JSError {
    fn from_napi_error(error: napi::Error) -> Self
    where
        Self: Sized,
    {
        Self {
            message: error.to_string(),
        }
    }
}

impl std::error::Error for JSError {}

impl Display for JSError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}
