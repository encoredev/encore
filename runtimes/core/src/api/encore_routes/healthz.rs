use axum::extract::Request;
use axum::response::{IntoResponse, Json};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone)]
pub struct Handler {
    pub app_revision: String,
    pub deploy_id: String,
}

impl axum::handler::Handler<(), ()> for Handler {
    type Future = std::pin::Pin<
        Box<dyn std::future::Future<Output = axum::response::Response<axum::body::Body>> + Send>,
    >;

    fn call(self, _req: Request, _state: ()) -> Self::Future {
        Box::pin(async move {
            Json(Response {
                code: "ok".into(),
                message: "Your Encore app is up and running!".into(),
                details: Details {
                    app_revision: self.app_revision,
                    encore_compiler: "".into(),
                    deploy_id: self.deploy_id,
                    checks: vec![],
                    enabled_experiments: vec![],
                },
            })
            .into_response()
        })
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
