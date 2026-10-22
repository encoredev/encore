use std::sync::Arc;

use anyhow::{anyhow, Context};
use axum::extract::ws::{Message, WebSocket, WebSocketUpgrade};
use futures::Future;
use tokio::sync::{
    mpsc::{self, UnboundedReceiver, UnboundedSender},
    oneshot, watch, Notify,
};

use crate::model::{self, Request, RequestData};

use super::{schema, APIResult, HandlerResponse, HandlerResponseInner, PValues};

pub enum StreamMessagePayload {
    InOut(Socket),
    Out(Sink),
    In(Stream),
}

/// Upgrades a streaming request to a WebSocket and hands the stream to `callback`.
///
/// The handler is invoked *before* the WebSocket handshake is completed, with a
/// stream whose I/O is buffered until the connection has been upgraded. The
/// `101 Switching Protocols` response is only sent once the handler first uses
/// the stream (sending, receiving or closing), or once it returns successfully.
///
/// If the handler (or middleware wrapping it) returns an error before touching
/// the stream, the upgrade is rejected and the error is returned, so the caller
/// can respond with a regular HTTP error (e.g. `403 Permission Denied`).
pub async fn upgrade_request<C, Fut>(
    req: Arc<Request>,
    callback: C,
) -> APIResult<axum::response::Response>
where
    C: FnOnce(Arc<Request>, StreamMessagePayload, oneshot::Sender<HandlerResponse>) -> Fut
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

    let direction = data.direction;
    let schema = schema::Stream::new(req_schema, resp_schema).into();
    upgrade_with(req, upgrade, schema, direction, callback).await
}

async fn upgrade_with<T, C, Fut>(
    ctx: T,
    upgrade: WebSocketUpgrade,
    schema: Arc<schema::Stream>,
    direction: model::StreamDirection,
    callback: C,
) -> APIResult<axum::response::Response>
where
    T: Send + 'static,
    C: FnOnce(T, StreamMessagePayload, oneshot::Sender<HandlerResponse>) -> Fut + Send + 'static,
    Fut: Future<Output = ()> + Send + 'static,
{
    let (socket, io) = Socket::pending(schema);
    let touched = socket.touched.clone();

    // For client streams the handler's return value is sent as the final message,
    // so we keep the sink here instead of handing it to the handler.
    let (payload, response_sink) = match direction {
        model::StreamDirection::InOut => (StreamMessagePayload::InOut(socket), None),
        model::StreamDirection::In => {
            let (sink, stream) = socket.split();
            (StreamMessagePayload::In(stream), Some(sink))
        }
        model::StreamDirection::Out => {
            let (sink, _stream) = socket.split();
            (StreamMessagePayload::Out(sink), None)
        }
    };

    let (tx, mut rx) = oneshot::channel::<HandlerResponse>();
    tokio::spawn((callback)(ctx, payload, tx));

    // Wait until the handler either uses the stream or completes.
    // If both have happened, the stream was used first and we must upgrade.
    let early_response = tokio::select! {
        biased;
        _ = touched.notified() => None,
        resp = &mut rx => Some(resp),
    };

    match early_response {
        // The handler (or a middleware) rejected the request before using the
        // stream: respond with a regular HTTP error instead of upgrading.
        Some(Ok(Err(err))) => return Err(err),
        Some(resp) => {
            if let Some(sink) = response_sink {
                send_handler_response(sink, resp);
            }
        }
        None => {
            if let Some(sink) = response_sink {
                tokio::spawn(async move { send_handler_response(sink, rx.await) });
            }
        }
    }

    Ok(upgrade
        .protocols(["encore-ws"])
        .on_failed_upgrade(|err| log::debug!("websocket upgrade failed: {err}"))
        .on_upgrade(move |ws| io.run(ws)))
}

fn send_handler_response(sink: Sink, resp: Result<HandlerResponse, oneshot::error::RecvError>) {
    match resp {
        Ok(Ok(HandlerResponseInner {
            payload: Some(resp),
            ..
        })) => {
            if sink.send(resp).is_err() {
                log::debug!("sink channel closed");
            }
        }
        Ok(Ok(HandlerResponseInner { payload: None, .. })) => {
            log::warn!("responded with empty response")
        }
        Ok(Err(err)) => log::warn!("responded with error: {err:?}"),
        Err(_) => log::debug!("response channel closed"),
    }
}

