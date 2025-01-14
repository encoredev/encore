use std::future::Future;

use bytes::{Bytes, BytesMut};
use futures::{Stream, StreamExt};
use napi::{noop_finalize, Env, JsFunction, JsUnknown, NapiRaw, Status};

use crate::threadsafe_function::{
    ThreadSafeCallContext, ThreadsafeFunction, ThreadsafeFunctionCallMode,
};

pub struct Reader<S> {
    state: ReaderState<S>,
}

impl<S, E> Reader<S>
where
    S: Stream<Item = Result<Bytes, E>> + Unpin + Send + 'static,
    E: std::error::Error + Send + 'static,
{
    pub fn new(stream: S) -> Self {
        Self {
            state: ReaderState::Initial(stream),
        }
    }

    pub fn start(&mut self, env: Env, push: JsFunction, destroy: JsFunction) -> napi::Result<()> {
        let (tx, rx) = tokio::sync::mpsc::unbounded_channel();
        let state = std::mem::replace(&mut self.state, ReaderState::Running(tx));
        let stream = match state {
            ReaderState::Initial(stream) => stream,
            _ => {
                return Err(napi::Error::new(
                    Status::GenericFailure,
                    "reader has already been started".to_owned(),
                ))
            }
        };

        let push = ThreadsafeFunction::create(
            env.raw(),
            // SAFETY: `push` is a valid JS function.
            unsafe { push.raw() },
            0,
            execute_push,
        )?;
        let destroy = ThreadsafeFunction::create(
            env.raw(),
            // SAFETY: `destroy` is a valid JS function.
            unsafe { destroy.raw() },
            0,
            execute_destroy,
        )?;

        let machine = StateMachine {
            stream,
            read_requests: rx,
            push,
            destroy,
            did_destroy: false,
        };
        tokio::spawn(machine.run());
        Ok(())
    }

    pub fn read(&self) -> napi::Result<()> {
        match &self.state {
            ReaderState::Running(tx) => {
                let _ = tx.send(());
                Ok(())
            }
            ReaderState::Initial(_) => Err(napi::Error::new(
                Status::GenericFailure,
                "reader has not been started".to_owned(),
            )),
        }
    }
}

enum ReaderState<S> {
    Initial(S),
    Running(tokio::sync::mpsc::UnboundedSender<()>),
}

struct StateMachine<S> {
    stream: S,
    read_requests: tokio::sync::mpsc::UnboundedReceiver<()>,
    push: ThreadsafeFunction<PushRequest>,
    destroy: ThreadsafeFunction<DestroyRequest>,
    did_destroy: bool,
}

impl<S, E> StateMachine<S>
where
    S: Stream<Item = Result<Bytes, E>> + Unpin + Send,
    E: std::error::Error + Send + 'static,
{
    async fn run(mut self) {
        'ReadRequestLoop: loop {
            // Wait for a read request.
            let Some(()) = self.read_requests.recv().await else {
                // The sender was dropped.
                self.notify_close();
                return;
            };

            // Read repeatedly until push() returns false.
            'PushLoop: loop {
                match self.stream.next().await.transpose() {
                    Ok(data) => {
                        let is_none = data.is_none();
                        match self.push(data).await {
                            // The stream successfully ended.
                            Ok(_) if is_none => {
                                // Note: don't notify_close; the node:stream API will handle
                                // automatically closing the stream when the data has actually been read.
                                // If we close here we end up destroying the stream too early, preventing
                                // the last data from being read.
                                return;
                            }

                            // We haven't reached the high water mark yet; continue pushing data.
                            Ok(true) => continue 'PushLoop,

                            // We've reached the high water mark; wait for the next read request.
                            Ok(false) => continue 'ReadRequestLoop,

                            // Something went wrong communicating with node. Close the stream with an error.
                            Err(err) => {
                                self.notify_err(err);
                                return;
                            }
                        }
                    }
                    Err(err) => {
                        self.notify_err(err);
                        return;
                    }
                }
            }
        }
    }

    fn notify_err<Err: std::error::Error + 'static>(&mut self, err: Err) {
        if self.did_destroy {
            return;
        }
        self.did_destroy = true;
        let req = DestroyRequest {
            err: Some(Box::new(err)),
        };
        self.destroy.call(req, ThreadsafeFunctionCallMode::Blocking);
    }

    fn notify_close(&mut self) {
        if self.did_destroy {
            return;
        }
        self.did_destroy = true;
        let req = DestroyRequest { err: None };
        self.destroy.call(req, ThreadsafeFunctionCallMode::Blocking);
    }

    fn push(&self, data: Option<Bytes>) -> impl Future<Output = napi::Result<bool>> {
        let (tx, rx) = tokio::sync::oneshot::channel();
        let req = PushRequest { data, response: tx };

        let result = self.push.call(req, ThreadsafeFunctionCallMode::Blocking);

        async move {
            match result {
                Status::Ok => match rx.await {
                    Ok(more) => Ok(more),
                    Err(_) => Err(napi::Error::new(
                        Status::GenericFailure,
                        "push response channel was dropped".to_owned(),
                    )),
                },

                status => Err(napi::Error::new(
                    status,
                    "failed to call push function".to_owned(),
                )),
            }
        }
    }
}

struct PushRequest {
    data: Option<Bytes>,
    response: tokio::sync::oneshot::Sender<bool>,
}

fn execute_push(ctx: ThreadSafeCallContext<PushRequest>) -> napi::Result<()> {
    let data: JsUnknown = match ctx.value.data {
        Some(data) => {
            // We need to convert this to a BytesMut since
            // JavaScript may modify the buffer.
            let mut data = BytesMut::from(data);
            let buf = unsafe {
                ctx.env.create_buffer_with_borrowed_data(
                    data.as_mut_ptr(),
                    data.len(),
                    data,
                    noop_finalize,
                )?
            };
            buf.into_unknown()
        }
        None => ctx.env.get_null()?.into_unknown(),
    };

    let more = ctx
        .callback
        .unwrap()
        .call(None, &[data])?
        .coerce_to_bool()?
        .get_value()?;
    _ = ctx.value.response.send(more);
    Ok(())
}

struct DestroyRequest {
    err: Option<Box<dyn std::error::Error>>,
}

fn execute_destroy(ctx: ThreadSafeCallContext<DestroyRequest>) -> napi::Result<()> {
    if let Some(err) = ctx.value.err {
        let err = ctx
            .env
            .create_error(napi::Error::new(Status::GenericFailure, err.to_string()))?;
        ctx.callback.unwrap().call(None, &[err.into_unknown()])?;
    } else {
        ctx.callback.unwrap().call_without_args(None)?;
    }
    Ok(())
}
