use crate::api::Request;
use crate::log::parse_js_stack;
use crate::napi_util::{await_promise, PromiseHandler};
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use encore_runtime_core::api;
use encore_runtime_core::api::schema;
use napi::bindgen_prelude::spawn;
use napi::{Env, JsFunction, JsUnknown, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

#[napi]
pub struct Gateway {
    #[allow(dead_code)]
    gateway: Option<Arc<api::gateway::Gateway>>,
}

impl Gateway {
    pub fn new(
        env: Env,
        gateway: Option<Arc<api::gateway::Gateway>>,
        cfg: GatewayConfig,
    ) -> napi::Result<Self> {
        if let Some(gw) = &gateway {
            if let Some(auth) = gw.auth_handler() {
                if let Some(handler) = cfg.auth {
                    let handler: Arc<dyn api::TypedHandler> = to_auth_handler(env, handler)?;

                    auth.set_local_handler_impl(Some(handler));
                }
            }
        }

        Ok(Self { gateway })
    }
}

#[napi(object)]
pub struct GatewayConfig {
    pub auth: Option<JsFunction>,
}

fn to_auth_handler(env: Env, handler: JsFunction) -> napi::Result<Arc<JSAuthHandler>> {
    let tsfn = ThreadsafeFunction::create(
        env.raw(),
        // SAFETY: `handler` is a valid JS function.
        unsafe { handler.raw() },
        0,
        resolve_on_js_thread,
    )?;

    Ok(Arc::new(JSAuthHandler { handler: tsfn }))
}

pub struct JSAuthHandler {
    handler: ThreadsafeFunction<AuthMessage>,
}

impl api::TypedHandler for JSAuthHandler {
    fn call(
        self: Arc<Self>,
        req: api::HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = api::HandlerResponse> + Send + 'static>> {
        Box::pin(async move {
            // Create a one-shot channel
            let (tx, mut rx) = tokio::sync::mpsc::channel(1);

            // Call the handler.
            let req = Request::new(req);
            self.handler.call(
                AuthMessage { tx, req },
                ThreadsafeFunctionCallMode::Blocking,
            );

            // Wait for a response.
            match rx.recv().await {
                Some(Ok(resp)) => Ok(resp),
                Some(Err(err)) => Err(err),
                None => Err(api::Error::internal(anyhow::anyhow!(
                    "handler did not respond",
                ))),
            }
        })
    }
}

struct AuthMessage {
    req: Request,
    tx: tokio::sync::mpsc::Sender<Result<schema::JSONPayload, api::Error>>,
}

fn resolve_on_js_thread(ctx: ThreadSafeCallContext<AuthMessage>) -> napi::Result<()> {
    let req = ctx.value.req.into_instance(ctx.env)?;
    let handler = AuthPromiseHandler;
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

#[derive(Debug, Clone, Copy)]
struct AuthPromiseHandler;

impl PromiseHandler for AuthPromiseHandler {
    type Output = Result<schema::JSONPayload, api::Error>;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output {
        let Some(val) = val else { return Ok(None) };
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
