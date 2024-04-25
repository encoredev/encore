use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use axum::body::Body;
use axum::http::{Response, StatusCode};
use bytes::Bytes;
use napi::bindgen_prelude::{Buffer, Either3};
use napi::{Either, Env, JsFunction, JsUnknown, NapiRaw};
use napi_derive::napi;
use tokio::sync::{mpsc, oneshot};

use encore_runtime_core::api;
use encore_runtime_core::api::IntoResponse;

use crate::api::Request;
use crate::log::parse_js_stack;
use crate::napi_util::{await_promise, PromiseHandler};
use crate::stream;
use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};

pub struct JSRawHandler {
    handler: ThreadsafeFunction<RawRequestMessage>,
}

pub fn new_handler(env: Env, func: JsFunction) -> napi::Result<Arc<dyn api::BoxedHandler>> {
    let handler = ThreadsafeFunction::create(
        env.raw(),
        // SAFETY: `handler` is a valid JS function.
        unsafe { func.raw() },
        0,
        raw_resolve_on_js_thread,
    )?;
    Ok(Arc::new(JSRawHandler { handler }))
}

struct RawRequestMessage {
    req: Request,
    resp: ResponseWriter,
    body: BodyReader,
    err_tx: mpsc::Sender<Result<(), api::Error>>,
}

enum ResponseWriterState {
    Initial {
        resp: axum::http::response::Builder,
        sender: oneshot::Sender<Response<Body>>,
    },
    StreamingBody {
        write: stream::write::WriteHalf,
    },
    Done,
}

impl ResponseWriterState {
    pub fn new(sender: oneshot::Sender<Response<Body>>) -> Self {
        let resp = axum::response::Response::builder().status(StatusCode::OK);
        Self::Initial { resp, sender }
    }

    pub fn set_head(
        self,
        status: u16,
        headers: axum::http::HeaderMap,
    ) -> Result<Self, (Self, anyhow::Error)> {
        let status = match StatusCode::from_u16(status) {
            Ok(status) => status,
            Err(err) => return Err((self, err.into())),
        };

        match self {
            Self::Initial { mut resp, sender } => {
                resp = resp.status(status);
                for (k, v) in headers {
                    if let Some(k) = k {
                        resp = resp.header(k, v);
                    }
                }
                Ok(Self::Initial { resp, sender })
            }
            _ => Ok(self),
        }
    }

    pub fn flush_header(self) -> Result<Self, (Self, anyhow::Error)> {
        match self {
            Self::Initial { resp, sender } => {
                let (write, read) = stream::write::new();
                let read = tokio_util::io::ReaderStream::new(read);

                let resp = match resp.body(Body::from_stream(read)) {
                    Ok(resp) => resp,
                    Err(err) => return Err((Self::Done, err.into())),
                };

                let _ = sender.send(resp);
                Ok(Self::StreamingBody { write })
            }
            _ => Ok(self),
        }
    }

    pub fn close(
        self,
        env: Env,
        buf: Option<Bytes>,
        callback: Option<JsFunction>,
    ) -> Result<Self, (Self, anyhow::Error)> {
        match self {
            Self::Initial { resp, sender } => {
                let body = match buf {
                    Some(buf) => Body::from(buf),
                    None => Body::empty(),
                };
                let resp = match resp.body(body) {
                    Ok(resp) => resp,
                    Err(err) => return Err((Self::Done, err.into())),
                };
                let _ = sender.send(resp);
                Ok(Self::Done)
            }
            Self::StreamingBody { mut write } => {
                if let Some(buf) = buf {
                    write.write(buf, None);
                }

                let tx = match to_sender(env, callback) {
                    Ok(tx) => tx,
                    Err(err) => return Err((Self::StreamingBody { write }, err.into())),
                };
                write.end(tx);

                Ok(Self::Done)
            }
            Self::Done => Ok(self),
        }
    }

    pub fn write_body(
        self,
        env: Env,
        buf: Bytes,
        callback: Option<JsFunction>,
    ) -> Result<Self, (Self, anyhow::Error)> {
        self.write_body_multi(env, vec![buf], callback)
    }

    pub fn write_body_multi(
        mut self,
        env: Env,
        bufs: Vec<Bytes>,
        callback: Option<JsFunction>,
    ) -> Result<Self, (Self, anyhow::Error)> {
        self = self.flush_header_if_needed();

        match self {
            Self::StreamingBody { mut write } => {
                let tx = match to_sender(env, callback) {
                    Ok(tx) => tx,
                    Err(err) => return Err((Self::StreamingBody { write }, err.into())),
                };
                write.writev(bufs, tx);
                Ok(Self::StreamingBody { write })
            }
            _ => Ok(self),
        }
    }

