use crate::error::coerce_to_api_error;
use crate::headers::parse_header_map;
use crate::napi_util::{await_promise, PromiseHandler};
use crate::pvalue::{encode_auth_payload, encode_request_payload, parse_pvalues, pvalues_or_null};
use crate::request_meta::RequestMeta;
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use crate::{raw_api, request_meta, websocket_api};
use encore_runtime_core::api::{self, HandlerResponse, HandlerResponseInner};
use encore_runtime_core::model::RequestData;
use napi::{Env, JsFunction, JsObject, JsUnknown, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

#[napi(object)]
pub struct APIRoute {
    pub service: String,
    pub name: String,
    pub raw: bool,
    pub streaming_request: bool,
    pub streaming_response: bool,
    pub handler: JsFunction,
}

pub fn new_api_handler(
    env: Env,
    func: JsFunction,
    raw: bool,
    streaming: bool,
) -> napi::Result<Arc<dyn api::BoxedHandler>> {
    if streaming {
        return websocket_api::new_handler(env, func);
    }
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
    pub fn new(inner: Arc<encore_runtime_core::model::Request>) -> Self {
        Self { inner }
    }

    #[napi]
    pub fn payload(&self, env: Env) -> napi::Result<JsUnknown> {
        match &self.inner.data {
            RequestData::RPC(data) => encode_request_payload(env, data.parsed_payload.as_ref()),
            RequestData::Auth(data) => encode_auth_payload(env, &data.parsed_payload),
            RequestData::PubSub(data) => pvalues_or_null(env, data.parsed_payload.as_ref()),
            RequestData::Stream(data) => encode_request_payload(env, data.parsed_payload.as_ref()),
        }
    }

    #[napi]
    pub fn meta(&self) -> napi::Result<RequestMeta> {
        request_meta::meta(&self.inner).map_err(napi::Error::from)
    }

    #[napi]
    pub fn get_auth_data(&self, env: Env) -> napi::Result<JsUnknown> {
        use RequestData::*;
        match &self.inner.data {
            RPC(data) => pvalues_or_null(env, data.auth_data.as_ref()),
            Stream(data) => pvalues_or_null(env, data.auth_data.as_ref()),
            Auth(_) | PubSub(_) => env.get_null().map(|val| val.into_unknown()),
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub struct APIPromiseHandler;

impl PromiseHandler for APIPromiseHandler {
    type Output = HandlerResponse;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output {
        let Some(val) = val else {
            return Ok(HandlerResponseInner {
                payload: None,
                extra_headers: None,
                status: None,
            });
        };

        let obj: JsObject = val
            .try_into()
            .map_err(|e| api::Error::invalid_argument("invalid handler response", e))?;

        let payload = obj
            .get_named_property::<napi::JsUnknown>("payload")
            .map_err(api::Error::internal)?;

        let extra_headers = obj
            .get_named_property::<napi::JsUnknown>("extraHeaders")
            .map_err(api::Error::internal)?;

        let status = obj
            .get_named_property::<napi::JsUnknown>("status")
            .map_err(api::Error::internal)?;

        let status = if status
            .get_type()
            .is_ok_and(|t| matches!(t, napi::ValueType::Number))
        {
            Some(
                status
                    .coerce_to_number()
                    .map_err(api::Error::internal)
                    .and_then(|s| s.get_uint32().map_err(api::Error::internal))
                    .and_then(|s| {
                        u16::try_from(s).map_err(|e| {
                            api::Error::invalid_argument("invalid http status code", e)
                        })
                    })?,
            )
        } else {
            None
        };

        match parse_pvalues(payload) {
            Ok(val) => Ok(HandlerResponseInner {
                payload: val,
                status,
                extra_headers: parse_header_map(extra_headers)
                    .map_err(|e| api::Error::invalid_argument("unable to parse extraHeaders", e))?,
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

struct TypedRequestMessage {
    req: Request,
    tx: tokio::sync::mpsc::UnboundedSender<HandlerResponse>,
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
            let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();

            // Call the handler.
            let req = Request::new(req);
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
            _ = ctx.value.tx.send(res);
            Ok(())
        }
    }
}
