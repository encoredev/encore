use crate::api::{APIPromiseHandler, Request};
use crate::napi_util::await_promise;
use crate::napi_util::PromiseHandler;
use crate::pvalue::{parse_pvalues, PVals};
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use encore_runtime_core::api::websocket::StreamMessagePayload;
use encore_runtime_core::api::{self, HandlerRequest, HandlerResponse};
use encore_runtime_core::api::{websocket_client, ToResponse};
use napi::{Env, JsFunction, JsObject, JsUnknown, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

struct WsRequestMessage {
    req: Request,
    payload: StreamMessagePayload,
    tx: tokio::sync::mpsc::UnboundedSender<HandlerResponse>,
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
            let internal_caller = req.internal_caller.clone();
            let resp = api::websocket::upgrade_request(req, |req, payload, tx| async move {
                self.handler.call(
                    WsRequestMessage {
                        tx,
                        payload,
                        req: Request::new(req),
                    },
                    ThreadsafeFunctionCallMode::Blocking,
                );
            });

            match resp {
                Ok(resp) => api::ResponseData::Raw(resp),
                Err(e) => api::ResponseData::Raw(e.to_response(internal_caller)),
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
    #[allow(dead_code)]
    inner: api::websocket::Socket,
}

#[napi]
impl Socket {
    fn new(inner: api::websocket::Socket) -> Self {
        Socket { inner }
    }

    #[napi]
    #[allow(dead_code)]
    pub fn send(&self, msg: PVals) -> napi::Result<()> {
        self.inner
            .send(msg.0)
            .map_err(|e| napi::Error::new(napi::Status::Unknown, e))
    }

    #[napi]
    #[allow(dead_code)]
    pub async fn recv(&self) -> napi::Result<PVals> {
        self.inner
            .recv()
            .await
            .map(PVals)
            .ok_or_else(|| napi::Error::new(napi::Status::Unknown, "socket receive channel closed"))
    }

    #[napi]
    #[allow(dead_code)]
    pub fn close(&self) {
        self.inner.close()
    }
}

#[napi]
struct Sink {
    #[allow(dead_code)]
    inner: api::websocket::Sink,
}

#[napi]
impl Sink {
    fn new(inner: api::websocket::Sink) -> Self {
        Sink { inner }
    }

    #[napi]
    #[allow(dead_code)]
    pub fn send(&self, msg: PVals) -> napi::Result<()> {
        self.inner
            .send(msg.0)
            .map_err(|e| napi::Error::new(napi::Status::Unknown, e))
    }

    #[napi]
    #[allow(dead_code)]
    pub fn close(&self) {
        self.inner.close()
    }
}

#[napi]
struct Stream {
    #[allow(dead_code)]
    inner: api::websocket::Stream,
}

#[napi]
impl Stream {
    fn new(inner: api::websocket::Stream) -> Self {
        Stream { inner }
    }

    #[napi]
    #[allow(dead_code)]
    pub async fn recv(&self) -> napi::Result<PVals> {
        self.inner
            .recv()
            .await
            .ok_or_else(|| napi::Error::new(napi::Status::Unknown, "socket receive channel closed"))
            .map(PVals)
    }
}

#[napi]
pub struct WebSocketClient {
    inner: Arc<websocket_client::WebSocketClient>,
}

#[napi]
impl WebSocketClient {
    pub fn new(inner: websocket_client::WebSocketClient) -> Self {
        WebSocketClient {
            inner: Arc::new(inner),
        }
    }

    #[napi]
    #[allow(dead_code)]
    pub fn send(&self, msg: JsUnknown) -> napi::Result<()> {
        let Some(msg) = parse_pvalues(msg)? else {
            return Err(napi::Error::new(
                napi::Status::InvalidArg,
                "no message data provided",
            ));
        };

        self.inner
            .send(msg)
            .map_err(|e| napi::Error::new(napi::Status::Unknown, e))?;

        Ok(())
    }

    #[napi(ts_return_type = "Promise<Record<string, any>>")]
    #[allow(dead_code)]
    pub fn recv(&self, env: Env) -> napi::Result<JsObject> {
        let inner = self.inner.clone();
        let fut = async move {
            inner
                .recv()
                .await
                .ok_or_else(|| {
                    napi::Error::new(
                        napi::Status::Unknown,
                        "websocket client receive channel closed",
                    )
                })?
                .map_err(|e| {
                    log::warn!("unable to parse incoming message: {e}");
                    napi::Error::new(
                        napi::Status::GenericFailure,
                        "unable to parse incoming message according to schema",
                    )
                })
        };

        env.execute_tokio_future(fut, |&mut _env, res| Ok(PVals(res)))
    }

    #[napi]
    #[allow(dead_code)]
    pub fn close(&self) {
        self.inner.close()
    }
}

fn ws_resolve_on_js_thread(ctx: ThreadSafeCallContext<WsRequestMessage>) -> napi::Result<()> {
    let req = ctx
        .value
        .req
        .into_instance(ctx.env)?
        .as_object(ctx.env)
        .into_unknown();

    let stream_arg = match ctx.value.payload {
        StreamMessagePayload::InOut(socket) => Socket::new(socket)
            .into_instance(ctx.env)?
            .as_object(ctx.env)
            .into_unknown(),
        StreamMessagePayload::Out(sink) => Sink::new(sink)
            .into_instance(ctx.env)?
            .as_object(ctx.env)
            .into_unknown(),
        StreamMessagePayload::In(stream) => Stream::new(stream)
            .into_instance(ctx.env)?
            .as_object(ctx.env)
            .into_unknown(),
    };

    let handler = APIPromiseHandler;

    match ctx.callback.unwrap().call(None, &[req, stream_arg]) {
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