    fn flush_header_if_needed(self) -> Self {
        match self {
            Self::Initial { .. } => match self.flush_header() {
                Ok(state) => state,
                Err((state, _)) => state,
            },
            _ => self,
        }
    }
}

fn to_sender(env: Env, callback: Option<JsFunction>) -> napi::Result<Option<oneshot::Sender<()>>> {
    let Some(callback) = callback else {
        return Ok(None);
    };
    let (tx, rx) = oneshot::channel::<()>();

    let mut callback = env.create_reference(callback)?;
    let fut = async move {
        _ = rx.await;
        Ok(())
    };
    env.execute_tokio_future(fut, move |&mut env, _| {
        let cb: JsFunction = env.get_reference_value(&callback)?;
        callback.unref(env)?;
        cb.call_without_args(None)?;
        Ok(())
    })?;
    Ok(Some(tx))
}

#[napi]
pub struct ResponseWriter {
    // Option to support moving out of self.
    state: Option<ResponseWriterState>,
}

#[napi]
impl ResponseWriter {
    #[napi]
    pub fn write_head(
        &mut self,
        status: u16,
        headers: Either<Vec<String>, HashMap<String, Either3<String, i32, Vec<String>>>>,
    ) -> napi::Result<()> {
        let Some(state) = self.state.take() else {
            return Err(napi::Error::new(
                napi::Status::GenericFailure,
                "missing state".to_string(),
            ));
        };

        let headers = parse_headers(headers)?;

        let (state, result) = match state.set_head(status, headers) {
            Ok(state) => (state, Ok(())),
            Err((state, err)) => (state, Err(err)),
        };
        self.state = Some(state);
        result.map_err(|err| napi::Error::new(napi::Status::GenericFailure, err.to_string()))
    }

    #[napi]
    pub fn write_body(
        &mut self,
        env: Env,
        buf: Buffer,
        callback: Option<JsFunction>,
    ) -> napi::Result<()> {
        let Some(state) = self.state.take() else {
            return Err(napi::Error::new(
                napi::Status::GenericFailure,
                "missing state".to_string(),
            ));
        };

        let buf = Bytes::from(buf.to_vec());
        let (state, result) = match state.write_body(env, buf, callback) {
            Ok(state) => (state, Ok(())),
            Err((state, err)) => (state, Err(err)),
        };
        self.state = Some(state);
        result.map_err(|err| napi::Error::new(napi::Status::GenericFailure, err.to_string()))
    }

    #[napi]
    pub fn write_body_multi(
        &mut self,
        env: Env,
        bufs: Vec<Buffer>,
        callback: Option<JsFunction>,
    ) -> napi::Result<()> {
        let Some(state) = self.state.take() else {
            return Err(napi::Error::new(
                napi::Status::GenericFailure,
                "missing state".to_string(),
            ));
        };

        let bufs: Vec<_> = bufs
            .into_iter()
            .map(|buf| Bytes::from(buf.to_vec()))
            .collect();
        let (state, result) = match state.write_body_multi(env, bufs, callback) {
            Ok(state) => (state, Ok(())),
            Err((state, err)) => (state, Err(err)),
        };
        self.state = Some(state);
        result.map_err(|err| napi::Error::new(napi::Status::GenericFailure, err.to_string()))
    }

    #[napi]
    pub fn close(
        &mut self,
        env: Env,
        buf: Option<Buffer>,
        callback: Option<JsFunction>,
    ) -> napi::Result<()> {
        let Some(state) = self.state.take() else {
            return Err(napi::Error::new(
                napi::Status::GenericFailure,
                "missing state".to_string(),
            ));
        };

        let buf = buf.map(|buf| Bytes::from(buf.to_vec()));
        let (state, result) = match state.close(env, buf, callback) {
            Ok(state) => (state, Ok(())),
            Err((state, err)) => (state, Err(err)),
        };
        self.state = Some(state);
        result.map_err(|err| napi::Error::new(napi::Status::GenericFailure, err.to_string()))
    }
}

#[napi]
pub struct BodyReader {
    reader: stream::read::Reader<axum::body::BodyDataStream>,
}

#[napi]
impl BodyReader {
    pub fn new(body: axum::body::BodyDataStream) -> Self {
        Self {
            reader: stream::read::Reader::new(body),
        }
    }

    #[napi]
    pub fn start(&mut self, env: Env, push: JsFunction, destroy: JsFunction) -> napi::Result<()> {
        self.reader.start(env, push, destroy)
    }

    #[napi]
    pub fn read(&self) -> napi::Result<()> {
        self.reader.read()
    }
}

