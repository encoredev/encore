use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, RwLock};

use anyhow::{Context, Result};
use axum::extract::Request;
use axum::RequestExt;
use chrono::{DateTime, Utc};
use http_body_util::BodyExt;
use serde::Deserialize;

use crate::api::{self, APIResult, IntoResponse};
use crate::encore::runtime::v1 as pb;
use crate::pubsub::manager::SubHandler;
use crate::pubsub::{self, MessageId};

use super::jwk::{self, CachingClient};

#[derive(Debug, Clone)]
pub struct PushSubscription {
    inner: Arc<Inner>,
}

#[derive(Debug)]
struct Inner {
    subscription_id: String,
    handler: RwLock<Option<Arc<SubHandler>>>,
    validator: GoogleJWTValidator,
}

impl PushSubscription {
    pub(super) fn new(cfg: &pb::PubSubSubscription) -> Self {
        let Some(pb::pub_sub_subscription::ProviderConfig::GcpConfig(gcp_cfg)) =
            cfg.provider_config.as_ref()
        else {
            panic!("missing gcp config for subscription")
        };

        let Some(service_account) = &gcp_cfg.push_service_account else {
            panic!("missing push_service_account for subscription")
        };

        let google_validator = GoogleJWTValidator {
            client: CachingClient::new(),
            push_service_account: service_account.clone(),
            audience: gcp_cfg.push_jwt_audience.clone(),
        };

        Self {
            inner: Arc::new(Inner {
                subscription_id: cfg.rid.clone(),
                handler: RwLock::new(None),
                validator: google_validator,
            }),
        }
    }
}

impl pubsub::Subscription for PushSubscription {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = Result<()>> + Send + '_>> {
        self.inner.handler.write().unwrap().replace(handler);

        // Block forever; the handler is called from the HTTP handler.
        Box::pin(futures::future::pending())
    }

    fn push_handler(&self) -> Option<(String, Arc<dyn pubsub::PushRequestHandler>)> {
        Some((
            self.inner.subscription_id.clone(),
            Arc::new(PushHandler {
                inner: self.inner.clone(),
            }),
        ))
    }
}

#[derive(Debug, Clone)]
struct PushHandler {
    inner: Arc<Inner>,
}

impl pubsub::PushRequestHandler for PushHandler {
    fn handle_push(
        &self,
        req: Request,
    ) -> Pin<Box<dyn Future<Output = axum::response::Response<axum::body::Body>> + Send + 'static>>
    {
        let inner = self.inner.clone();
        Box::pin(async move {
            match inner.handle_req(req).await {
                Ok(()) => axum::response::Response::new(axum::body::Body::empty()),
                Err(e) => {
                    log::error!("push handler returned error: {:?}", e);
                    e.into_response()
                }
            }
        })
    }
}

/// Payload of a push message from GCP Pub/Sub.
/// This is documented in https://cloud.google.com/pubsub/docs/push
#[derive(Debug, Deserialize)]
struct PushPayload {
    message: PushMessage,
    #[allow(dead_code)]
    subscription: String,
    #[serde(rename = "deliveryAttempt")]
    delivery_attempt: Option<u32>,
}

#[derive(Debug, Deserialize)]
struct PushMessage {
    #[serde(default)]
    attributes: HashMap<String, String>,
    #[serde(with = "base64")]
    data: Vec<u8>,
    #[serde(rename = "messageId")]
    message_id: String,
    #[serde(rename = "publishTime")]
    publish_time: DateTime<Utc>,
}

impl Inner {
    async fn handle_req(&self, req: Request) -> APIResult<()> {
        // Do we have a handler registered yet? If not, there's no point in proceeding.
        let handler = {
            let read_guard = self.handler.read().unwrap();
            let Some(handler) = (*read_guard).clone() else {
                return Err(api::Error {
                    code: api::ErrCode::Internal,
                    message: "no handler registered for subscription".to_string(),
                    internal_message: None,
                    stack: None,
                });
            };
            handler
        };

        // Validate the JWT token.
        _ = self
            .validator
            .validate_google_jwt(req.headers())
            .await
            .map_err(api::Error::internal)?;

        // Parse the request payload.
        let bytes = req
            .into_limited_body()
            .collect()
            .await
            .map_err(api::Error::internal)?
            .to_bytes();
        let msg: PushPayload = serde_json::from_slice(&bytes).map_err(api::Error::internal)?;

        let body: Option<serde_json::Value> = serde_json::from_slice(&msg.message.data)
            .map_err(|e| api::Error::invalid_argument("unable to parse message body as JSON", e))?;

        let msg = pubsub::Message {
            id: msg.message.message_id as MessageId,
            publish_time: Some(msg.message.publish_time),
            attempt: msg.delivery_attempt.unwrap_or(1),
            data: pubsub::MessageData {
                attrs: msg.message.attributes,
                body,
                raw_body: msg.message.data,
            },
        };

        match handler.handle_message(msg).await {
            Ok(()) => Ok(()),
            Err(err) => {
                log::info!("message handler failed, nacking message: {:?}", err);
                Err(err)
            }
        }
    }
}

