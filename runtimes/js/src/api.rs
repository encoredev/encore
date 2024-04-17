use crate::log::parse_js_stack;
use crate::napi_util::{await_promise, PromiseHandler};
use crate::raw_api;
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use encore_runtime_core::api;
use encore_runtime_core::api::schema;
use encore_runtime_core::model::RequestData;
use napi::bindgen_prelude::spawn;
use napi::{Env, JsFunction, JsUnknown, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

#[napi(object)]
pub struct APIRoute {
    pub service: String,
    pub name: String,
    pub raw: bool,
    pub handler: JsFunction,
}

pub fn new_api_handler(
    env: Env,
    func: JsFunction,
    raw: bool,
) -> napi::Result<Arc<dyn api::BoxedHandler>> {
    if raw {
        return raw_api::new_handler(env, func);
    }
    let handler = ThreadsafeFunction::create(
        env.raw(),
        // SAFETY: `handler` is a valid JS function.
        unsafe { func.raw() },
        0,
        typed_resolve_on_js_thread,
    )?;
    Ok(Arc::new(JSTypedHandler { handler }))
}

#[napi]
pub struct Request {
    pub(crate) inner: Arc<encore_runtime_core::model::Request>,
}

#[napi]
impl Request {
    #[napi]
    pub fn payload(&self, env: Env) -> napi::Result<JsUnknown> {
        match &self.inner.data {
            RequestData::RPC(data) => env.to_js_value(&data.parsed_payload),
            RequestData::Auth(data) => env.to_js_value(&data.parsed_payload),
            RequestData::PubSub(data) => env.to_js_value(&data.parsed_payload),
        }
    }

    #[napi]
    pub fn get_auth_data(&self, env: Env) -> napi::Result<JsUnknown> {
        use RequestData::*;
        match &self.inner.data {
            RPC(data) => env.to_js_value(&data.auth_data),
            Auth(_) | PubSub(_) => env.get_null().map(|val| val.into_unknown()),
        }
    }

    #[napi]
    pub fn method(&self) -> Option<&'static str> {
        match &self.inner.data {
            RequestData::RPC(data) => Some(data.method.as_str()),
            RequestData::Auth(_) => None,
            RequestData::PubSub(_) => None,
        }
    }

    #[napi]
    pub fn path(&self) -> Option<String> {
        match &self.inner.data {
            RequestData::RPC(data) => Some(data.path.clone()),
            RequestData::Auth(_) => None,
            RequestData::PubSub(_) => None,
        }
    }
}

#[derive(Debug, Clone, Copy)]
struct APIPromiseHandler;

impl PromiseHandler for APIPromiseHandler {
    type Output = Result<schema::JSONPayload, api::Error>;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output {
        let Some(val) = val else {
            return Ok(None);
        };
        match env.from_js_value(val) {
            Ok(val) => Ok(val),
            Err(err) => self.error(env, err),
        }
    }

    fn reject(&self, env: Env, val: napi::JsUnknown) -> Self::Output {
        let obj = val.coerce_to_object().map_err(|_| api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some("an unknown exception was thrown".into()),
            stack: None,
        })?;

        // Get the message field.
        let mut message: String = obj
            .get_named_property::<JsUnknown>("message")
            .and_then(|val| val.coerce_to_string())
            .and_then(|val| env.from_js_value(val))
            .map_err(|_| api::Error {
                code: api::ErrCode::Internal,
                message: api::ErrCode::Internal.default_public_message().into(),
                internal_message: Some("an unknown exception was thrown".into()),
                stack: None,
            })?;

        // Get the error code field.
        let code: api::ErrCode = obj
            .get_named_property::<JsUnknown>("code")
            .and_then(|val| val.coerce_to_string())
            .and_then(|val| env.from_js_value::<String, _>(val))
            .map(|val| {
                val.parse::<api::ErrCode>()
                    .unwrap_or(api::ErrCode::Internal)
            })
            .unwrap_or(api::ErrCode::Internal);

        // Get the JS stack
        let stack = obj
            .get_named_property::<JsUnknown>("stack")
            .and_then(|val| parse_js_stack(&env, val))
            .map(|val| Some(val))
            .unwrap_or(None);

        let mut internal_message = None;
        if code == api::ErrCode::Internal {
            internal_message = Some(message);
            message = api::ErrCode::Internal.default_public_message().into();
        }

        Err(api::Error {
            code,
            message,
            stack,
            internal_message,
        })
    }

    fn error(&self, _: Env, err: napi::Error) -> Self::Output {
        Err(api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some(err.to_string()),
            stack: None,
        })
    }
}

struct TypedRequestMessage {
    req: Request,
    tx: tokio::sync::mpsc::Sender<Result<schema::JSONPayload, api::Error>>,
}

pub struct JSTypedHandler {
    handler: ThreadsafeFunction<TypedRequestMessage>,
}

impl api::BoxedHandler for JSTypedHandler {
    fn call(
        self: Arc<Self>,
        req: api::HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = api::ResponseData> + Send + 'static>> {
        Box::pin(async move {
            // Create a one-shot channel
            let (tx, mut rx) = tokio::sync::mpsc::channel(1);

            // Call the handler.
            let req = Request { inner: req };
            self.handler.call(
                TypedRequestMessage { tx, req },
                ThreadsafeFunctionCallMode::Blocking,
            );

            // Wait for a response.
            let resp = match rx.recv().await {
                Some(Ok(resp)) => Ok(resp),
                Some(Err(err)) => Err(err),
                None => Err(api::Error::internal(anyhow::anyhow!(
                    "handler did not respond",
                ))),
            };

            api::ResponseData::Typed(resp)
        })
    }
}

fn typed_resolve_on_js_thread(ctx: ThreadSafeCallContext<TypedRequestMessage>) -> napi::Result<()> {
    let req = ctx.value.req.into_instance(ctx.env)?;
    let handler = APIPromiseHandler;
    match ctx.callback.unwrap().call(None, &[req]) {
        Ok(result) => {
            await_promise(ctx.env, result, ctx.value.tx.clone(), handler);
            Ok(())
        }
        Err(err) => {
            let res = handler.error(ctx.env, err);
            spawn(async move {
                _ = ctx.value.tx.send(res).await;
            });
            Ok(())
        }
    }
}
