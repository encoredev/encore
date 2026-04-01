use crate::api::Request;
use crate::error::coerce_to_api_error;
use crate::napi_util::{await_promise, OnceSender, PromiseHandler};
use crate::pvalue::parse_pvalues;
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use encore_runtime_core::api::HandlerResponse;
use encore_runtime_core::api::{self, HandlerResponseInner};
use napi::{Env, JsFunction, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

#[napi]
pub struct Gateway {
    #[allow(dead_code)]
    gateway: Option<api::gateway::Gateway>,
}

impl Gateway {
    pub fn new(
        env: Env,
        gateway: Option<api::gateway::Gateway>,
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
            let (tx, rx) = tokio::sync::oneshot::channel();
            let tx = OnceSender::new(tx);

            // Call the handler.
            let req = Request::new(req);
            self.handler.call(
                AuthMessage { tx, req },
                ThreadsafeFunctionCallMode::Blocking,
            );

            // Wait for a response.
            match rx.await {
                Ok(Ok(resp)) => Ok(resp),
                Ok(Err(err)) => Err(err),
                Err(_) => Err(api::Error::internal(anyhow::anyhow!(
                    "handler did not respond",
                ))),
            }
        })
    }
}

struct AuthMessage {
    req: Request,
    tx: OnceSender<HandlerResponse>,
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
            ctx.value.tx.send(res);
            Ok(())
        }
    }
}

#[derive(Debug, Clone, Copy)]
struct AuthPromiseHandler;

impl PromiseHandler for AuthPromiseHandler {
    type Output = HandlerResponse;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output {
        let Some(val) = val else {
            return Ok(HandlerResponseInner {
                payload: None,
                extra_headers: None,
                status: None,
            });
        };
        match parse_pvalues(val) {
            Ok(val) => Ok(HandlerResponseInner {
                payload: val,
                extra_headers: None,
                status: None,
            }),
            Err(err) => self.error(env, err),
        }
    }

    fn reject(&self, env: Env, val: napi::JsUnknown) -> Self::Output {
        Err(coerce_to_api_error(env, val)?)
    }

    fn error(&self, _: Env, err: napi::Error) -> Self::Output {
        Err(api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some(err.to_string()),
            stack: None,
            details: None,
        })
    }
}
