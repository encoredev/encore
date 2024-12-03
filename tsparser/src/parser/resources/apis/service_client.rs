use crate::parser::resources::apis::api::CallEndpointUsage;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::span_err::ErrReporter;
use swc_common::sync::Lrc;

#[derive(Debug, Clone)]
pub struct ServiceClient {
    pub service_name: String,
}

pub fn resolve_service_client_usage(
    data: &ResolveUsageData,
    client: Lrc<ServiceClient>,
) -> Option<Usage> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(method) => {
            let method_name = method.method.as_ref();

            Some(Usage::CallEndpoint(CallEndpointUsage {
                range: data.expr.range,
                endpoint: (client.service_name.clone(), method_name.to_string()),
            }))
        }
        _ => {
            data.expr.err("invalid service client usage");
            None
        }
    }
}
