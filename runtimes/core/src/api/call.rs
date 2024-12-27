use std::borrow::{Borrow, Cow};
use std::collections::HashMap;
use std::future::Future;
use std::sync::Arc;
use std::time::SystemTime;

use anyhow::Context;
use tokio_tungstenite::tungstenite::client::IntoClientRequest;
use url::Url;

use encore::runtime::v1 as pb;

use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::meta::MetaKey;
use crate::api::reqauth::{service_auth_method, svcauth};
use crate::api::schema::{JSONPayload, ToOutgoingRequest};
use crate::api::{schema, APIResult, Endpoint, EndpointMap};
use crate::model::{SpanKey, TraceEventId};
use crate::names::EndpointName;
use crate::trace::Tracer;
use crate::{api, encore, model, secrets, EncoreName, Hosted};

use super::reqauth::meta::MetaMapMut;
use super::websocket_client::WebSocketClient;
use super::HandshakeSchema;
use super::ResponsePayload;

/// Tracks where services are located and how to call them.
pub struct ServiceRegistry {
    endpoints: Arc<EndpointMap>,
    base_urls: HashMap<EncoreName, String>,
    http_client: reqwest::Client,
    tracer: Tracer,
    service_auth: HashMap<EncoreName, Arc<dyn svcauth::ServiceAuthMethod>>,
    deploy_id: String,
}

impl ServiceRegistry {
    #[allow(clippy::too_many_arguments)]
    pub fn new(
        secrets: &secrets::Manager,
        endpoints: Arc<EndpointMap>,
        env: &pb::Environment,
        sd: pb::ServiceDiscovery,
        own_address: Option<&str>,
        own_auth_methods: &[Arc<dyn svcauth::ServiceAuthMethod>],
        hosted_services: &Hosted,
        deploy_id: String,
        http_client: reqwest::Client,
        tracer: Tracer,
    ) -> anyhow::Result<Self> {
        let mut base_urls = HashMap::with_capacity(sd.services.len());
        let mut service_auth = HashMap::with_capacity(sd.services.len());
        for (svc, mut loc) in sd.services {
            let svc = EncoreName::from(svc);
            base_urls.insert(svc.clone(), loc.base_url);

            let auth_method = if loc.auth_methods.is_empty() {
                Arc::new(svcauth::Noop)
            } else {
                service_auth_method(secrets, env, loc.auth_methods.swap_remove(0))
                    .context("compute service auth methods")?
            };
            service_auth.insert(svc, auth_method);
        }

        if let Some(own_address) = own_address {
            let own_address = format!("http://{}", own_address);
            for svc_name in hosted_services.iter() {
                if !base_urls.contains_key(svc_name) {
                    let svc = EncoreName::from(svc_name);
                    base_urls.insert(svc.clone(), own_address.clone());

                    let auth_method = if own_auth_methods.is_empty() {
                        Arc::new(svcauth::Noop)
                    } else {
                        own_auth_methods[0].clone()
                    };
                    service_auth.insert(svc, auth_method);
                }
            }
        } else if !hosted_services.is_empty() {
            // This shouldn't happen if things are configured correctly.
            ::log::error!(
                "internal encore error: cannot host services without provided own address"
            );
        }

        Ok(Self {
            endpoints,
            base_urls,
            http_client,
            tracer,
            service_auth,
            deploy_id,
        })
    }

    pub fn service_base_url<Q>(&self, service_name: &Q) -> Option<&String>
    where
        EncoreName: Borrow<Q>,
        Q: Eq + std::hash::Hash + ?Sized,
    {
        self.base_urls.get(service_name)
    }

    pub fn service_auth_method<Q>(
        &self,
        service_name: &Q,
    ) -> Option<Arc<dyn svcauth::ServiceAuthMethod>>
    where
        EncoreName: Borrow<Q>,
        Q: Eq + std::hash::Hash + ?Sized,
    {
        self.service_auth.get(service_name).cloned()
    }

