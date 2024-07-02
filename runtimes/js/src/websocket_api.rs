use crate::api::{APIPromiseHandler, Request};
use crate::napi_util::await_promise;
use crate::napi_util::PromiseHandler;
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};
use anyhow::anyhow;
use axum::extract::ws::{Message, WebSocket};
use axum::extract::{FromRequestParts, WebSocketUpgrade};
use encore_runtime_core::api::{self, schema, HandlerRequest};
use napi::{Env, JsFunction, NapiRaw};
use napi_derive::napi;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, Mutex};
use tokio::sync::mpsc::{UnboundedReceiver, UnboundedSender};

struct WsRequestMessage {
    req: Request,
    socket: Socket,
    tx: tokio::sync::mpsc::UnboundedSender<Result<schema::JSONPayload, api::Error>>,
}

pub struct JSWebSocketHandler {
    handler: ThreadsafeFunction<WsRequestMessage>,
    websocket_upgrade: Mutex<Option<WebSocketUpgrade>>,
}

impl JSWebSocketHandler {
    fn take_websocket_upgrade(&self) -> Option<WebSocketUpgrade> {
        self.websocket_upgrade.lock().unwrap().take()
    }
}

#[async_trait::async_trait]
impl api::BoxedHandler for JSWebSocketHandler {
    async fn extract_from_parts(
        &self,
        parts: &mut axum::http::request::Parts,
    ) -> api::APIResult<()> {
        let state = &();
        let upgrade = WebSocketUpgrade::from_request_parts(parts, state).await?;
        self.websocket_upgrade.lock().unwrap().replace(upgrade);
        Ok(())
    }
    fn call(
        self: Arc<Self>,
        req: HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = api::ResponseData> + Send + 'static>> {
        Box::pin(async move {
            // Create a one-shot channel
            let (tx, _rx) = tokio::sync::mpsc::unbounded_channel();

            if let Some(socket) = self.take_websocket_upgrade() {
                let resp = socket.on_upgrade(|ws: WebSocket| async move {
                    let socket = Socket::new(ws);
                    let req = Request::new(req);

                    // Call the handler.
                    let status = self.handler.call(
                        WsRequestMessage { tx, socket, req },
                        ThreadsafeFunctionCallMode::Blocking,
                    );

                    log::debug!("js ws handler responded with status: {status}");
                });

                api::ResponseData::Raw(resp)
            } else {
                api::ResponseData::Typed(Err(api::Error::internal(anyhow!(
                    "faild to extract websocket upgrade"
                ))))
            }
        })
    }
}

#[napi]
pub struct Socket {
    out_tx: UnboundedSender<Message>,
    in_rx: tokio::sync::Mutex<UnboundedReceiver<Message>>,
    notify_close: Arc<tokio::sync::Notify>,
}

#[napi]
impl Socket {
    fn new(mut websocket: WebSocket) -> Self {
        let (out_tx, mut out_rx) = tokio::sync::mpsc::unbounded_channel();
        let (in_tx, in_rx) = tokio::sync::mpsc::unbounded_channel();
        let notify_close = Arc::new(tokio::sync::Notify::new());

        let _handle = tokio::spawn({
            let notify_close = notify_close.clone();
            async move {
                loop {
                    tokio::select! {
                        msg = websocket.recv() => {
                            let msg = msg.unwrap().unwrap();
                            in_tx.send(msg).unwrap();
                        }
                        msg = out_rx.recv() => {
                            match msg {
                                Some(msg) => websocket.send(msg).await.unwrap(),
                                None => todo!(),
                            }
                        }
                        _ = notify_close.notified() => {
                            websocket.close().await.unwrap();
                            break;
                        }
                    }
                }

                log::trace!("socket closed");
            }
        });

        Socket {
            out_tx,
            in_rx: tokio::sync::Mutex::new(in_rx),
            notify_close,
        }
    }

    #[napi]
    pub async fn send(&self, msg: String) -> napi::Result<()> {
        self.out_tx
            .send(msg.into())
            .map_err(|_e| napi::Error::new(napi::Status::Unknown, "send channel close"))?;
        Ok(())
    }

    #[napi]
    pub async fn recv(&self) -> napi::Result<String> {
        Ok(self
            .in_rx
            .lock()
            .await
            .recv()
            .await
            .ok_or(napi::Error::new(
                napi::Status::Unknown,
                "receive channel closed",
            ))?
            .to_text()
            .map_err(|_e| napi::Error::new(napi::Status::Unknown, "not valid utf-8"))?
            .to_string())
    }

    #[napi]
    pub fn close(&self) {
        self.notify_close.notify_one()
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

    Ok(Arc::new(JSWebSocketHandler {
        handler,
        websocket_upgrade: Mutex::new(None),
    }))
}

fn ws_resolve_on_js_thread(ctx: ThreadSafeCallContext<WsRequestMessage>) -> napi::Result<()> {
    let socket = ctx.value.socket.into_instance(ctx.env)?.as_object(ctx.env);
    let req = ctx.value.req.into_instance(ctx.env)?.as_object(ctx.env);
    let handler = APIPromiseHandler;

    match ctx.callback.unwrap().call(None, &[req, socket]) {
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
