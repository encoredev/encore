use crate::api::reqauth::caller::Caller;
use crate::api::reqauth::meta::MetaMap;
use crate::api::APIResult;
use crate::encore::runtime::v1 as pb;
use crate::{api, model, secrets};
use anyhow::Context;
use std::sync::Arc;
use std::time::SystemTime;

pub mod caller;
mod encoreauth;
pub mod meta;
pub mod platform;
pub mod svcauth;

/// Computes the service auth method to use for communicating with a given service.
pub fn service_auth_method(
    secrets: &secrets::Manager,
    env: &pb::Environment,
    auth_method: pb::ServiceAuth,
) -> anyhow::Result<Arc<dyn svcauth::ServiceAuthMethod>> {
    let obj: Arc<dyn svcauth::ServiceAuthMethod> = match auth_method.auth_method {
        None | Some(pb::service_auth::AuthMethod::Noop(_)) => Arc::new(svcauth::Noop),
        Some(pb::service_auth::AuthMethod::EncoreAuth(ea)) => {
            let auth_keys = ea
                .auth_keys
                .into_iter()
                .filter_map(|k| {
                    let data = k.data?;
                    Some(svcauth::EncoreAuthKey {
                        key_id: k.id,
                        data: secrets.load(data),
                    })
                })
                .collect::<Vec<_>>();

            if auth_keys.is_empty() {
                anyhow::bail!("no auth keys provided for encore-auth method");
            }

            Arc::new(svcauth::EncoreAuth::new(
                env.app_slug.clone(),
                env.env_name.clone(),
                auth_keys,
            ))
        }
    };
    Ok(obj)
}

#[derive(Debug, Clone)]
pub struct CallMeta {
    /// The trace id to use. Equal to caller_trace_id if set, and generated otherwise.
    pub trace_id: model::TraceId,

    /// The trace id of the caller; None if not traced.
    pub caller_trace_id: Option<model::TraceId>,

    /// The span id of the caller (None if there's no parent).
    pub parent_span_id: Option<model::SpanId>,

    /// The span id of THIS request, if predefined by the caller (None in most cases).
    pub this_span_id: Option<model::SpanId>,

    /// The event id which started the API call (None if there's no parent).
    pub parent_event_id: Option<model::TraceEventId>,

    /// Correlation id to use.
    pub ext_correlation_id: Option<String>,

    /// Information about an internal call, if any.
    /// If set it can be trusted as it has been authenticated.
    pub internal: Option<InternalCallMeta>,
}

#[derive(Debug, Clone)]
pub struct InternalCallMeta {
    /// The source of the call.
    pub caller: Caller,

    /// The authenticated user id, if any.
    pub auth_uid: Option<String>,
    /// The user data, if any.
    pub auth_data: Option<serde_json::Map<String, serde_json::Value>>,
}

impl CallMeta {
    pub fn parse_with_caller(
        auth: &[Arc<dyn svcauth::ServiceAuthMethod>],
        headers: &axum::http::HeaderMap,
    ) -> APIResult<Self> {
        Self::parse(headers, auth, true)
    }

    pub fn parse_without_caller(headers: &axum::http::HeaderMap) -> APIResult<Self> {
        Self::parse(headers, &[], false)
    }

