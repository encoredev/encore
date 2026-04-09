use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

use axum::extract::Request;
use axum::response::{IntoResponse, Json};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone)]
pub struct Handler {
    pub app_revision: String,
    pub deploy_id: String,
    pub app_slug: String,
    pub env_name: String,
    pub shutting_down: Arc<AtomicBool>,
}

impl Handler {
    pub fn health_check(self) -> (axum::http::StatusCode, Json<Response>) {
        let details = Details {
            app_slug: self.app_slug,
            env_name: self.env_name,
            app_revision: self.app_revision,
            encore_compiler: "".into(),
            deploy_id: self.deploy_id,
            checks: vec![],
            enabled_experiments: vec![],
        };

        if self.shutting_down.load(Ordering::Relaxed) {
            log::trace!(code = "shutting_down"; "handling incoming health check request during shutdown");
            return (
                axum::http::StatusCode::SERVICE_UNAVAILABLE,
                Json(Response {
                    code: "shutting_down".into(),
                    message: "Service is shutting down".into(),
                    details,
                }),
            );
        }

        log::trace!(code = "ok"; "handling incoming health check request");
        (
            axum::http::StatusCode::OK,
            Json(Response {
                code: "ok".into(),
                message: "Your Encore app is up and running!".into(),
                details,
            }),
        )
    }
}

impl axum::handler::Handler<(), ()> for Handler {
    type Future = std::pin::Pin<
        Box<dyn std::future::Future<Output = axum::response::Response<axum::body::Body>> + Send>,
    >;

    fn call(self, _req: Request, _state: ()) -> Self::Future {
        let resp = self.health_check();
        Box::pin(async move { resp.into_response() })
    }
}

#[derive(Serialize, Deserialize)]
pub struct Response {
    pub code: String,
    pub message: String,
    pub details: Details,
}

#[derive(Serialize, Deserialize)]
pub struct Details {
    pub app_slug: String,
    pub env_name: String,
    pub app_revision: String,
    pub encore_compiler: String,
    pub deploy_id: String,
    pub checks: Vec<CheckResult>,
    pub enabled_experiments: Vec<String>,
}

#[derive(Serialize, Deserialize)]
pub struct CheckResult {
    pub name: String,
    pub passed: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}
