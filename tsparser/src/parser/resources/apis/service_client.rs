use crate::parser::resources::apis::api::CallEndpointUsage;
use crate::parser::resources::Resource;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use anyhow::Result;
use swc_common::sync::Lrc;

#[derive(Debug, Clone)]
pub struct ServiceClient {
    pub service_name: String,
}

pub fn resolve_service_client_usage(
    data: &ResolveUsageData,
    client: Lrc<ServiceClient>,
) -> Result<Option<Usage>> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(method) => {
            let method_name = method.method.as_ref();
            // Find the method call on the service client.
            for r in data.resources {
                if let Resource::APIEndpoint(ep) = r {
                    if ep.service_name == client.service_name && ep.name == method_name {
                        return Ok(Some(Usage::CallEndpoint(CallEndpointUsage {
                            range: data.expr.range,
                            endpoint: ep.clone(),
                        })));
                    }
                }
            }

            anyhow::bail!(
                "invalid service client usage: endpoint {}.{} not found",
                client.service_name,
                method_name
            );
        }
        _ => anyhow::bail!("invalid service client usage"),
    }
}
