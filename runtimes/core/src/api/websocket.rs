use std::sync::Arc;

use anyhow::{anyhow, Context};
use axum::extract::ws::{Message, WebSocket};
use futures::Future;
use tokio::sync::{
    mpsc::{self, UnboundedReceiver, UnboundedSender},
    watch,
};

use crate::model::{self, Request, RequestData};

use super::{schema, APIResult, HandlerResponse, HandlerResponseInner, PValues};

pub enum StreamMessagePayload {
    InOut(Socket),
    Out(Sink),
    In(Stream),
}

pub fn upgrade_request<C, Fut>(
    req: Arc<Request>,
    callback: C,
) -> APIResult<axum::response::Response>
where
    C: FnOnce(Arc<Request>, StreamMessagePayload, UnboundedSender<HandlerResponse>) -> Fut
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

    let (tx, mut rx) = mpsc::unbounded_channel::<HandlerResponse>();

    let direction = data.direction;
    Ok(upgrade
        .protocols(["encore-ws"])
        .on_failed_upgrade(|err| log::debug!("websocket upgrade failed: {err}"))
        .on_upgrade(move |ws| async move {
            let socket = Socket::new(ws, schema::Stream::new(req_schema, resp_schema).into());

            let payload = match direction {
                model::StreamDirection::InOut => StreamMessagePayload::InOut(socket),
                model::StreamDirection::In => {
                    let (sink, stream) = socket.split();

                    tokio::spawn(async move {
                        match rx.recv().await {
                            Some(resp) => match resp {
                                Ok(HandlerResponseInner {
                                    payload: Some(resp),
                                    ..
                                }) => {
                                    if sink.send(resp).is_err() {
                                        log::debug!("sink channel closed");
                                    }
                                }
                                Ok(HandlerResponseInner { payload: None, .. }) => {
                                    log::warn!("responded with empty response")
                                }
                                Err(err) => log::warn!("responded with error: {err:?}"),
                            },
                            None => log::debug!("response channel closed"),
                        };
                    });

                    StreamMessagePayload::In(stream)
                }
                model::StreamDirection::Out => {
                    let (sink, _stream) = socket.split();
                    StreamMessagePayload::Out(sink)
                }
            };

            (callback)(req, payload, tx).await
        }))
}

pub struct Socket {
    outgoing_message_tx: UnboundedSender<PValues>,
    incoming_message_rx: tokio::sync::Mutex<UnboundedReceiver<PValues>>,
    shutdown: watch::Sender<bool>,
}

impl Socket {
    fn new(mut websocket: WebSocket, schema: Arc<schema::Stream>) -> Self {
        let (shutdown, mut shutdown_watch) = watch::channel(false);

        let (outgoing_message_tx, mut outgoing_messages_rx) = mpsc::unbounded_channel();
        let (incoming_messages_tx, incoming_message_rx) = mpsc::unbounded_channel();

        tokio::spawn({
            async move {
                loop {
                    tokio::select! {
                        msg = websocket.recv() => match msg {
                            None => {
                                log::trace!("websocket closed");
                                break
                            },
                            Some(Ok(msg)) => {
                                if let Err(e) = Socket::handle_incoming_message(
                                    &schema,
                                    &incoming_messages_tx,
                                    msg,
                                )
                                .await
                                {
                                    log::warn!("failed handling incoming message: {e}");
                                    break;
                                }
                            },
                            Some(Err(e)) => {
                                log::debug!("websocket receive failed: {e}");
                                break;
                            }
                        },
                        msg = outgoing_messages_rx.recv() => {
                            match msg {
                                None => {
                                    _ = websocket.close().await;
                                    log::trace!("websocket closed");
                                    break;
                                }
                                Some(msg) => Socket::handle_outgoing_message(&schema, &mut websocket, msg).await
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

    pub fn send(&self, msg: PValues) -> anyhow::Result<()> {
        self.outgoing_message_tx.send(msg)?;
        Ok(())
    }

    pub async fn recv(&self) -> Option<PValues> {
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
        schema: &schema::Stream,
        websocket: &mut WebSocket,
        msg: PValues,
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

    async fn handle_incoming_message<M>(
        schema: &schema::Stream,
        incoming: &UnboundedSender<PValues>,
        msg: M,
    ) -> anyhow::Result<()>
    where
        M: MessagePayload,
    {
        if let Some(data) = msg.payload() {
            match schema.parse_incoming_message(data).await {
                Ok(msg) => {
                    if let Err(e) = incoming.send(msg) {
                        return Err(anyhow!("tried to send on closed channel: {e}"));
                    }
                }
                Err(e) => log::warn!("failed to parse incoming message: {e}"),
            };
        }

        Ok(())
    }
}

trait MessagePayload {
    fn payload(&self) -> Option<&[u8]>;
}

impl MessagePayload for axum::extract::ws::Message {
    fn payload(&self) -> Option<&[u8]> {
        match self {
            Message::Text(text) => Some(text.as_bytes()),
            Message::Binary(data) => Some(data),
            // these message types are handled by axum
            Message::Ping(_) | Message::Pong(_) | Message::Close(_) => None,
        }
    }
}
pub struct Sink {
    tx: UnboundedSender<PValues>,
    shutdown: watch::Sender<bool>,
}

impl Sink {
    pub fn send(&self, msg: PValues) -> anyhow::Result<()> {
        self.tx.send(msg)?;
        Ok(())
    }

    pub fn close(&self) {
        _ = self.shutdown.send(true);
    }
}

pub struct Stream {
    rx: tokio::sync::Mutex<UnboundedReceiver<PValues>>,
}
impl Stream {
    pub async fn recv(&self) -> Option<PValues> {
        self.rx.lock().await.recv().await
    }
}
