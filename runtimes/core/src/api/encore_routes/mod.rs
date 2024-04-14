use axum::routing;

use crate::pubsub;

pub mod healthz;

pub struct Desc {
    pub healthz: healthz::Handler,
    pub push_registry: pubsub::PushHandlerRegistry,
}

impl Desc {
    pub fn router(self) -> axum::Router<()> {
        axum::Router::new()
            .route("/__encore/healthz", routing::any(self.healthz))
            .route(
                "/__encore/pubsub/push/:subscription_id",
                routing::any(self.push_registry),
            )
    }
}
