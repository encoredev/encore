use crate::api::{APIPromiseHandler, Request};
use crate::napi_util::await_promise;
use crate::napi_util::PromiseHandler;
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use axum::response::IntoResponse;
use encore_runtime_core::api::websocket::StreamMessagePayload;
use encore_runtime_core::api::{self, schema, APIResult, HandlerRequest};
use napi::{Env, JsFunction, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

struct WsRequestMessage {
    req: Request,
    payload: StreamMessagePayload,
    tx: tokio::sync::mpsc::UnboundedSender<APIResult<schema::JSONPayload>>,
}

pub struct JSWebSocketHandler {
    handler: ThreadsafeFunction<WsRequestMessage>,
}

impl api::BoxedHandler for JSWebSocketHandler {
    fn call(
        self: Arc<Self>,
        req: HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = api::ResponseData> + Send + 'static>> {
        Box::pin(async move {
            let resp = api::websocket::upgrade_request(req, |req, payload, tx| async move {
                let status = self.handler.call(
                    WsRequestMessage {
                        tx,
                        payload,
                        req: Request::new(req),
                    },
                    ThreadsafeFunctionCallMode::Blocking,
                );

                log::debug!("js ws handler responded with status: {status}");
            });

            match resp {
                Ok(resp) => api::ResponseData::Raw(resp),
                Err(e) => api::ResponseData::Raw(e.into_response()),
            }
        })
    }
}

pub fn new_handler(env: Env, func: JsFunction) -> napi::Result<Arc<dyn api::BoxedHandler>> {
    let handler = ThreadsafeFunction::create(
        env.raw(),
        // SAFETY: `handler` is a valid JS function.
        unsafe { func.raw() },
        0,
        ws_resolve_on_js_thread,
    )?;

    Ok(Arc::new(JSWebSocketHandler { handler }))
}

#[napi]
struct Socket {
    inner: api::websocket::Socket,
}

#[napi]
impl Socket {
    fn new(inner: api::websocket::Socket) -> Self {
        Socket { inner }
    }

    #[napi]
    pub fn send(&self, msg: serde_json::Map<String, serde_json::Value>) -> napi::Result<()> {
        self.inner
            .send(msg)
            .map_err(|e| napi::Error::new(napi::Status::Unknown, e))
    }

    #[napi]
    pub async fn recv(&self) -> napi::Result<serde_json::Map<String, serde_json::Value>> {
        self.inner
            .recv()
            .await
            .ok_or_else(|| napi::Error::new(napi::Status::Unknown, "socket receive channel closed"))
    }

    #[napi]
    pub fn close(&self) {
        self.inner.close()
    }
}

#[napi]
struct Sink {
    inner: api::websocket::Sink,
}

#[napi]
impl Sink {
    fn new(inner: api::websocket::Sink) -> Self {
        Sink { inner }
    }

    #[napi]
    pub fn send(&self, msg: serde_json::Map<String, serde_json::Value>) -> napi::Result<()> {
        self.inner
            .send(msg)
            .map_err(|e| napi::Error::new(napi::Status::Unknown, e))
    }

    #[napi]
    pub fn close(&self) {
        self.inner.close()
    }
}

#[napi]
struct Stream {
    inner: api::websocket::Stream,
}

#[napi]
impl Stream {
    fn new(inner: api::websocket::Stream) -> Self {
        Stream { inner }
    }

    #[napi]
    pub async fn recv(&self) -> napi::Result<serde_json::Map<String, serde_json::Value>> {
        self.inner
            .recv()
            .await
            .ok_or_else(|| napi::Error::new(napi::Status::Unknown, "socket receive channel closed"))
    }
}

fn ws_resolve_on_js_thread(ctx: ThreadSafeCallContext<WsRequestMessage>) -> napi::Result<()> {
    let req = ctx
        .value
        .req
        .into_instance(ctx.env)?
        .as_object(ctx.env)
        .into_unknown();

    let args = match ctx.value.payload {
        StreamMessagePayload::Bidi(socket) => {
            vec![
                req,
                Socket::new(socket)
                    .into_instance(ctx.env)?
                    .as_object(ctx.env)
                    .into_unknown(),
            ]
        }
        StreamMessagePayload::Out(request, sink) => {
            vec![
                req,
                ctx.env.to_js_value(&request)?,
                Sink::new(sink)
                    .into_instance(ctx.env)?
                    .as_object(ctx.env)
                    .into_unknown(),
            ]
        }
        StreamMessagePayload::In(stream) => {
            vec![
                req,
                Stream::new(stream)
                    .into_instance(ctx.env)?
                    .as_object(ctx.env)
                    .into_unknown(),
            ]
        }
    };

    let handler = APIPromiseHandler;

    match ctx.callback.unwrap().call(None, &args) {
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