    fn parse(
        headers: &axum::http::HeaderMap,
        auth: &[Arc<dyn svcauth::ServiceAuthMethod>],
        parse_caller: bool,
    ) -> APIResult<Self> {
        let do_parse = move || -> anyhow::Result<CallMeta> {
            use meta::MetaKey;

            if let Some(version) = headers.get_meta(MetaKey::Version) {
                if version != "1" {
                    anyhow::bail!("unknown encore meta version");
                }
            }

            let mut meta = CallMeta {
                trace_id: model::TraceId::generate(),
                caller_trace_id: None,
                parent_span_id: None,
                this_span_id: None,
                parent_event_id: None,
                ext_correlation_id: None,
                internal: None,
            };

            // If it was an internal call, parse it.
            if parse_caller {
                if let Some(caller) = headers.get_meta(MetaKey::Caller) {
                    // Find the auth method.
                    let auth_method = headers.get_meta(MetaKey::SvcAuthMethod);
                    let Some(auth) = auth.iter().find(|a| auth_method == Some(a.name())) else {
                        anyhow::bail!("unknown service auth method");
                    };

                    // Verify the caller's signature.
                    auth.verify(headers, SystemTime::now())
                        .context("invalid service authentication data")?;

                    let caller = caller.parse().context("invalid meta caller")?;
                    meta.internal = Some(InternalCallMeta {
                        caller,
                        auth_uid: headers.get_meta(MetaKey::UserId).map(|s| s.to_string()),
                        auth_data: headers
                            .get_meta(MetaKey::UserData)
                            .map(serde_json::from_str)
                            .transpose()
                            .context("invalid auth data")?,
                    });
                };
            }

            // For now we only read the traceparent for internal-to-internal calls, this is because CloudRun
            // is adding a traceparent header to all requests, which is causing our trace system to get confused
            // and think that the initial request is a child of another already traced request
            //
            // In the future we should be able to remove this check and read the traceparent header for all requests
            // to interopt with other tracing systems.
            if let Some(traceparent) = headers.get_meta(MetaKey::TraceParent) {
                // Parse the traceparent.
                if let Ok((trace_id, parent_span_id)) = parse_traceparent(traceparent) {
                    meta.trace_id = trace_id;
                    meta.caller_trace_id = Some(trace_id);
                    meta.parent_span_id = Some(parent_span_id);
                };

                // If the caller is a gateway, ignore the parent span id as gateways don't currently record a span.
                // If we include it the root request won't be tagged as such.
                if let Some(internal) = &meta.internal {
                    if matches!(internal.caller, Caller::Gateway { .. }) {
                        meta.parent_span_id = None;
                    }
                }

                // Parse the trace state.
                if let (Some(event_id), parent_span) =
                    parse_tracestate(headers.meta_values(MetaKey::TraceState))
                {
                    meta.parent_event_id = Some(event_id);
                    // If we where given a parent span ID, use that instead of the one from the traceparent header
                    // This is because GCP Cloud Run will add it's own spans in before the application code is run
                    // and thus we lose the parent span ID from the traceparent header
                    if let Some(parent_span) = parent_span {
                        meta.parent_span_id = Some(parent_span);
                    }
                }
            }

            meta.ext_correlation_id = headers.get_meta(MetaKey::XCorrelationId).map(|s| {
                // Limit the maximum length the correlation id can have.
                s[..s.len().min(64)].to_string()
            });

            Ok(meta)
        };

        do_parse().map_err(|e| api::Error::invalid_argument("unable to parse request", e))
    }
}

fn parse_traceparent(s: &str) -> anyhow::Result<(model::TraceId, model::SpanId)> {
    let version = "00";
    let trace_id_len = 32;
    let span_id_len = 16;
    let trace_flags_len = 2;

    let ver_start = 0;
    let ver_end = ver_start + version.len();
    let ver_sep = ver_end;

    let trace_id_start = ver_sep + 1;
    let trace_id_end = trace_id_start + trace_id_len;
    let trace_id_sep = trace_id_end;

    let span_id_start = trace_id_sep + 1;
    let span_id_end = span_id_start + span_id_len;
    let span_id_sep = span_id_end;

    let trace_flags_start = span_id_sep + 1;
    let trace_flags_end = trace_flags_start + trace_flags_len;
    let total_len = trace_flags_end;

    if s.len() != total_len {
        anyhow::bail!("invalid traceparent length");
    } else if &s[ver_start..ver_end] != version {
        anyhow::bail!("invalid traceparent version");
    } else if &s[ver_sep..ver_sep + 1] != "-" {
        anyhow::bail!("invalid traceparent version separator");
    } else if &s[trace_id_sep..trace_id_sep + 1] != "-" {
        anyhow::bail!("invalid traceparent trace id separator");
    } else if &s[span_id_sep..span_id_sep + 1] != "-" {
        anyhow::bail!("invalid traceparent span id separator");
    }

    let trace_id = &s[trace_id_start..trace_id_end];
    let trace_id = model::TraceId::parse_std(trace_id).context("invalid trace id")?;

    let span_id = &s[span_id_start..span_id_end];
    let span_id = model::SpanId::parse_std(span_id).context("invalid span id")?;

    Ok((trace_id, span_id))
}

fn parse_tracestate<'a>(
    vals: impl Iterator<Item = &'a str>,
) -> (Option<model::TraceEventId>, Option<model::SpanId>) {
    enum Data {
        EventId(model::TraceEventId),
        SpanId(model::SpanId),
    }

    let parse_entry = |val: &str| -> Option<Data> {
        let (key, val) = val.split_once('=')?;

        match key {
            "encore/event-id" => Some(Data::EventId(val.parse().ok()?)),
            "encore/span-id" => Some(Data::SpanId(model::SpanId::parse_std(val).ok()?)),
            _ => None,
        }
    };

    let mut event_id = None;
    let mut span_id = None;

    for val in vals {
        for field in val.split(',') {
            match parse_entry(field) {
                Some(Data::EventId(id)) => event_id = Some(id),
                Some(Data::SpanId(id)) => span_id = Some(id),
                None => (),
            }
        }
    }

    (event_id, span_id)
}
