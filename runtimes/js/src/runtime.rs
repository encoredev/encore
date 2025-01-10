use crate::api::{new_api_handler, APIRoute, Request};
use crate::gateway::{Gateway, GatewayConfig};
use crate::log::Logger;
use crate::pubsub::{PubSubSubscription, PubSubSubscriptionConfig, PubSubTopic};
use crate::pvalue::{parse_pvalues, PVals};
use crate::secret::Secret;
use crate::sqldb::SQLDatabase;
use crate::{meta, objects, websocket_api};
use encore_runtime_core::api::PValues;
use encore_runtime_core::pubsub::SubName;
use encore_runtime_core::{api, EncoreName, EndpointName};
use napi::{bindgen_prelude::*, JsObject};
use napi::{Error, JsUnknown, Status};
use napi_derive::napi;
use std::future::Future;
use std::sync::{Arc, OnceLock};
use std::thread;

// TODO: remove storing of result after `get_or_try_init` is stabilized
static RUNTIME: OnceLock<napi::Result<Arc<encore_runtime_core::Runtime>>> = OnceLock::new();

#[napi]
pub struct Runtime {
    pub(crate) runtime: Arc<encore_runtime_core::Runtime>,
}

#[napi(object)]
#[derive(Default)]
pub struct RuntimeOptions {
    pub test_mode: Option<bool>,
}

fn init_runtime(test_mode: bool) -> napi::Result<encore_runtime_core::Runtime> {
    // Initialize logging.
    encore_runtime_core::log::init();

    encore_runtime_core::Runtime::builder()
        .with_test_mode(test_mode)
        .with_meta_autodetect()
        .with_runtime_config_from_env()
        .build()
        .map_err(|e| {
            Error::new(
                Status::GenericFailure,
                format!("failed to initialize runtime: {:?}", e),
            )
        })
}

#[napi]
impl Runtime {
    #[napi(constructor)]
    pub fn new(options: Option<RuntimeOptions>) -> napi::Result<Self> {
        let options = options.unwrap_or_default();
        let test_mode = options
            .test_mode
            .unwrap_or(std::env::var("NODE_ENV").is_ok_and(|val| val == "test"));

        if test_mode {
            // Don't reuse the runtime in tests, as vitest and other test frameworks
            // use multiple workers to isolate tests from each other. We don't want tests
            // to be making API calls to other tests' workers.
            let runtime = Arc::new(init_runtime(true)?);

            // If we're running tests, there's no specific entrypoint so
            // start the runtime in the background immediately.
            {
                let rt = runtime.clone();
                thread::spawn(move || {
                    rt.run_blocking();
                });
            }

            return Ok(Self { runtime });
        }

        let runtime = RUNTIME
            .get_or_init(|| Ok(Arc::new(init_runtime(false)?)))
            .clone()?;

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
    pub fn bucket(&self, encore_name: String) -> napi::Result<objects::Bucket> {
        let bkt = self
            .runtime
            .objects()
            .bucket(encore_name.into())
            .ok_or_else(|| Error::new(Status::GenericFailure, "bucket not found"))?;
        Ok(objects::Bucket::new(bkt))
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

    #[napi(ts_return_type = "Promise<Record<string, any> | null | ApiCallError>")]
    pub fn api_call(
        &self,
        env: Env,
        service: String,
        endpoint: String,
        payload: Option<JsUnknown>,
        source: Option<&Request>,
    ) -> napi::Result<JsObject> {
        let payload = match payload {
            Some(payload) => parse_pvalues(payload)?,
            None => None,
        };
        let endpoint = encore_runtime_core::EndpointName::new(service, endpoint);

        let fut = self.do_api_call(endpoint, payload, source);
        let fut = async move {
            let res: napi::Result<Either<Option<PValues>, APICallError>> = match fut.await {
                Ok(data) => Ok(Either::A(data)),
                Err(err) => Ok(Either::B(err.into())),
            };
            res
        };

        env.execute_tokio_future(fut, |&mut _env, res| {
            Ok(match res {
                Either::A(pvals) => Either::A(pvals.map(PVals)),
                Either::B(err) => Either::B(err),
            })
        })
    }

    fn do_api_call<'a>(
        &'a self,
        endpoint: EndpointName,
        payload: Option<PValues>,
        source: Option<&'a Request>,
    ) -> impl Future<Output = api::APIResult<Option<PValues>>> + 'static {
        let source = source.map(|s| s.inner.clone());
        let fut = self.runtime.api().call(endpoint, payload, source);

        async move {
            let data = fut.await?;
            Ok(match (data.header, data.body) {
                (None, api::Body::Raw(_) | api::Body::Typed(None)) => None,
                (None, api::Body::Typed(Some(body))) => Some(body),
                (Some(header), api::Body::Raw(_) | api::Body::Typed(None)) => Some(header),
                (Some(header), api::Body::Typed(Some(body))) => {
                    let mut combined = header;
                    combined.extend(body);
                    Some(combined)
                }
            })
        }
    }

    #[napi(ts_return_type = "Promise<WebSocketClient>")]
    pub fn stream(
        &self,
        env: Env,
        service: String,
        endpoint: String,
        payload: Option<JsUnknown>,
        source: Option<&Request>,
    ) -> napi::Result<JsObject> {
        let payload = match payload {
            Some(payload) => parse_pvalues(payload)?,
            None => None,
        };
        let endpoint = encore_runtime_core::EndpointName::new(service, endpoint);
        let source = source.map(|s| s.inner.clone());
        let fut = self.runtime.api().stream(endpoint, payload, source);

        let fut = async move {
            fut.await.map_err(|e| {
                Error::new(
                    Status::GenericFailure,
                    format!("failed to make api call: {:?}", e),
                )
            })
        };

        env.execute_tokio_future(fut, |&mut _env, res| {
            Ok(websocket_api::WebSocketClient::new(res))
        })
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

    /// Reports the total number of worker threads,
    /// including the main thread.
    #[napi]
    pub fn num_worker_threads(&self) -> u32 {
        match self.runtime.compute().worker_threads {
            Some(n) => {
                if n > 0 {
                    n as u32
                } else {
                    num_cpus::get() as u32
                }
            }
            None => 1u32,
        }
    }
}

#[napi]
pub struct APICallError {
    pub code: String,
    pub message: String,
    pub details: Option<PVals>,
}

impl From<api::Error> for APICallError {
    fn from(value: api::Error) -> Self {
        Self {
            code: value.code.to_string(),
            message: value.message,
            details: value.details.map(|d| PVals(*d)),
        }
    }
}