    pub fn api_call(
        &self,
        target: EndpointName,
        data: JSONPayload,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = APIResult<ResponsePayload>> + 'static {
        let tracer = self.tracer.clone();
        let call = model::APICall { source, target };
        let start_event_id = tracer.rpc_call_start(&call);

        let fut = self.do_api_call(&call.target, data, call.source.as_deref(), start_event_id);
        async move {
            let result = fut.await;
            if let Some(start_event_id) = start_event_id {
                tracer.rpc_call_end(&call, start_event_id, result.as_ref().err());
            }
            result
        }
    }

    pub fn connect_stream(
        &self,
        target: EndpointName,
        data: JSONPayload,
        source: Option<Arc<model::Request>>,
    ) -> impl Future<Output = APIResult<WebSocketClient>> + 'static {
        let tracer = self.tracer.clone();
        let call = model::APICall { source, target };
        let start_event_id = tracer.rpc_call_start(&call);

        let fut =
            self.do_connect_stream(&call.target, data, call.source.as_deref(), start_event_id);

        async move {
            let result = fut.await;
            if let Some(start_event_id) = start_event_id {
                tracer.rpc_call_end(&call, start_event_id, result.as_ref().err());
            }
            result
        }
    }

    fn do_api_call(
        &self,
        target: &EndpointName,
        data: JSONPayload,
        source: Option<&model::Request>,
        start_event_id: Option<TraceEventId>,
    ) -> impl Future<Output = APIResult<ResponsePayload>> + 'static {
        let http_client = self.http_client.clone();
        let req = self.prepare_api_call_request(target, data, source, start_event_id);
        async move {
            match req {
                Ok((req, resp_schema)) => {
                    let fut = http_client.execute(req);
                    match fut.await {
                        Ok(resp) => {
                            if !resp.status().is_success() {
                                return Err(extract_error(resp).await);
                            }
                            resp_schema.extract(resp).await
                        }
                        Err(e) => Err(api::Error::internal(e)),
                    }
                }
                Err(e) => Err(e),
            }
        }
    }

    fn prepare_api_call_request(
        &self,
        target: &EndpointName,
        mut data: JSONPayload,
        source: Option<&model::Request>,
        start_event_id: Option<TraceEventId>,
    ) -> APIResult<(reqwest::Request, Arc<schema::Response>)> {
        let base_url = self
            .base_urls
            .get(target.service())
            .ok_or_else(|| api::Error {
                code: api::ErrCode::NotFound,
                message: "service not found".into(),
                internal_message: Some(format!(
                    "no service discovery configuration found for service {}",
                    target.service()
                )),
                stack: None,
                details: None,
            })?;

        let Some(endpoint) = self.endpoints.get(target).cloned() else {
            return Err(api::Error {
                code: api::ErrCode::NotFound,
                message: "endpoint not found".into(),
                internal_message: Some(format!(
                    "endpoint {} not found in application metadata",
                    target
                )),
                stack: None,
                details: None,
            });
        };

        let req_schema = &endpoint.request[0];
        let method = req_schema.methods[0];
        let req_path = req_schema.path.to_request_path(&mut data)?;
        let req_url = format!("{}{}", base_url, req_path);
        let req_url = Url::parse(&req_url).map_err(|_| api::Error {
            code: api::ErrCode::Internal,
            message: "failed to build endpoint url".into(),
            internal_message: Some(format!(
                "failed to build endpoint url for endpoint {}",
                target
            )),
            stack: None,
            details: None,
        })?;

        let mut req = self
            .http_client
            .request(method.into(), req_url)
            .build()
            .map_err(api::Error::internal)?;

        if let Some(qry) = &req_schema.query {
            qry.to_outgoing_request(&mut data, &mut req)?;
        }
        if let Some(hdr) = &req_schema.header {
            hdr.to_outgoing_request(&mut data, &mut req)?;
        }

        match &req_schema.body {
            schema::RequestBody::Typed(Some(body)) => {
                body.to_outgoing_request(&mut data, &mut req)?
            }
            schema::RequestBody::Typed(None) => {}

            schema::RequestBody::Raw => {
                return Err(api::Error {
                    code: api::ErrCode::Internal,
                    message: "internal error".into(),
                    internal_message: Some("cannot make api calls to raw endpoints".to_string()),
                    stack: None,
                    details: None,
                });
            }
        }

        // Add call metadata.
        let headers = req.headers_mut();
        self.propagate_call_meta(headers, &endpoint, source, start_event_id)
            .map_err(api::Error::internal)?;

        let resp_schema = endpoint.response.clone();

        Ok((req, resp_schema))
    }

    fn do_connect_stream(
        &self,
        target: &EndpointName,
        data: JSONPayload,
        source: Option<&model::Request>,
        start_event_id: Option<TraceEventId>,
    ) -> impl Future<Output = APIResult<WebSocketClient>> + 'static {
        let req = self.prepare_stream_request(target, data, source, start_event_id);
        async move {
            match req {
                Ok((req, outgoing, incoming)) => {
                    let schema = schema::Stream::new(incoming, outgoing);
                    WebSocketClient::connect(req, schema).await
                }
                Err(e) => Err(e),
            }
        }
    }

    fn prepare_stream_request(
        &self,
        target: &EndpointName,
        mut data: JSONPayload,
        source: Option<&model::Request>,
        start_event_id: Option<TraceEventId>,
    ) -> APIResult<(
        http::Request<()>,
        Arc<schema::Request>,
        Arc<schema::Response>,
    )> {
        let base_url = self
            .base_urls
            .get(target.service())
            .ok_or_else(|| api::Error {
                code: api::ErrCode::NotFound,
                message: "service not found".into(),
                internal_message: Some(format!(
                    "no service discovery configuration found for service {}",
                    target.service()
                )),
                stack: None,
                details: None,
            })?;

        let Some(endpoint) = self.endpoints.get(target) else {
            return Err(api::Error {
                code: api::ErrCode::NotFound,
                message: "endpoint not found".into(),
                internal_message: Some(format!(
                    "endpoint {} not found in application metadata",
                    target
                )),
                stack: None,
                details: None,
            });
        };

        let Some(handshake) = &endpoint.handshake else {
            return Err(api::Error {
                code: api::ErrCode::NotFound,
                message: "no handshake schema found".into(),
                internal_message: Some(format!(
                    "endpoint {} doesn't have a handshake schema specified",
                    target
                )),
                stack: None,
                details: None,
            });
        };

        let req_path = handshake.path().to_request_path(&mut data)?;

        let base_url = base_url
            .replace("http://", "ws://")
            .replace("https://", "wss://");

        let req_url = Url::parse(&format!("{}{}", base_url, req_path)).map_err(|_| api::Error {
            code: api::ErrCode::Internal,
            message: "failed to build endpoint url".into(),
            internal_message: Some(format!(
                "failed to build endpoint url for endpoint {}",
                target
            )),
            stack: None,
            details: None,
        })?;

        let mut req = req_url
            .into_client_request()
            .map_err(|e| api::Error::invalid_argument("unable to create request", e))?;

        if let HandshakeSchema::Request(req_schema) = handshake.as_ref() {
            if let Some(qry) = &req_schema.query {
                qry.to_outgoing_request(&mut data, &mut req)?;
            }

            if let Some(hdr) = &req_schema.header {
                hdr.to_outgoing_request(&mut data, &mut req)?;
            }
        }

        self.propagate_call_meta(req.headers_mut(), endpoint, source, start_event_id)
            .map_err(api::Error::internal)?;

        let outgoing = endpoint.request[0].clone();
        let incoming = endpoint.response.clone();
        Ok((req, outgoing, incoming))
    }

    fn propagate_call_meta(
        &self,
        headers: &mut reqwest::header::HeaderMap,
        endpoint: &Endpoint,
        source: Option<&model::Request>,
        parent_event_id: Option<TraceEventId>,
    ) -> anyhow::Result<()> {
        let svc_auth_method = self
            .service_auth_method(endpoint.name.service())
            .ok_or_else(|| api::Error {
                code: api::ErrCode::NotFound,
                message: "not found".into(),
                internal_message: Some(format!(
                    "no service auth method found for service {}",
                    endpoint.name.service()
                )),
                stack: None,
                details: None,
            })?;

        let caller = match source {
            Some(source) => match source.data {
                model::RequestData::RPC(ref data) => {
                    Caller::APIEndpoint(data.endpoint.name.clone())
                }
                model::RequestData::Auth(ref data) => {
                    Caller::APIEndpoint(data.auth_handler.clone())
                }
                model::RequestData::PubSub(ref data) => Caller::PubSubMessage {
                    topic: data.topic.clone(),
                    subscription: data.subscription.clone(),
                    message_id: data.message_id.clone(),
                },
                model::RequestData::Stream(ref data) => {
                    Caller::APIEndpoint(data.endpoint.name.clone())
                }
            },
            None => Caller::App {
                deploy_id: self.deploy_id.clone(),
            },
        };

        let desc = CallDesc {
            caller: &caller,
            svc_auth_method: svc_auth_method.as_ref(),
            parent_span: source.map(|r| r.span),
            parent_event_id,
            ext_correlation_id: source.and_then(|r| {
                r.ext_correlation_id
                    .as_ref()
                    .map(|id| Cow::Borrowed(id.as_str()))
            }),
            auth_user_id: source.and_then(|r| {
                match &r.data {
                    model::RequestData::RPC(data) => data.auth_user_id.as_ref(),
                    model::RequestData::Stream(data) => data.auth_user_id.as_ref(),
                    model::RequestData::Auth(_) => None,
                    model::RequestData::PubSub(_) => None,
                }
                .map(|id| Cow::Borrowed(id.as_str()))
            }),
            auth_data: source.and_then(|r| match &r.data {
                model::RequestData::RPC(data) => data.auth_data.as_ref(),
                model::RequestData::Stream(data) => data.auth_data.as_ref(),
                model::RequestData::Auth(_) => None,
                model::RequestData::PubSub(_) => None,
            }),
        };

        desc.add_meta(headers)?;

        Ok(())
    }
}