#[derive(Debug)]
struct GoogleJWTValidator {
    client: jwk::CachingClient,
    audience: Option<String>,
    push_service_account: String,
}

/// The certs URL for RSA keys.
const GOOGLE_SA_CERTS_URL: &str = "https://www.googleapis.com/oauth2/v3/certs";

/// The certs URL for other keys.
const GOOGLE_IAP_CERTS_URL: &str = "https://www.gstatic.com/iap/verify/public_key-jwk";

impl GoogleJWTValidator {
    pub async fn validate_google_jwt(&self, req: &axum::http::HeaderMap) -> anyhow::Result<()> {
        // Extract the JWT from the header
        let auth_header = req
            .get("Authorization")
            .ok_or_else(|| anyhow::anyhow!("missing auth header"))?;
        let token = auth_header
            .to_str()
            .map_err(|_| anyhow::anyhow!("invalid auth header"))?;
        let token = token
            .strip_prefix("Bearer ")
            .ok_or_else(|| anyhow::anyhow!("invalid auth header"))?;

        let token_header = jsonwebtoken::decode_header(token)?;
        let Some(token_key_id) = token_header.kid.as_ref() else {
            return Err(anyhow::anyhow!("missing kid in token header"));
        };

        let url = match token_header.alg {
            jsonwebtoken::Algorithm::RS256
            | jsonwebtoken::Algorithm::RS384
            | jsonwebtoken::Algorithm::RS512 => GOOGLE_SA_CERTS_URL,
            _ => GOOGLE_IAP_CERTS_URL,
        };

        // Get the JWK set to validate the token against.
        let jwks = self
            .client
            .get(url)
            .await
            .context("unable to fetch JWK keys")?;

        // Find the key that matches the token.
        let jwk_key = jwks.find(&token_key_id).ok_or_else(|| {
            anyhow::anyhow!("unable to find JWK key for token: {:?}", token_key_id)
        })?;

        // Decode all the claims.
        #[derive(Deserialize)]
        struct Claims {
            // Custom claims from GCP
            email: String,
            email_verified: bool,
        }

        let decoding_key = jsonwebtoken::DecodingKey::from_jwk(jwk_key)
            .context("unable to create JWT decoding key")?;

        // Per the Go GCP library, the only supported algorithms are RS256 and ES256.
        let alg = match token_header.alg {
            jsonwebtoken::Algorithm::RS256 | jsonwebtoken::Algorithm::ES256 => token_header.alg,
            _ => {
                return Err(anyhow::anyhow!(
                    "unexpected algorithm: {:?}",
                    token_header.alg
                ));
            }
        };

        let mut validation = jsonwebtoken::Validation::new(alg);
        if let Some(aud) = &self.audience {
            validation.set_audience(&[aud]);
        }
        validation.set_issuer(&["accounts.google.com", "https://accounts.google.com"]);
        validation.set_required_spec_claims(&["exp", "iss", "aud"]);

        let jwt = jsonwebtoken::decode::<Claims>(token, &decoding_key, &validation)
            .context("unable to decode JWT claims")?;
        if jwt.claims.email != self.push_service_account {
            return Err(anyhow::anyhow!("invalid email"));
        }
        if !jwt.claims.email_verified {
            return Err(anyhow::anyhow!("email not verified"));
        }

        Ok(())
    }
}

mod base64 {
    use base64::engine::{general_purpose::STANDARD, Engine};
    use serde::{Deserialize, Serialize};
    use serde::{Deserializer, Serializer};

    #[allow(dead_code)]
    pub fn serialize<S: Serializer>(v: &Vec<u8>, s: S) -> Result<S::Ok, S::Error> {
        let base64 = STANDARD.encode(v);
        String::serialize(&base64, s)
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<Vec<u8>, D::Error> {
        let base64 = String::deserialize(d)?;
        STANDARD
            .decode(base64.as_bytes())
            .map_err(|e| serde::de::Error::custom(e))
    }
}
