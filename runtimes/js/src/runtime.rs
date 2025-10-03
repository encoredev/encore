use crate::api::{new_api_handler, APIRoute, Request};
use crate::gateway::{Gateway, GatewayConfig};
use crate::log::Logger;
use crate::napi_util::EnvMap;
use crate::pubsub::{PubSubSubscription, PubSubSubscriptionConfig, PubSubTopic};
use crate::pvalue::{parse_pvalues, transform_pvalues_request, PVals};
use crate::secret::Secret;
use crate::sqldb::SQLDatabase;
use crate::{meta, objects, websocket_api};
use encore_runtime_core::api::{AuthOpts, PValues};
use encore_runtime_core::pubsub::SubName;
use encore_runtime_core::{api, EncoreName, EndpointName};
use napi::Ref;
use napi::{bindgen_prelude::*, JsFunction, JsObject};
use napi::{Error, JsUnknown, Status};
use napi_derive::napi;
use std::future::Future;
use std::str::FromStr;
use std::sync::{Arc, OnceLock};
use std::thread;

// TODO: remove storing of result after `get_or_try_init` is stabilized
static RUNTIME: OnceLock<napi::Result<Arc<encore_runtime_core::Runtime>>> = OnceLock::new();

// Type constructors registered from javascript so we can create those type from rust
static TYPE_CONSTRUCTORS: EnvMap<Arc<TypeConstructorRefs>> = EnvMap::new();

struct TypeConstructorRefs {
    decimal: Ref<()>,
}

#[napi]
pub struct Runtime {
    pub(crate) runtime: Arc<encore_runtime_core::Runtime>,
}

#[napi]
impl Runtime {
    pub fn create_decimal(env: Env, val: &str) -> napi::Result<JsUnknown> {
        let constructors = TYPE_CONSTRUCTORS.get(env).ok_or_else(|| {
            Error::new(Status::GenericFailure, "Type constructors not initialized")
        })?;

        let constructor: JsFunction = env.get_reference_value(&constructors.decimal)?;
        constructor.call(None, &[env.create_string(val)?])
    }
}

#[napi(object)]
pub struct RuntimeTypeConstructors {
    pub decimal: JsFunction,
}

#[napi(object)]
pub struct RuntimeOptions {
    pub test_mode: Option<bool>,
    pub type_constructors: RuntimeTypeConstructors,
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
                format!("failed to initialize runtime: {e:?}"),
            )
        })
}

#[napi]
impl Runtime {
    #[napi(constructor)]
    pub fn new(env: Env, options: RuntimeOptions) -> napi::Result<Self> {
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

        TYPE_CONSTRUCTORS.get_or_init(env, || {
            Arc::new(TypeConstructorRefs {
                decimal: env
                    .create_reference(options.type_constructors.decimal)
                    .expect("couldn't create reference to Decimal"),
            })
        });

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
        let endpoint_name = encore_runtime_core::EndpointName::new(route.service, route.name);

        let eps = self.runtime.api().endpoints();
        let resp_schema = eps.get(&endpoint_name).map(|ep| ep.response.clone());

        let handler = new_api_handler(
            env,
            route.handler,
            route.raw,
            route.streaming_request || route.streaming_response,
            resp_schema,
        )?;

        // If we're not hosting an API server, this is a no-op.
        let Some(srv) = self.runtime.api().server() else {
            return Ok(());
        };

        srv.register_handler(endpoint_name, handler).map_err(|e| {
            Error::new(
                Status::GenericFailure,
                format!("failed to register handler: {e:?}"),
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
        opts: Option<CallOpts>,
    ) -> napi::Result<JsObject> {
        let endpoint_name = encore_runtime_core::EndpointName::new(service, endpoint);

        let eps = self.runtime.api().endpoints();
        let req_schema = eps.get(&endpoint_name).map(|ep| ep.request[0].clone());

        let payload = payload
            .and_then(|p| parse_pvalues(p).transpose())
            .transpose()?
            .map(|payload| match req_schema {
                Some(schema) => transform_pvalues_request(payload, schema)
                    .map_err(|err| napi::Error::new(napi::Status::InvalidArg, err.to_string())),
                None => Ok(payload),
            })
            .transpose()?;

        let opts = opts.map(TryFrom::try_from).transpose()?;
        let fut = self.do_api_call(endpoint_name, payload, source, opts);
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
        opts: Option<api::CallOpts>,
    ) -> impl Future<Output = api::APIResult<Option<PValues>>> + 'static {
        let source = source.map(|s| s.inner.clone());
        let fut = self.runtime.api().call(endpoint, payload, source, opts);

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
        opts: Option<CallOpts>,
    ) -> napi::Result<JsObject> {
        let payload = match payload {
            Some(payload) => parse_pvalues(payload)?,
            None => None,
        };
        let endpoint = encore_runtime_core::EndpointName::new(service, endpoint);
        let source = source.map(|s| s.inner.clone());
        let opts = opts.map(TryFrom::try_from).transpose()?;
        let fut = self.runtime.api().stream(endpoint, payload, source, opts);

        let fut = async move {
            fut.await.map_err(|e| {
                Error::new(
                    Status::GenericFailure,
                    format!("failed to make api call: {e:?}"),
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

#[napi(object)]
#[derive(Clone, Default, Debug)]
/// CallOpts can be used to set options for API calls.
pub struct CallOpts {
    pub auth_data: Option<PVals>,
}

impl TryFrom<CallOpts> for api::CallOpts {
    type Error = napi::Error<napi::Status>;

    fn try_from(value: CallOpts) -> Result<api::CallOpts> {
        let auth = if let Some(ref data) = value.auth_data {
            let user_id = data
                .0
                .get("userID")
                .and_then(|v| v.as_str())
                .ok_or_else(|| {
                    napi::Error::new(napi::Status::InvalidArg, "userID missing in auth data")
                })?;
            Some(AuthOpts {
                user_id: user_id.to_string(),
                data: data.0.clone(),
            })
        } else {
            None
        };

        Ok(api::CallOpts { auth })
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

#[napi]
pub struct Decimal {
    inner: encore_runtime_core::api::Decimal,
}

#[napi]
impl Decimal {
    #[napi(constructor)]
    pub fn new(value: String) -> napi::Result<Self> {
        match encore_runtime_core::api::Decimal::from_str(&value) {
            Ok(decimal) => Ok(Self { inner: decimal }),
            Err(err) => Err(Error::new(
                Status::InvalidArg,
                format!("Invalid decimal format: '{}'", err),
            )),
        }
    }

    #[napi(js_name = "toString")]
    pub fn js_to_string(&self) -> String {
        self.inner.to_string()
    }

    #[napi]
    pub fn add(&self, other: &Decimal) -> Decimal {
        use std::ops::Add;
        Decimal {
            inner: self.inner.add(&other.inner),
        }
    }

    #[napi]
    pub fn sub(&self, other: &Decimal) -> Decimal {
        use std::ops::Sub;
        Decimal {
            inner: self.inner.sub(&other.inner),
        }
    }

    #[napi]
    pub fn mul(&self, other: &Decimal) -> Decimal {
        use std::ops::Mul;
        Decimal {
            inner: self.inner.mul(&other.inner),
        }
    }

    #[napi]
    pub fn div(&self, other: &Decimal) -> Decimal {
        use std::ops::Div;
        Decimal {
            inner: self.inner.div(&other.inner),
        }
    }
}
