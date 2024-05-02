use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use anyhow::Context;
use napi::bindgen_prelude::spawn;
use napi::{Env, Error, JsFunction, JsUnknown, NapiRaw, Status};
use napi_derive::napi;

use encore_runtime_core::pubsub::{SubscriptionObj, TopicObj};
use encore_runtime_core::{api, model, pubsub};

use crate::api::Request;
use crate::log::parse_js_stack;
use crate::napi_util::{await_promise, PromiseHandler};
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

    #[napi]
    pub async fn publish(
        &self,
        body: Option<serde_json::Value>,
        source: Option<&Request>,
    ) -> napi::Result<String> {
        let source = source.map(|s| s.inner.as_ref());
        let raw_body = serde_json::to_vec_pretty(&body).context("failed to serialize body")?;
        let res = self
            .topic
            .publish(
                pubsub::MessageData {
                    attrs: HashMap::new(), // TODO
                    body,
                    raw_body,
                },
                source,
            )
            .await;

        match res {
            Ok(id) => Ok(id),
            Err(e) => Err(Error::new(
                Status::GenericFailure,
                format!("failed to publish: {}", e),
            )),
        }
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

    fn reject(&self, env: Env, val: JsUnknown) -> Self::Output {
        let obj = val.coerce_to_object().map_err(|_| api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some("an unknown exception was thrown".into()),
            stack: None,
        })?;

        // Get the message field.
        let message: String = obj
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

        Err(api::Error {
            code,
            message,
            stack,
            internal_message: None,
        })
    }

    fn error(&self, _: Env, err: Error) -> Self::Output {
        Err(api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some(err.to_string()),
            stack: None,
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