impl api::BoxedHandler for JSRawHandler {
    fn call(
        self: Arc<Self>,
        req: api::HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = api::ResponseData> + Send + 'static>> {
        Box::pin(async move {
            let (body_tx, mut body_rx) = oneshot::channel();

            let Some(body) = req.take_raw_body() else {
                let err = api::Error::internal(anyhow::anyhow!("missing body"));
                return api::ResponseData::Raw(err.into_response());
            };

            // Call the handler.
            let req = Request::new(req);
            let resp = ResponseWriter {
                state: Some(ResponseWriterState::new(body_tx)),
            };
            let body = BodyReader::new(body.into_data_stream());

            let (err_tx, mut err_rx) = mpsc::channel(1);

            self.handler.call(
                RawRequestMessage {
                    req,
                    resp,
                    body,
                    err_tx,
                },
                ThreadsafeFunctionCallMode::Blocking,
            );

            // Wait for a response body on body_rx, or an error to be received on err_rx.
            let resp = tokio::select! {
                resp = &mut body_rx => {
                    match resp {
                        Ok(resp) => resp,
                        Err(_) => {
                            let err = api::Error::internal(anyhow::anyhow!("handler did not respond"));
                            err.into_response()
                        }
                    }
                }
                err = err_rx.recv() => {
                    match err {
                        Some(Err(err)) => err.into_response(),
                        _ => {
                            // We didn't get an error. Wait for the response body instead.
                            match body_rx.await {
                                Ok(resp) => resp,
                                Err(_) => {
                                    let err = api::Error::internal(anyhow::anyhow!("handler did not respond"));
                                    err.into_response()
                                }
                            }
                        }
                    }
                }
            };

            api::ResponseData::Raw(resp)
        })
    }
}

fn parse_headers(
    headers: Either<Vec<String>, HashMap<String, Either3<String, i32, Vec<String>>>>,
) -> napi::Result<axum::http::HeaderMap> {
    fn key_err(err: axum::http::header::InvalidHeaderName) -> napi::Error {
        napi::Error::new(napi::Status::GenericFailure, err.to_string())
    }
    fn val_err(err: axum::http::header::InvalidHeaderValue) -> napi::Error {
        napi::Error::new(napi::Status::GenericFailure, err.to_string())
    }

    let mut map = axum::http::HeaderMap::new();
    match headers {
        Either::A(headers) => {
            for i in (0..headers.len()).step_by(2) {
                let key = &headers[i];
                let key: axum::http::HeaderName = headers[i].parse().map_err(key_err)?;
                let value = &headers[i + 1];
                let value: axum::http::HeaderValue = value.parse().map_err(val_err)?;
                map.append(key, value);
            }
        }

        Either::B(headers) => {
            for (key, value) in headers {
                let key: axum::http::HeaderName = key.parse().map_err(key_err)?;
                match value {
                    Either3::A(value) => {
                        let value: axum::http::HeaderValue = value.parse().map_err(val_err)?;
                        map.append(key, value);
                    }
                    Either3::B(value) => {
                        let value: axum::http::HeaderValue =
                            value.to_string().parse().map_err(val_err)?;
                        map.append(key, value);
                    }
                    Either3::C(values) => {
                        for value in values {
                            let value: axum::http::HeaderValue = value.parse().map_err(val_err)?;
                            map.append(key.clone(), value);
                        }
                    }
                }
            }
        }
    }

    Ok(map)
}

fn raw_resolve_on_js_thread(ctx: ThreadSafeCallContext<RawRequestMessage>) -> napi::Result<()> {
    let req = ctx.value.req.into_instance(ctx.env)?;
    let resp = ctx.value.resp.into_instance(ctx.env)?;
    let body = ctx.value.body.into_instance(ctx.env)?;
    let req = req.as_object(ctx.env);
    let resp = resp.as_object(ctx.env);
    let body = body.as_object(ctx.env);

    let handler = RawPromiseHandler;
    match ctx.callback.unwrap().call(None, &[req, resp, body]) {
        Ok(result) => {
            await_promise(ctx.env, result, ctx.value.err_tx.clone(), handler);
            Ok(())
        }
        Err(err) => {
            let res = handler.error(ctx.env, err);
            tokio::spawn(async move {
                _ = ctx.value.err_tx.send(res).await;
            });
            Ok(())
        }
    }
}

#[derive(Debug, Clone, Copy)]
struct RawPromiseHandler;

impl PromiseHandler for RawPromiseHandler {
    type Output = Result<(), api::Error>;

    fn resolve(&self, _env: Env, _val: Option<JsUnknown>) -> Self::Output {
        Ok(())
    }

    fn reject(&self, env: Env, val: JsUnknown) -> Self::Output {
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
