use crate::api::{new_api_handler, APIRoute, Request};
use crate::gateway::{Gateway, GatewayConfig};
use crate::log::Logger;
use crate::pubsub::{PubSubSubscription, PubSubSubscriptionConfig, PubSubTopic};
use crate::secret::Secret;
use crate::sqldb::SQLDatabase;
use crate::{meta, websocket_api};
use encore_runtime_core::api::schema::JSONPayload;
use encore_runtime_core::pubsub::SubName;
use encore_runtime_core::EncoreName;
use napi::bindgen_prelude::*;
use napi::{Error, Status};
use napi_derive::napi;
use std::sync::Arc;
use std::thread;

#[napi]
pub struct Runtime {
    pub(crate) runtime: Arc<encore_runtime_core::Runtime>,
}

#[napi(object)]
#[derive(Default)]
pub struct RuntimeOptions {
    pub test_mode: Option<bool>,
    pub is_worker: Option<bool>,
}

#[napi]
impl Runtime {
    #[napi(constructor)]
    pub fn new(options: Option<RuntimeOptions>) -> napi::Result<Self> {
        let options = options.unwrap_or_default();
        // Initialize logging.
        encore_runtime_core::log::init();

        let test_mode = options
            .test_mode
            .unwrap_or(std::env::var("NODE_ENV").is_ok_and(|val| val == "test"));
        let is_worker = options.is_worker.unwrap_or(false);
        let runtime = encore_runtime_core::Runtime::builder()
            .with_test_mode(test_mode)
            .with_meta_autodetect()
            .with_runtime_config_from_env()
            .with_worker(is_worker)
            .build()
            .map_err(|e| {
                Error::new(
                    Status::GenericFailure,
                    format!("failed to initialize runtime: {:?}", e),
                )
            })?;
        let runtime = Arc::new(runtime);

        // If we're running tests, there's no specific entrypoint so
        // start the runtime in the background immediately.
        if test_mode {
            let runtime = runtime.clone();
            thread::spawn(move || {
                runtime.run_blocking();
            });
        }

        Ok(Self { runtime })
    }

    #[napi]
    pub async fn run_forever(&self) {
        let runtime = self.runtime.clone();
        thread::spawn(move || {
            runtime.run_blocking();
        });

        // Block the async function forever.
        futures::future::pending::<()>().await;
    }

    #[napi]
    pub fn sql_database(&self, encore_name: String) -> SQLDatabase {
        let encore_name: encore_runtime_core::EncoreName = encore_name.into();
        let db = self.runtime.sqldb().database(&encore_name);
        SQLDatabase::new(db)
    }

    #[napi]
    pub fn pubsub_topic(&self, encore_name: String) -> napi::Result<PubSubTopic> {
        let topic = self
            .runtime
            .pubsub()
            .topic(encore_name.into())
            .ok_or_else(|| Error::new(Status::GenericFailure, "topic not found"))?;
        Ok(PubSubTopic::new(topic))
    }

    #[napi]
    pub fn gateway(
        &self,
        env: Env,
        encore_name: String,
        cfg: GatewayConfig,
    ) -> napi::Result<Gateway> {
        let name: EncoreName = encore_name.into();
        let gw = self.runtime.api().gateway(&name).cloned();
        Gateway::new(env, gw, cfg)
    }

    /// Gets the root logger from the runtime
    #[napi]
    pub fn logger(&self) -> Logger {
        Logger::new()
    }

    #[napi]
    pub fn pubsub_subscription(
        &self,
        env: Env,
        cfg: PubSubSubscriptionConfig,
    ) -> napi::Result<PubSubSubscription> {
        let handler = Arc::new(cfg.to_handler(env)?);
        let sub = self
            .runtime
            .pubsub()
            .subscription(SubName {
                topic: cfg.topic_name.into(),
                subscription: cfg.subscription_name.into(),
            })
            .ok_or_else(|| Error::new(Status::GenericFailure, "subscription not found"))?;
        Ok(PubSubSubscription::new(sub, handler))
    }

    #[napi]
    pub fn register_handler(&self, env: Env, route: APIRoute) -> napi::Result<()> {
        let handler = new_api_handler(
            env,
            route.handler,
            route.raw,
            route.streaming_request || route.streaming_response,
        )?;

        // If we're not hosting an API server, this is a no-op.
        let Some(srv) = self.runtime.api().server() else {
            return Ok(());
        };

        let endpoint = encore_runtime_core::EndpointName::new(route.service, route.name);
        srv.register_handler(endpoint, handler).map_err(|e| {
            Error::new(
                Status::GenericFailure,
                format!("failed to register handler: {:?}", e),
            )
        })
    }

    #[napi]
    pub fn register_test_handler(&self, env: Env, route: APIRoute) -> napi::Result<()> {
        // Currently no difference between test and non-test handlers.
        self.register_handler(env, route)
    }

    #[napi]
    pub fn register_handlers(&self, env: Env, routes: Vec<APIRoute>) -> napi::Result<()> {
        for route in routes {
            self.register_handler(env, route)?;
        }
        Ok(())
    }

    #[napi]
    pub fn secret(&self, encore_name: String) -> Option<Secret> {
        self.runtime
            .secrets()
            .app_secret(encore_name.into())
            .map(Secret::new)
    }

    #[napi]
    pub async fn api_call(
        &self,
        service: String,
        endpoint: String,
        data: JSONPayload,
        source: Option<&Request>,
    ) -> Either<JSONPayload, APICallError> {
        let endpoint = encore_runtime_core::EndpointName::new(service, endpoint);
        let source = source.map(|s| s.inner.as_ref());
        let res = self.runtime.api().call(&endpoint, data, source).await;

        match res {
            Ok(data) => Either::A(data),
            Err(e) => Either::B(APICallError {
                code: e.code.to_string(),
                message: e.message,
            }),
        }
    }

    #[napi]
    pub async fn stream(
        &self,
        service: String,
        endpoint: String,
        data: JSONPayload,
        source: Option<&Request>,
    ) -> napi::Result<websocket_api::WebSocketClient> {
        let endpoint = encore_runtime_core::EndpointName::new(service, endpoint);
        let source = source.map(|s| s.inner.as_ref());
        let client = self
            .runtime
            .api()
            .stream(&endpoint, data, source)
            .await
            .map_err(|e| {
                Error::new(
                    Status::GenericFailure,
                    format!("failed to make api call: {:?}", e),
                )
            })?;

        Ok(websocket_api::WebSocketClient::new(client))
    }

    /// Returns the version of the Encore runtime being used
    #[napi]
    pub fn version() -> String {
        encore_runtime_core::version().to_string()
    }

    /// Returns the git commit hash used to build the Encore runtime
    #[napi]
    pub fn build_commit() -> String {
        encore_runtime_core::build_commit().to_string()
    }

    #[napi]
    pub fn app_meta(&self) -> meta::AppMeta {
        let md = self.runtime.app_meta();
        md.clone().into()
    }
}

#[napi]
pub struct APICallError {
    pub code: String,
    pub message: String,
}
