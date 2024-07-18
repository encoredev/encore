use std::sync::Arc;

use anyhow::{anyhow, Context};
use axum::extract::ws::{Message, WebSocket};
use futures::Future;
use tokio::sync::{
    mpsc::{self, UnboundedReceiver, UnboundedSender},
    watch,
};

use crate::model::{self, Request, RequestData};

use super::{schema, APIResult};

pub enum StreamMessagePayload {
    Bidi(Socket),
    Out(serde_json::Map<String, serde_json::Value>, Sink),
    In(Stream),
}

pub fn upgrade_request<C, Fut>(
    req: Arc<Request>,
    callback: C,
) -> APIResult<axum::response::Response>
where
    C: FnOnce(
            Arc<Request>,
            StreamMessagePayload,
            UnboundedSender<APIResult<schema::JSONPayload>>,
        ) -> Fut
        + Send
        + 'static,
    Fut: Future<Output = ()> + Send + 'static,
{
    let RequestData::Stream(ref data) = req.data else {
        return Err(super::Error::internal(anyhow!(
            "wrong request data type for stream"
        )));
    };

    let req_schema = data
        .endpoint
        .request
        .first()
        .context("no request schema")
        .map_err(super::Error::internal)?
        .clone();

    let resp_schema = data.endpoint.response.clone();

    let upgrade = {
        if let Some(upgrade) = data
            .websocket_upgrade
            .lock()
            .expect("mutex poisoned")
            .take()
        {
            upgrade
        } else {
            return Err(super::Error::internal(anyhow!(
                "websocket already upgraded"
            )));
        }
    };

    let (tx, mut rx) = mpsc::unbounded_channel::<APIResult<schema::JSONPayload>>();

    let direction = data.direction;
    Ok(upgrade
        .protocols(["encore-ws"])
        .on_failed_upgrade(|err| log::debug!("websocket upgrade failed: {err}"))
        .on_upgrade(move |ws| async move {
            let socket = Socket::new(ws, req_schema, resp_schema);

            let payload = match direction {
                model::StreamDirection::Bidi => StreamMessagePayload::Bidi(socket),
                model::StreamDirection::In => {
                    let (sink, stream) = socket.split();

                    tokio::spawn(async move {
                        match rx.recv().await {
                            Some(resp) => match resp {
                                Ok(Some(resp)) => {
                                    if sink.send(resp).is_err() {
                                        log::debug!("sink channel closed");
                                    }
                                }
                                Ok(None) => log::warn!("responded with empty response"),
                                Err(err) => log::warn!("responded with error: {err:?}"),
                            },
                            None => log::debug!("response channel closed"),
                        };
                    });

                    StreamMessagePayload::In(stream)
                }
                model::StreamDirection::Out => {
                    let (sink, stream) = socket.split();
                    let payload = match stream.recv().await {
                        Some(payload) => payload,
                        None => {
                            log::debug!("stream channel closed while waiting for first payload");
                            return;
                        }
                    };

                    StreamMessagePayload::Out(payload, sink)
                }
            };

            (callback)(req, payload, tx).await
        }))
}

pub struct Socket {
    outgoing_message_tx: UnboundedSender<serde_json::Map<String, serde_json::Value>>,
    incoming_message_rx:
        tokio::sync::Mutex<UnboundedReceiver<serde_json::Map<String, serde_json::Value>>>,
    shutdown: watch::Sender<bool>,
}

fn get_message_payload(msg: &Message) -> Option<&[u8]> {
    match msg {
        Message::Text(text) => Some(text.as_bytes()),
        Message::Binary(data) => Some(data),
        Message::Ping(_) | Message::Pong(_) | Message::Close(_) => None,
    }
}

impl Socket {
    fn new(
        mut websocket: WebSocket,
        incoming: Arc<schema::Request>,
        outgoing: Arc<schema::Response>,
    ) -> Self {
        let (shutdown, mut shutdown_watch) = watch::channel(false);

        let (outgoing_message_tx, mut outgoing_messages_rx) = mpsc::unbounded_channel();
        let (incoming_messages_tx, incoming_message_rx) = mpsc::unbounded_channel();

        let schema = SocketSchema { incoming, outgoing };
        tokio::spawn({
            async move {
                loop {
                    tokio::select! {
                        msg = websocket.recv() => {
                            if let Err(e) = Socket::handle_incoming_message(&schema, &incoming_messages_tx, msg).await {
                                log::debug!("websocket receive failed: {e}");
                                break;
                            }
                        },
                        msg = outgoing_messages_rx.recv() => {
                            match msg {
                                None => {
                                    _ = websocket.close().await;
                                    log::debug!("websocket closed");
                                    break;
                                },
                                Some(msg) => Socket::handle_outgoing_message(&schema, &mut websocket, msg).await,
                            }
                        },
                        _ = shutdown_watch.changed() => {
                            // gracefully shutdown, wait for all messages to be read on out channel
                            // before closing the websocket
                            outgoing_messages_rx.close();
                        }
                    }
                }

                log::trace!("socket closed");
            }
        });

        Socket {
            outgoing_message_tx,
            incoming_message_rx: tokio::sync::Mutex::new(incoming_message_rx),
            shutdown,
        }
    }