pub struct Socket {
    outgoing_message_tx: UnboundedSender<PValues>,
    incoming_message_rx: tokio::sync::Mutex<UnboundedReceiver<PValues>>,
    shutdown: watch::Sender<bool>,
    touched: Arc<Notify>,
}

/// The connection side of a [`Socket`] whose WebSocket has not been upgraded yet.
/// Messages sent on the socket are buffered until [`PendingIo::run`] is called.
struct PendingIo {
    schema: Arc<schema::Stream>,
    outgoing_messages_rx: UnboundedReceiver<PValues>,
    incoming_messages_tx: UnboundedSender<PValues>,
    shutdown_watch: watch::Receiver<bool>,
}

impl PendingIo {
    async fn run(self, mut websocket: WebSocket) {
        let PendingIo {
            schema,
            mut outgoing_messages_rx,
            incoming_messages_tx,
            mut shutdown_watch,
        } = self;

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
}

impl Socket {
    /// Creates a socket that is not yet connected to a WebSocket.
    fn pending(schema: Arc<schema::Stream>) -> (Self, PendingIo) {
        let (shutdown, shutdown_watch) = watch::channel(false);
        let (outgoing_message_tx, outgoing_messages_rx) = mpsc::unbounded_channel();
        let (incoming_messages_tx, incoming_message_rx) = mpsc::unbounded_channel();

        let socket = Socket {
            outgoing_message_tx,
            incoming_message_rx: tokio::sync::Mutex::new(incoming_message_rx),
            shutdown,
            touched: Arc::new(Notify::new()),
        };
        let io = PendingIo {
            schema,
            outgoing_messages_rx,
            incoming_messages_tx,
            shutdown_watch,
        };
        (socket, io)
    }

    pub fn send(&self, msg: PValues) -> anyhow::Result<()> {
        self.touched.notify_one();
        self.outgoing_message_tx.send(msg)?;
        Ok(())
    }

    pub async fn recv(&self) -> Option<PValues> {
        self.touched.notify_one();
        self.incoming_message_rx.lock().await.recv().await
    }

    pub fn close(&self) {
        self.touched.notify_one();
        _ = self.shutdown.send(true);
    }

    pub fn split(self) -> (Sink, Stream) {
        let Self {
            outgoing_message_tx: tx,
            incoming_message_rx: rx,
            shutdown,
            touched,
        } = self;

        let sink = Sink {
            tx,
            shutdown,
            touched: touched.clone(),
        };
        let stream = Stream { rx, touched };

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
    touched: Arc<Notify>,
}

impl Sink {
    pub fn send(&self, msg: PValues) -> anyhow::Result<()> {
        self.touched.notify_one();
        self.tx.send(msg)?;
        Ok(())
    }

    pub fn close(&self) {
        self.touched.notify_one();
        _ = self.shutdown.send(true);
    }
}

pub struct Stream {
    rx: tokio::sync::Mutex<UnboundedReceiver<PValues>>,
    touched: Arc<Notify>,
}
impl Stream {
    pub async fn recv(&self) -> Option<PValues> {
        self.touched.notify_one();
        self.rx.lock().await.recv().await
    }
}

#[cfg(test)]
mod tests {
    use std::time::Duration;

    use axum::{routing::get, Router};
    use futures_util::StreamExt;
    use tokio_tungstenite::{connect_async, tungstenite};

    use super::*;
    use crate::api::{ErrCode, Error, ToResponse};

    struct NoBody;

    impl schema::StreamMessage for NoBody {
        fn body(&self) -> Option<&schema::Body> {
            None
        }
    }

    fn permission_denied() -> Error {
        Error {
            code: ErrCode::PermissionDenied,
            message: "not allowed".into(),
            internal_message: None,
            stack: None,
            details: None,
        }
    }

    /// Serves a single stream endpoint that runs `handler` and returns its ws:// URL.
    async fn serve<H, Fut>(direction: model::StreamDirection, handler: H) -> String
    where
        H: Fn(StreamMessagePayload, oneshot::Sender<HandlerResponse>) -> Fut
            + Clone
            + Send
            + Sync
            + 'static,
        Fut: Future<Output = ()> + Send + 'static,
    {
        let app = Router::new().route(
            "/",
            get(move |upgrade: WebSocketUpgrade| {
                let handler = handler.clone();
                async move {
                    let schema = Arc::new(schema::Stream::new(NoBody, NoBody));
                    let resp = upgrade_with((), upgrade, schema, direction, move |(), p, tx| {
                        handler(p, tx)
                    })
                    .await;
                    match resp {
                        Ok(resp) => resp,
                        Err(err) => err.to_response(None),
                    }
                }
            }),
        );

        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
        format!("ws://{addr}/")
    }

