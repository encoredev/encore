use crate::api::auth::{AuthHandler, AuthPayload, AuthRequest, AuthResponse};
use crate::api::schema::encoding::Schema;
use crate::api::schema::{JSONPayload};
use crate::api::APIResult;
use crate::log::LogFromRust;
use crate::model::{AuthRequestData, RequestData};
use crate::trace::Tracer;
use crate::{api, model, EndpointName};
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, RwLock};

pub struct LocalAuthHandler {
    pub name: EndpointName,
    pub schema: Schema,
    pub handler: RwLock<Option<Arc<dyn api::BoxedHandler>>>,
    pub tracer: Tracer,
}

impl LocalAuthHandler {
    pub fn set_handler(&self, handler: Option<Arc<dyn api::BoxedHandler>>) {
        let mut guard = self.handler.write().unwrap();
        *guard = handler;
    }
}

impl AuthHandler for LocalAuthHandler {
    fn name(&self) -> &EndpointName {
        &self.name
    }

    fn handle_auth(
        self: Arc<Self>,
        req: AuthRequest,
    ) -> Pin<Box<dyn Future<Output = APIResult<AuthResponse>> + Send + 'static>> {
        let this = self.clone();
        Box::pin(async move {
            let handler = {
                let guard = this.handler.read().unwrap();
                // If we don't have a handler set, return an error.
                let Some(handler) = guard.as_ref() else {
                    return Err(api::Error::internal(anyhow::anyhow!(
                        "auth handler implementation not registered for {}",
                        this.name
                    )));
                };
                handler.clone()
            };

            let query = match &self.schema.query {
                None => None,
                Some(qry) => qry.parse(req.query.as_deref())?,
            };
            let header = match &self.schema.header {
                None => None,
                Some(hdr) => hdr.parse(&req.headers)?,
            };

            let meta = req.call_meta;
            let span_id = meta.this_span_id.unwrap_or_else(model::SpanId::generate);
            let span = model::SpanKey(meta.trace_id, span_id);
            let parent_span = meta.parent_span_id.map(|sp| meta.trace_id.with_span(sp));

            let req = Arc::new(model::Request {
                span,
                parent_trace: None,
                parent_span,
                caller_event_id: meta.parent_event_id,
                ext_correlation_id: meta.ext_correlation_id,
                is_platform_request: false, // TODO
                internal_caller: None,      // TODO
                start: tokio::time::Instant::now(),
                data: RequestData::Auth(AuthRequestData {
                    auth_handler: this.name().clone(),
                    parsed_payload: AuthPayload { query, header },
                }),
            });

            let logger = crate::log::root();
            logger.info(Some(&req), "running auth handler", None);

            self.tracer.request_span_start(&req);
            let auth_response: APIResult<JSONPayload> = handler.call(req.clone()).await;
            let duration = tokio::time::Instant::now().duration_since(req.start);

            if let Err(e) = &auth_response {
                logger.error(Some(&req), "auth handler failed", Some(e), None);
            }
            logger.info(Some(&req), "auth handler completed", {
                let mut fields = crate::log::Fields::new();
                let dur_ms = (duration.as_secs() as f64 * 1000f64)
                    + (duration.subsec_nanos() as f64 / 1_000_000f64);
                fields.insert(
                    "duration".into(),
                    serde_json::Value::Number(serde_json::Number::from_f64(dur_ms).unwrap_or_else(
                        || {
                            // Fall back to integer if the f64 conversion fails
                            serde_json::Number::from(duration.as_millis() as u64)
                        },
                    )),
                );
                Some(fields)
            });

            let result: APIResult<(serde_json::Map<String, serde_json::Value>, String)> =
                match auth_response {
                    Ok(Some(payload)) => {
                        let auth_uid = payload
                            .get("userID")
                            .and_then(|v| v.as_str())
                            .map(String::from);
                        match auth_uid {
                            Some(uid) => Ok((payload, uid.to_string())),
                            None => Err(api::Error {
                                code: api::ErrCode::Unauthenticated,
                                message: "unauthenticated".to_string(),
                                internal_message: Some(
                                    "auth handler did not return a userID field".to_string(),
                                ),
                                stack: None,
                            }),
                        }
                    }
                    Ok(None) => Err(api::Error {
                        code: api::ErrCode::Unauthenticated,
                        message: "unauthenticated".to_string(),
                        internal_message: Some("auth handler returned null".to_string()),
                        stack: None,
                    }),
                    Err(e) => Err(e),
                };

            match result {
                Ok((auth_data, auth_uid)) => {
                    let model_resp = model::Response {
                        request: req.clone(),
                        duration,
                        data: model::ResponseData::Auth(Ok(model::AuthSuccessResponse {
                            user_data: auth_data.clone(),
                            user_id: auth_uid.clone(),
                        })),
                    };
                    self.tracer.request_span_end(&model_resp);
                    Ok(AuthResponse::Authenticated {
                        auth_uid,
                        auth_data,
                    })
                }
                Err(e) => {
                    let model_resp = model::Response {
                        request: req.clone(),
                        duration,
                        data: model::ResponseData::Auth(Err(e.clone())),
                    };
                    self.tracer.request_span_end(&model_resp);
                    Err(e)
                }
            }
        })
    }
}
