use crate::api::auth::{AuthHandler, AuthRequest, AuthResponse};
use crate::api::call::{CallDesc, ServiceRegistry};
use crate::api::httputil::{convert_headers, join_url_path, merge_query};
use crate::api::jsonschema::{DecodeConfig, JSONSchema};
use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::meta::{MetaKey, MetaMap};
use crate::api::reqauth::svcauth;
use crate::api::{APIResult, PValues};
use crate::{api, EndpointName};
use anyhow::Context;
use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

pub struct RemoteAuthHandler {
    name: EndpointName,
    svc_auth_method: Arc<dyn svcauth::ServiceAuthMethod>,
    auth_handler_url: reqwest::Url,
    http_client: reqwest::Client,
    auth_data_schema: JSONSchema,
}

impl RemoteAuthHandler {
    pub fn new(
        name: EndpointName,
        reg: &ServiceRegistry,
        http_client: reqwest::Client,
        auth_data_schema: JSONSchema,
    ) -> anyhow::Result<Self> {
        let svc_auth_method = reg
            .service_auth_method(name.service())
            .context("no service auth method found for auth handler")?;

        let auth_handler_url = {
            let mut base_url: reqwest::Url = reg
                .service_base_url(name.service())
                .context("no base url found for auth handler")?
                .parse()
                .context("invalid service base url")?;

            let auth_path = format!("/__encore/authhandler/{}", name.endpoint());
            let combined_path =
                join_url_path(base_url.path(), &auth_path).context("invalid auth handler path")?;
            base_url.set_path(&combined_path);
            base_url
        };

        Ok(Self {
            name,
            svc_auth_method,
            auth_handler_url,
            http_client,
            auth_data_schema,
        })
    }

    fn build_req(&self, auth_req: &AuthRequest) -> APIResult<reqwest::Request> {
        let dest = self.auth_handler_url.clone();
        let mut req = self
            .http_client
            .post(dest)
            .headers(convert_headers(&auth_req.headers))
            .build()
            .map_err(api::Error::internal)?;

        if let Some(query) = merge_query(req.url().query(), auth_req.query.as_deref()) {
            let query = query.as_ref();
            req.url_mut().set_query(Some(query));
        }

        Ok(req)
    }

    async fn handle_auth(self: Arc<Self>, req: AuthRequest) -> APIResult<AuthResponse> {
        // TODO this is copied from the Go version but should be better designed.
        // We should have a way of identifying the gateway as the caller.
        // There is Caller::Gateway but it means something else.
        let caller = Caller::APIEndpoint(EndpointName::new("gateway", "__encore/authhandler"));

        let meta = &req.call_meta;
        let desc: CallDesc<()> = CallDesc {
            caller: &caller,
            parent_span: meta.parent_span_id.map(|sp| meta.trace_id.with_span(sp)),
            parent_event_id: None,
            ext_correlation_id: meta
                .ext_correlation_id
                .as_ref()
                .map(|s| Cow::Borrowed(s.as_str())),
            auth_user_id: None,
            auth_data: None,
            svc_auth_method: self.svc_auth_method.as_ref(),
        };

        let mut req = self.build_req(&req)?;
        desc.add_meta(req.headers_mut())
            .map_err(api::Error::internal)?;

        let resp = self
            .http_client
            .execute(req)
            .await
            .map_err(api::Error::internal)?;

        // Resolve the user id, if present, since parse_api_response consumes resp.
        let user_id = resp
            .headers()
            .get_meta(MetaKey::UserId)
            .map(|s| s.to_string());

        match parse_auth_response(resp, &self.auth_data_schema).await {
            Ok(data) => {
                if let Some(user_id) = user_id {
                    Ok(AuthResponse::Authenticated {
                        auth_uid: user_id,
                        auth_data: data,
                    })
                } else {
                    Ok(AuthResponse::Unauthenticated {
                        error: api::Error::unauthenticated(),
                    })
                }
            }

            // Map the unauthenticated error code to the unauthenticated result.
            Err(error) if error.code == api::ErrCode::Unauthenticated => {
                Ok(AuthResponse::Unauthenticated { error })
            }

            Err(err) => Err(err),
        }
    }
}

impl AuthHandler for RemoteAuthHandler {
    fn name(&self) -> &EndpointName {
        &self.name
    }

    fn handle_auth(
        self: Arc<Self>,
        req: AuthRequest,
    ) -> Pin<Box<dyn Future<Output = APIResult<AuthResponse>> + Send + 'static>> {
        Box::pin(self.handle_auth(req))
    }
}

async fn parse_auth_response(resp: reqwest::Response, schema: &JSONSchema) -> APIResult<PValues> {
    let status = resp.status();
    if status.is_success() {
        // Do we have a JSON response?
        match resp.headers().get(reqwest::header::CONTENT_TYPE) {
            Some(content_type) if content_type == "application/json" => {
                let bytes = resp.bytes().await.map_err(api::Error::internal)?;
                let mut jsonde = serde_json::Deserializer::from_slice(&bytes);
                let cfg = DecodeConfig {
                    coerce_strings: false,
                };
                let value = schema.deserialize(&mut jsonde, cfg).map_err(|e| {
                    api::Error::invalid_argument("unable to decode response body", e)
                })?;
                Ok(value)
            }
            _ => Err(api::Error::internal(anyhow::anyhow!(
                "missing auth data from auth handler"
            ))),
        }
    } else {
        match resp.headers().get(reqwest::header::CONTENT_TYPE) {
            Some(content_type) if content_type == "application/json" => {
                match resp.json::<api::Error>().await {
                    Ok(data) => Err(data),
                    Err(e) => Err(api::Error::internal(e)),
                }
            }
            _ => {
                // We have some non-JSON error response.
                let body = resp.text().await.unwrap_or_else(|_| "".into());
                Err(api::Error {
                    code: api::ErrCode::Internal,
                    message: body,
                    internal_message: None,
                    stack: None,
                    details: None,
                })
            }
        }
    }
}
