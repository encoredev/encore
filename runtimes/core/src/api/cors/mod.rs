use std::str::FromStr;
use anyhow::Context;
use axum::http::{HeaderName, HeaderValue};
use tower_http::cors;
use crate::encore::runtime::v1 as pb;

/// The default set of allowed headers.
const ALWAYS_ALLOWED_HEADERS : [HeaderName; 8] = [
    HeaderName::from_static("accept"),
    HeaderName::from_static("authorization"),
    HeaderName::from_static("content-type"),
    HeaderName::from_static("origin"),
    HeaderName::from_static("user-agent"),
    HeaderName::from_static("x-correlation-id"),
    HeaderName::from_static("x-request-id"),
    HeaderName::from_static("x-requested-with"),
];

pub const ALWAYS_EXPOSED_HEADERS: [HeaderName; 3] = [
    HeaderName::from_static("x-request-id"),
    HeaderName::from_static("x-correlation-id"),
    HeaderName::from_static("x-encore-trace-id"),
];

pub fn layer(cfg: &pb::gateway::Cors) -> anyhow::Result<cors::CorsLayer> {
    let mut allowed_headers = cfg.extra_allowed_headers.iter()
        .map(|s| HeaderName::from_str(&s)).collect::<Result<Vec<_>, _>>()
        .context("failed to parse extra allowed headers")?;
    allowed_headers.extend_from_slice(&ALWAYS_ALLOWED_HEADERS);

    let mut exposed_headers = cfg.extra_exposed_headers.iter()
        .map(|s| HeaderName::from_str(&s)).collect::<Result<Vec<_>, _>>()
        .context("failed to parse extra exposed headers")?;
    exposed_headers.extend_from_slice(&ALWAYS_EXPOSED_HEADERS);

    // Compute the allowed origins.
    let allow_origin = {
        use pb::gateway::cors::AllowedOriginsWithCredentials;
        let with_creds = match &cfg.allowed_origins_with_credentials {
            Some(AllowedOriginsWithCredentials::UnsafeAllowAllOriginsWithCredentials(true)) => OriginSet::All,
            Some(AllowedOriginsWithCredentials::AllowedOrigins(list)) => OriginSet::new(list.allowed_origins.clone()),
            _ => OriginSet::Some(vec![]),
        };
        let without_creds = OriginSet::new(cfg.allowed_origins_without_credentials.clone().unwrap_or_default().allowed_origins);

        let pred = move |origin: &HeaderValue, req: &axum::http::request::Parts| {
            let Ok(origin) = origin.to_str() else {
                return false;
            };
            let headers = &req.headers;
            if headers.contains_key(axum::http::header::AUTHORIZATION) || headers.contains_key(axum::http::header::COOKIE) {
                with_creds.allows(origin)
            } else {
                without_creds.allows(origin)
            }
        };
        pred
    };

    let layer = cors::CorsLayer::new()
        .allow_private_network(cfg.allow_private_network_access)
        .allow_headers(cors::AllowHeaders::list(allowed_headers))
        .expose_headers(cors::ExposeHeaders::list(exposed_headers))
        .allow_credentials(!cfg.disable_credentials)
        .allow_methods(cors::AllowMethods::mirror_request())
        .allow_origin(cors::AllowOrigin::predicate(allow_origin))
        ;
    Ok(layer)
}

enum OriginSet {
    All,
    Some(Vec<crate::api::cors::Origin>)
}

impl OriginSet {
    fn new(origins: Vec<String>) -> Self {
        let mut set = Vec::with_capacity(origins.len());
        for o in origins {
            if o == "*" {
                return Self::All;
            }
            set.push(crate::api::cors::Origin::new(o));
        }
        Self::Some(set)
    }

    fn allows(&self, origin: &str) -> bool {
        let origin = origin.to_lowercase();
        match self {
            Self::All => true,
            Self::Some(origins) => origins.iter().any(|o| o.matches(&origin)),
        }
    }
}

enum Origin {
    Exact(String),
    Wildcard {
        prefix: String,
        suffix: String,
    }
}

impl Origin {
    fn new(origin: String) -> Self {
        match origin.split_once('*') {
            Some((prefix, suffix)) => Self::Wildcard {
                prefix: prefix.to_string(),
                suffix: suffix.to_string(),
            },
            None => Self::Exact(origin),
        }
    }

    fn matches(&self, origin: &str) -> bool {
        match self {
            Self::Exact(exact) => origin == exact,
            Self::Wildcard { prefix, suffix } => {
                // Length must be greater than the prefix and suffix combined,
                // to ensure the wildcard matches at least one character.
                origin.len() > (prefix.len()+suffix.len()) &&
                    origin.starts_with(prefix) &&
                    origin.ends_with(suffix)
            },
        }
    }
}