    pub fn send(&self, msg: serde_json::Map<String, serde_json::Value>) -> anyhow::Result<()> {
        self.outgoing_message_tx.send(msg)?;
        Ok(())
    }

    pub async fn recv(&self) -> Option<serde_json::Map<String, serde_json::Value>> {
        self.incoming_message_rx.lock().await.recv().await
    }

    pub fn close(&self) {
        _ = self.shutdown.send(true);
    }

    pub fn split(self) -> (Sink, Stream) {
        let Self {
            outgoing_message_tx: tx,
            incoming_message_rx: rx,
            shutdown,
        } = self;

        let sink = Sink { tx, shutdown };
        let stream = Stream { rx };

        (sink, stream)
    }

    async fn handle_outgoing_message(
        schema: &SocketSchema,
        websocket: &mut WebSocket,
        msg: serde_json::Map<String, serde_json::Value>,
    ) {
        let msg = schema
            .to_outgoing_message(msg)
            .and_then(|msg| String::from_utf8(msg).map_err(super::Error::internal));

        match msg {
            Ok(msg) => {
                if let Err(e) = websocket.send(Message::Text(msg)).await {
                    log::debug!("failed to send message to socket: {e}")
                }
            }
            Err(e) => log::warn!("failed to send message to socket: {e}"),
        }
    }

    async fn handle_incoming_message(
        schema: &SocketSchema,
        incoming: &UnboundedSender<serde_json::Map<String, serde_json::Value>>,
        msg: Option<Result<Message, axum::Error>>,
    ) -> anyhow::Result<()> {
        match msg {
            None => {
                return Err(anyhow!("websocket receive channel closed"));
            }

            Some(Ok(msg)) => {
                if let Some(data) = get_message_payload(&msg) {
                    match schema.parse_incoming_message(data).await {
                        Ok(msg) => {
                            if let Err(e) = incoming.send(msg) {
                                return Err(anyhow!("tried to send on closed channel: {e}"));
                            }
                        }
                        Err(e) => log::warn!("failed to parse incoming message: {e}"),
                    };
                } else {
                    log::info!("unhandled message type received: {msg:?}")
                }
            }
            Some(Err(e)) => {
                log::error!("received error on channel: {e}");
            }
        }

        Ok(())
    }
}
pub struct Sink {
    tx: UnboundedSender<serde_json::Map<String, serde_json::Value>>,
    shutdown: watch::Sender<bool>,
}

impl Sink {
    pub fn send(&self, msg: serde_json::Map<String, serde_json::Value>) -> anyhow::Result<()> {
        self.tx.send(msg)?;
        Ok(())
    }

    pub fn close(&self) {
        _ = self.shutdown.send(true);
    }
}

pub struct Stream {
    rx: tokio::sync::Mutex<UnboundedReceiver<serde_json::Map<String, serde_json::Value>>>,
}
impl Stream {
    pub async fn recv(&self) -> Option<serde_json::Map<String, serde_json::Value>> {
        self.rx.lock().await.recv().await
    }
}

struct SocketSchema {
    incoming: Arc<schema::Request>,
    outgoing: Arc<schema::Response>,
}

impl SocketSchema {
    fn to_outgoing_message(
        &self,
        msg: serde_json::Map<String, serde_json::Value>,
    ) -> APIResult<Vec<u8>> {
        let body_schema = self.outgoing.body.clone().ok_or_else(|| {
            super::Error::internal(anyhow!("outgoing message body can't be empty"))
        })?;

        body_schema.to_outgoing_payload(&Some(msg))
    }

    async fn parse_incoming_message(
        &self,
        bytes: &[u8],
    ) -> APIResult<serde_json::Map<String, serde_json::Value>> {
        let schema::RequestBody::Typed(Some(ref body)) = self.incoming.body else {
            return Err(super::Error {
                code: super::ErrCode::InvalidArgument,
                message: "invalid streaming body type in schema".to_string(),
                internal_message: None,
                stack: None,
            });
        };

        let value = body
            .parse_incoming_request_body(bytes.to_vec().into())
            .await?
            .ok_or_else(|| super::Error {
                code: super::ErrCode::InvalidArgument,
                message: "missing payload".to_string(),
                internal_message: None,
                stack: None,
            })?;

        Ok(value)
    }
}
