use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::Context;
use napi::{Env, Error, JsFunction, JsObject, JsUnknown, NapiRaw, Status};
use napi_derive::napi;

use encore_runtime_core::pubsub::{SubscriptionObj, TopicObj};
use encore_runtime_core::{api, model, pubsub};

use crate::api::Request;
use crate::error::coerce_to_api_error;
use crate::napi_util::{await_promise, PromiseHandler};
use crate::pvalue::parse_pvalues;
use crate::threadsafe_function::{ThreadSafeCallContext, ThreadsafeFunction};

#[napi]
pub struct PubSubTopic {
    topic: TopicObj,
}

#[napi]
impl PubSubTopic {
    pub(crate) fn new(topic: TopicObj) -> Self {
        Self { topic }
    }

    #[napi(ts_return_type = "Promise<string>")]
    pub fn publish(
        &self,
        env: Env,
        body: JsUnknown,
        source: Option<&Request>,
    ) -> napi::Result<JsObject> {
        let Some(payload) = parse_pvalues(body).context("failed to parse payload")? else {
            return Err(Error::new(
                Status::InvalidArg,
                "no message payload provided",
            ));
        };

        let source = source.map(|s| s.inner.clone());
        let fut = self.topic.publish(payload, source);
        let fut = async move {
            match fut.await {
                Ok(id) => Ok(id),
                Err(e) => Err(Error::new(
                    Status::GenericFailure,
                    format!("failed to publish: {}", e),
                )),
            }
        };

        env.spawn_future(fut)
    }
}

#[napi(object)]
pub struct PubSubSubscriptionConfig {
    pub topic_name: String,
    pub subscription_name: String,
    pub handler: JsFunction,
}

impl PubSubSubscriptionConfig {
    pub fn to_handler(&self, env: Env) -> napi::Result<JSSubscriptionHandler> {
        let tsfn = ThreadsafeFunction::create(
            env.raw(),
            // SAFETY: `handler` is a valid JS function.
            unsafe { self.handler.raw() },
            0,
            resolve_on_js_thread,
        )?;

        Ok(JSSubscriptionHandler {
            handler: Arc::new(tsfn),
        })
    }
}

#[napi]
pub struct PubSubSubscription {
    sub: Arc<SubscriptionObj>,
    handler: Arc<JSSubscriptionHandler>,
}

impl PubSubSubscription {
    pub fn new(sub: Arc<SubscriptionObj>, handler: Arc<JSSubscriptionHandler>) -> Self {
        Self { sub, handler }
    }
}

#[napi]
impl PubSubSubscription {
    #[napi]
    pub async fn subscribe(&self) -> napi::Result<()> {
        self.sub.subscribe(self.handler.clone()).await.map_err(|e| {
            Error::new(
                Status::GenericFailure,
                format!("failed to subscribe: {}", e),
            )
        })
    }
}

struct PubSubMessageRequest {
    req: Request,
    tx: tokio::sync::mpsc::UnboundedSender<Result<(), api::Error>>,
}

#[derive(Debug)]
pub struct JSSubscriptionHandler {
    handler: Arc<ThreadsafeFunction<PubSubMessageRequest>>,
}

impl pubsub::SubscriptionHandler for JSSubscriptionHandler {
    fn handle_message(
        &self,
        msg: Arc<model::Request>,
    ) -> Pin<Box<dyn Future<Output = Result<(), api::Error>> + Send + '_>> {
        let handler = self.handler.clone();
        Box::pin(async move {
            let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
            let req = Request::new(msg);
            handler.call(
                PubSubMessageRequest { req, tx },
                crate::threadsafe_function::ThreadsafeFunctionCallMode::Blocking,
            );

            match rx.recv().await {
                Some(Ok(())) => Ok(()),
                Some(Err(err)) => Err(err),
                None => Err(api::Error::internal(anyhow::anyhow!(
                    "subscription handler did not respond",
                ))),
            }
        })
    }
}

#[derive(Debug, Clone, Copy)]
struct SubscriptionPromiseHandler;

impl PromiseHandler for SubscriptionPromiseHandler {
    type Output = Result<(), api::Error>;

    fn resolve(&self, _: Env, _: Option<JsUnknown>) -> Self::Output {
        Ok(())
    }

    fn reject(&self, env: Env, val: napi::JsUnknown) -> Self::Output {
        Err(coerce_to_api_error(env, val)?)
    }

    fn error(&self, _: Env, err: Error) -> Self::Output {
        Err(api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some(err.to_string()),
            stack: None,
            details: None,
        })
    }
}

fn resolve_on_js_thread(ctx: ThreadSafeCallContext<PubSubMessageRequest>) -> napi::Result<()> {
    let handler = SubscriptionPromiseHandler;
    let req = ctx.value.req.into_instance(ctx.env)?;
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