    async fn expect_closed<S>(ws: &mut S)
    where
        S: futures_util::Stream<Item = Result<tungstenite::Message, tungstenite::Error>> + Unpin,
    {
        let next = tokio::time::timeout(Duration::from_secs(5), ws.next())
            .await
            .expect("timed out waiting for close");
        match next {
            None | Some(Ok(tungstenite::Message::Close(_))) | Some(Err(_)) => {}
            Some(Ok(msg)) => panic!("expected close, got {msg:?}"),
        }
    }

    #[tokio::test]
    async fn error_before_stream_use_rejects_handshake() {
        let url = serve(model::StreamDirection::Out, |payload, tx| async move {
            // Simulate an async authorization check that fails before the
            // handler touches the stream.
            tokio::time::sleep(Duration::from_millis(20)).await;
            _ = tx.send(Err(permission_denied()));
            drop(payload);
        })
        .await;

        match connect_async(url).await {
            Err(tungstenite::Error::Http(resp)) => {
                assert_eq!(resp.status(), 403);
                let body = String::from_utf8(resp.body().clone().unwrap_or_default()).unwrap();
                assert!(body.contains("permission_denied"), "body: {body}");
            }
            Ok((_, resp)) => panic!("expected handshake rejection, got {}", resp.status()),
            Err(e) => panic!("unexpected error: {e}"),
        }
    }

    #[tokio::test]
    async fn stream_use_upgrades() {
        let url = serve(model::StreamDirection::Out, |payload, tx| async move {
            let StreamMessagePayload::Out(sink) = payload else {
                panic!("expected sink");
            };
            tokio::time::sleep(Duration::from_millis(20)).await;
            sink.close();
            _ = tx.send(Ok(HandlerResponseInner {
                payload: None,
                extra_headers: None,
                status: None,
            }));
        })
        .await;

        let (mut ws, resp) = connect_async(url).await.expect("handshake should succeed");
        assert_eq!(resp.status(), 101);
        expect_closed(&mut ws).await;
    }

    #[tokio::test]
    async fn error_after_stream_use_keeps_upgrade() {
        let url = serve(model::StreamDirection::Out, |payload, tx| async move {
            let StreamMessagePayload::Out(sink) = payload else {
                panic!("expected sink");
            };
            sink.close();
            _ = tx.send(Err(permission_denied()));
        })
        .await;

        let (mut ws, resp) = connect_async(url).await.expect("handshake should succeed");
        assert_eq!(resp.status(), 101);
        expect_closed(&mut ws).await;
    }

    #[tokio::test]
    async fn success_without_stream_use_upgrades() {
        let url = serve(model::StreamDirection::Out, |payload, tx| async move {
            drop(payload);
            _ = tx.send(Ok(HandlerResponseInner {
                payload: None,
                extra_headers: None,
                status: None,
            }));
        })
        .await;

        let (mut ws, resp) = connect_async(url).await.expect("handshake should succeed");
        assert_eq!(resp.status(), 101);
        expect_closed(&mut ws).await;
    }

    #[tokio::test]
    async fn receiving_upgrades_before_handler_completes() {
        let (done_tx, done_rx) = oneshot::channel::<()>();
        let done_tx = Arc::new(std::sync::Mutex::new(Some(done_tx)));

        let url = serve(model::StreamDirection::InOut, move |payload, tx| {
            let done_tx = done_tx.clone();
            async move {
                let StreamMessagePayload::InOut(socket) = payload else {
                    panic!("expected socket");
                };
                // Blocks until the client closes the connection, which can only
                // happen if the handshake completes while we wait here.
                while socket.recv().await.is_some() {}
                _ = tx.send(Ok(HandlerResponseInner {
                    payload: None,
                    extra_headers: None,
                    status: None,
                }));
                if let Some(done) = done_tx.lock().unwrap().take() {
                    _ = done.send(());
                }
            }
        })
        .await;

        let (mut ws, resp) = connect_async(url).await.expect("handshake should succeed");
        assert_eq!(resp.status(), 101);
        ws.close(None).await.unwrap();
        tokio::time::timeout(Duration::from_secs(5), done_rx)
            .await
            .expect("handler should finish after the client closes")
            .unwrap();
    }
}
