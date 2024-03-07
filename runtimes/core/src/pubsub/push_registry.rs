use std::{collections::HashMap, pin::Pin, sync::Arc};

use axum::{extract::Path, RequestExt};
use futures::Future;
use std::sync::RwLock;

use crate::api::{self, IntoResponse};

use super::PushRequestHandler;

#[derive(Debug, Clone)]
pub struct PushHandlerRegistry {
    inner: Arc<Inner>,
}

impl PushHandlerRegistry {
    pub fn new() -> Self {
        Self {
            inner: Arc::new(Inner {
                handlers: Arc::new(RwLock::new(HashMap::new())),
            }),
        }
    }
}

#[derive(Debug)]
struct Inner {
    handlers: Arc<RwLock<HashMap<String, Arc<dyn PushRequestHandler>>>>,
}

impl PushHandlerRegistry {
    pub(super) fn register(&self, subscription_id: String, handler: Arc<dyn PushRequestHandler>) {
        self.inner
            .handlers
            .write()
            .unwrap()
            .insert(subscription_id, handler);
    }

    async fn handle(
        self,
        mut req: axum::extract::Request,
    ) -> axum::response::Response<axum::body::Body> {
        let id: String = match req.extract_parts().await {
            Ok(Path(id)) => id,
            Err(e) => {
                return api::Error::internal(e).into_response();
            }
        };

        let handler = self.inner.handlers.read().unwrap().get(&id).cloned();

        match handler {
            Some(handler) => handler.handle_push(req).await,
            None => {
                api::Error::not_found("no handler registered for push subscription").into_response()
            }
        }
    }
}

impl axum::handler::Handler<(), ()> for PushHandlerRegistry {
    type Future = Pin<Box<dyn Future<Output = axum::response::Response<axum::body::Body>> + Send>>;

    fn call(self, req: axum::extract::Request, _state: ()) -> Self::Future {
        Box::pin(self.handle(req))
    }
}