pub struct CallDesc<'a, AuthData> {
    pub caller: &'a Caller,

    pub parent_span: Option<SpanKey>,
    pub parent_event_id: Option<TraceEventId>,
    pub ext_correlation_id: Option<Cow<'a, str>>,

    pub auth_user_id: Option<Cow<'a, str>>,
    pub auth_data: Option<AuthData>,

    pub svc_auth_method: &'a dyn svcauth::ServiceAuthMethod,
}

impl<'a, AuthData> CallDesc<'a, AuthData>
where
    AuthData: serde::ser::Serialize + 'a,
{
    pub fn add_meta<R: MetaMapMut>(self, headers: &mut R) -> anyhow::Result<()> {
        headers.set(MetaKey::Version, "1".to_string())?;

        if let Some(span) = self.parent_span {
            headers.set(
                MetaKey::TraceParent,
                format!(
                    "00-{}-{}-01",
                    span.0.serialize_std(),
                    span.1.serialize_std(),
                ),
            )?;

            let mut trace_state = format!("encore/span-id={}", span.1.serialize_std());

            if let Some(event_id) = self.parent_event_id.map(|id| id.serialize()) {
                trace_state.push_str(",encore/event-id=");
                trace_state.push_str(event_id.to_string().as_str());
            }
            headers.set(MetaKey::TraceState, trace_state)?;
        }

        // TODO handle GCP span propagation with tracestate key.
        // headers.set(MetaKey::TraceState, "")?;

        if let Some(corr_id) = self.ext_correlation_id {
            headers.set(MetaKey::XCorrelationId, corr_id.into_owned())?;
        }

        // Add auth data.
        if let Some(auth_uid) = self.auth_user_id {
            headers.set(MetaKey::UserId, auth_uid.into_owned())?;
            if let Some(auth_data) = self.auth_data {
                if let Ok(auth_data) = serde_json::to_string(&auth_data) {
                    headers.set(MetaKey::UserData, auth_data)?;
                }
            }
        }

        // Caller.
        headers.set(MetaKey::Caller, self.caller.serialize())?;

        let now = SystemTime::now();

        self.svc_auth_method
            .sign(headers, now)
            .map_err(api::Error::internal)?;

        headers.set(
            MetaKey::SvcAuthMethod,
            self.svc_auth_method.name().to_string(),
        )?;

        Ok(())
    }
}

async fn extract_error(resp: reqwest::Response) -> api::Error {
    match resp.bytes().await {
        Ok(bytes) => serde_json::from_slice(&bytes).unwrap_or_else(|err| {
            api::Error::invalid_argument("unable to parse error response", err)
        }),
        Err(err) => api::Error::invalid_argument("unable to read response body", err),
    }
}
