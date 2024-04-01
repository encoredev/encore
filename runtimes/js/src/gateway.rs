use crate::api::to_handler;
use encore_runtime_core::api;
use napi::{Env, JsFunction};
use napi_derive::napi;
use std::sync::Arc;

#[napi]
pub struct Gateway {
    #[allow(dead_code)]
    gateway: Option<Arc<api::gateway::Gateway>>,
}

impl Gateway {
    pub fn new(
        env: Env,
        gateway: Option<Arc<api::gateway::Gateway>>,
        cfg: GatewayConfig,
    ) -> napi::Result<Self> {
        if let Some(gw) = &gateway {
            if let Some(auth) = gw.auth_handler() {
                if let Some(handler) = cfg.auth {
                    let handler: Arc<dyn api::BoxedHandler> = to_handler(env, handler)?;

                    auth.set_local_handler_impl(Some(handler));
                }
            }
        }

        Ok(Self { gateway })
    }
}

#[napi(object)]
pub struct GatewayConfig {
    pub auth: Option<JsFunction>,
}
