use std::sync::Arc;

use futures::sink::SinkExt;
use futures::stream::SplitSink;
use futures::stream::SplitStream;
use futures::stream::StreamExt;
use tokio::net::TcpStream;
use tokio::sync::mpsc::UnboundedReceiver;
use tokio::sync::mpsc::UnboundedSender;
use tokio::sync::watch;
use tokio::sync::Mutex;
use tokio_tungstenite::MaybeTlsStream;
use tokio_tungstenite::{tungstenite::Message, WebSocketStream};

use super::schema;
use super::APIResult;
use super::PValues;

pub struct WebSocketClient {
    send_channel: UnboundedSender<Message>,
    receive_channel: Mutex<UnboundedReceiver<Message>>,
    shutdown: watch::Sender<bool>,
    schema: Arc<schema::Stream>,
}

impl WebSocketClient {
    pub async fn connect(
        request: http::Request<()>,
        schema: schema::Stream,
    ) -> APIResult<WebSocketClient> {
        let (connection, _resp) = tokio_tungstenite::connect_async(request)
            .await
            .map_err(|e| super::Error {
                code: super::ErrCode::Unknown,
                message: "failed connecting to websocket endpoint".to_string(),
                internal_message: Some(e.to_string()),
                stack: None,
                details: None,
            })?;

        let (ws_write, ws_read) = connection.split();

        let (send_channel_tx, send_channel_rx) = tokio::sync::mpsc::unbounded_channel();
        let (receive_channel_tx, receive_channel_rx) = tokio::sync::mpsc::unbounded_channel();

        let (shutdown, shutdown_watch) = watch::channel(false);

        tokio::spawn(send_to_ws(send_channel_rx, ws_write, shutdown_watch));
        tokio::spawn(ws_to_receive(ws_read, receive_channel_tx));

        Ok(WebSocketClient {
            send_channel: send_channel_tx,
            receive_channel: Mutex::new(receive_channel_rx),
            shutdown,
            schema: schema.into(),
        })
    }

    pub fn send(&self, msg: PValues) -> APIResult<()> {
        let msg = self.schema.to_outgoing_message(msg)?;
        let msg = String::from_utf8(msg).map_err(super::Error::internal)?;

        self.send_channel
            .send(Message::Text(msg))
            .map_err(super::Error::internal)?;

        Ok(())
    }

    pub async fn recv(&self) -> Option<APIResult<PValues>> {
        loop {
            let msg = self.receive_channel.lock().await.recv().await;

            let bytes: bytes::Bytes = match msg {
                Some(Message::Text(msg)) => msg.into(),
                Some(Message::Binary(vec)) => vec.into(),
                Some(_msg) => continue,
                None => return None,
            };

            return Some(self.schema.parse_incoming_message(&bytes).await);
        }
    }

    pub fn close(&self) {
        if let Err(err) = self.shutdown.send(true) {
            log::trace!("error sending shutdown signal: {err}");
        }
    }
}

async fn send_to_ws(
    mut rx: UnboundedReceiver<Message>,
    mut ws: SplitSink<WebSocketStream<MaybeTlsStream<TcpStream>>, Message>,
    mut shutdown: watch::Receiver<bool>,
) {
    loop {
        tokio::select! {
            _ = shutdown.changed() => {
                rx.close()
            },
            msg = rx.recv() => match msg {
                Some(msg) => {
                    if let Err(err) = ws.send(msg).await {
                        log::debug!("failed sending over websocket: {err}");
                    }
                },
                None => {
                    log::trace!("receive channel closed, shutting down");

                    if let Err(err) = ws.close().await {
                        log::trace!("closing websocket failed: {err}");
                    }

                    break;
                },

            },
        }
    }
}

async fn ws_to_receive(
    mut ws: SplitStream<WebSocketStream<MaybeTlsStream<TcpStream>>>,
    tx: UnboundedSender<Message>,
) {
    loop {
        let msg = ws.next().await;

        match msg {
            Some(Ok(msg)) => {
                if let Err(err) = tx.send(msg) {
                    log::warn!("failed sending to receive channel: {err}");
                    break;
                }
            }
            Some(Err(err)) => {
                log::debug!("received an error from websocket: {err}");
                break;
            }
            None => {
                log::trace!("websocket closed, shutting down");
                break;
            }
        }
    }
}
