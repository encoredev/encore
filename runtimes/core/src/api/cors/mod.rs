use crate::api::{auth, EndpointMap};
use crate::encore::runtime::v1 as pb;
use crate::encore::runtime::v1::gateway::CorsAllowedOrigins;
use anyhow::Context;
use axum::http::{HeaderName, HeaderValue};
use std::collections::HashSet;
use std::str::FromStr;

use self::cors_headers_config::{ensure_usable_cors_rules, CorsHeadersConfig};

pub mod cors_headers_config;

/// The default set of allowed headers.
#[allow(clippy::declare_interior_mutable_const)]
const ALWAYS_ALLOWED_HEADERS: [HeaderName; 8] = [
    HeaderName::from_static("accept"),
    HeaderName::from_static("authorization"),
    HeaderName::from_static("content-type"),
    HeaderName::from_static("origin"),
    HeaderName::from_static("user-agent"),
    HeaderName::from_static("x-correlation-id"),
    HeaderName::from_static("x-request-id"),
    HeaderName::from_static("x-requested-with"),
];

#[allow(clippy::declare_interior_mutable_const)]
pub const ALWAYS_EXPOSED_HEADERS: [HeaderName; 3] = [
    HeaderName::from_static("x-request-id"),
    HeaderName::from_static("x-correlation-id"),
    HeaderName::from_static("x-encore-trace-id"),
];

pub fn config(cfg: &pb::gateway::Cors, meta: MetaHeaders) -> anyhow::Result<CorsHeadersConfig> {
    let allow_any_headers = cfg.extra_allowed_headers.iter().any(|val| val == "*");

    let allow_headers = if allow_any_headers {
        cors_headers_config::AllowHeaders::mirror_request()
    } else {
        let mut allowed_headers = cfg
            .extra_allowed_headers
            .iter()
            .map(|s| HeaderName::from_str(s))
            .collect::<Result<Vec<_>, _>>()
            .context("failed to parse extra allowed headers")?;
        #[allow(clippy::borrow_interior_mutable_const)]
        allowed_headers.extend_from_slice(&ALWAYS_ALLOWED_HEADERS);
        allowed_headers.extend(meta.allow_headers);

        cors_headers_config::AllowHeaders::list(allowed_headers)
    };

    let mut exposed_headers = cfg
        .extra_exposed_headers
        .iter()
        .map(|s| HeaderName::from_str(s))
        .collect::<Result<Vec<_>, _>>()
        .context("failed to parse extra exposed headers")?;
    #[allow(clippy::borrow_interior_mutable_const)]
    exposed_headers.extend_from_slice(&ALWAYS_EXPOSED_HEADERS);
    exposed_headers.extend(meta.expose_headers);

    // Compute the allowed origins.
    let allow_origin = {
        use pb::gateway::cors::AllowedOriginsWithCredentials;
        let with_creds = match &cfg.allowed_origins_with_credentials {
            Some(AllowedOriginsWithCredentials::UnsafeAllowAllOriginsWithCredentials(true)) => {
                OriginSet::All
            }
            Some(AllowedOriginsWithCredentials::AllowedOrigins(list)) => {
                OriginSet::new(list.allowed_origins.clone())
            }
            _ => OriginSet::Some(vec![]),
        };
        let without_creds = {
            if let Some(CorsAllowedOrigins { allowed_origins }) =
                &cfg.allowed_origins_without_credentials
            {
                OriginSet::new(allowed_origins.to_vec())
            } else {
                OriginSet::All
            }
        };

        let pred = move |origin: &HeaderValue, req: &axum::http::request::Parts| {
            let Ok(origin) = origin.to_str() else {
                return false;
            };
            let headers = &req.headers;
            if headers.contains_key(axum::http::header::AUTHORIZATION)
                || headers.contains_key(axum::http::header::COOKIE)
            {
                with_creds.allows(origin)
            } else {
                without_creds.allows(origin)
            }
        };
        pred
    };

    let config = CorsHeadersConfig::new()
        .allow_private_network(cfg.allow_private_network_access)
        .allow_headers(allow_headers)
        .expose_headers(cors_headers_config::ExposeHeaders::list(exposed_headers))
        .allow_credentials(!cfg.disable_credentials)
        .allow_methods(cors_headers_config::AllowMethods::mirror_request())
        .allow_origin(cors_headers_config::AllowOrigin::predicate(allow_origin));

    ensure_usable_cors_rules(&config);
    Ok(config)
}

enum OriginSet {
    All,
    Some(Vec<Origin>),
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
    Wildcard { prefix: String, suffix: String },
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
                origin.len() > (prefix.len() + suffix.len())
                    && origin.starts_with(prefix)
                    && origin.ends_with(suffix)
            }
        }
    }
}

/// Additional CORS configuration based on the app metadata.
pub struct MetaHeaders {
    pub allow_headers: HashSet<HeaderName>,
    pub expose_headers: HashSet<HeaderName>,
}

impl MetaHeaders {
    pub fn from_schema(endpoints: &EndpointMap, auth: Option<&auth::Authenticator>) -> Self {
        let mut allow_headers = HashSet::new();
        let mut expose_headers = HashSet::new();

        for ep in endpoints.values() {
            if !ep.exposed {
                continue;
            }
            for h in ep.request.iter().flat_map(|req| req.header.iter()) {
                allow_headers.extend(h.header_names());
            }
            expose_headers.extend(ep.response.header.iter().flat_map(|h| h.header_names()));
        }

        // If we have an auth handler, add the auth headers to the allow list.
        if let Some(auth) = auth {
            allow_headers.extend(auth.schema().header.iter().flat_map(|h| h.header_names()));
        }

        Self {
            allow_headers,
            expose_headers,
        }
    }
}

#[cfg(test)]
mod tests {
    use http::header::{
        ACCESS_CONTROL_ALLOW_HEADERS, ACCESS_CONTROL_ALLOW_ORIGIN, ACCESS_CONTROL_REQUEST_HEADERS,
        ACCESS_CONTROL_REQUEST_METHOD, AUTHORIZATION, CONTENT_TYPE, ORIGIN,
    };
    use pingora::http::{RequestHeader, ResponseHeader};

    use super::*;

    fn check_origins(cors: &CorsHeadersConfig, creds: bool, good: bool, origins: &[HeaderValue]) {
        for origin in origins {
            let mut req = RequestHeader::build("OPTIONS", b"/", None).expect("construct request");
            req.insert_header(ORIGIN, origin)
                .expect("insert origin header");

            if creds {
                req.insert_header(AUTHORIZATION, HeaderValue::from_static("dummy"))
                    .expect("insert authorization header");
            }

            let mut resp = ResponseHeader::build(200, None).expect("construct response");

            cors.apply(&req, &mut resp).expect("apply cors config");

            let allow_origin = resp.headers.get(ACCESS_CONTROL_ALLOW_ORIGIN);
            let allowed = allow_origin.map(|val| val == origin).unwrap_or(false);

            if allowed != good {
                panic!(
                    "origin={:?} creds={}: got allowed={}, want {}",
                    origin, creds, allowed, good
                );
            } else {
                println!(
                    "origin={:?} creds={}: ok allowed={}",
                    origin, creds, allowed
                );
            }
        }
    }

    fn check_headers(cors: &CorsHeadersConfig, allowed_headers: &[HeaderName], want_ok: bool) {
        let mut req = RequestHeader::build("OPTIONS", b"/", None).expect("construct request");

        req.insert_header(ORIGIN, HeaderValue::from_static("https://example.org"))
            .expect("insert origin header");

        req.insert_header(
            ACCESS_CONTROL_REQUEST_METHOD,
            HeaderValue::from_static("GET"),
        )
        .expect("insert access-control-request-method");

        let allowed_headers_val = HeaderValue::from_bytes(allowed_headers.join(", ").as_bytes())
            .expect("create access-control-request-headers value");
        req.insert_header(ACCESS_CONTROL_REQUEST_HEADERS, allowed_headers_val)
            .expect("insert access-control-request-headers");

        let mut resp = ResponseHeader::build(200, None).expect("construct response");

        cors.apply(&req, &mut resp).expect("apply cors headers");

        // check allowed headers.
        let allow_headers_val = resp
            .headers
            .get(ACCESS_CONTROL_ALLOW_HEADERS)
            .expect("access-control-allow-headers to be present")
            .to_str()
            .expect("convert to str");
        let allow_headers: HashSet<HeaderName> = allow_headers_val
            .split(",")
            .map(|val| {
                HeaderName::from_bytes(val.trim().as_bytes()).expect("construct header name")
            })
            .collect();

        if want_ok {
            for val in allowed_headers {
                assert!(
                    allow_headers.contains(val),
                    "want header {:?} to be allowed, got false; resp header={}",
                    val,
                    allow_headers_val
                )
            }
        } else {
            assert_ne!(
                allow_headers_val, "",
                "want headers not to be allowed, got {}",
                allow_headers_val
            );
        }
    }

    struct TestCase<'a> {
        cors_cfg: pb::gateway::Cors,
        creds_good_origins: &'a [HeaderValue],
        creds_bad_origins: &'a [HeaderValue],
        nocreds_good_origins: &'a [HeaderValue],
        nocreds_bad_origins: &'a [HeaderValue],
        good_headers: &'a [HeaderName],
        bad_headers: &'a [HeaderName],
    }

    fn run_test_case(test_case: TestCase) {
        let meta = MetaHeaders {
            allow_headers: [HeaderName::from_static("x-static-test")].into(),
            expose_headers: [HeaderName::from_static("x-exposed-test")].into(),
        };

        let cors = config(&test_case.cors_cfg, meta).expect("run cors config");

        check_origins(&cors, true, true, test_case.creds_good_origins);
        check_origins(&cors, true, false, test_case.creds_bad_origins);

        check_origins(&cors, false, true, test_case.nocreds_good_origins);
        check_origins(&cors, false, false, test_case.nocreds_bad_origins);

        check_headers(&cors, test_case.good_headers, true);

        for header in test_case.bad_headers {
            let mut test_headers = test_case.good_headers.to_vec();
            test_headers.push(header.clone());

            check_headers(&cors, &test_headers, false);
        }
    }

    #[test]
    fn test_empty() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: None,
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static("evil.com"),
                HeaderValue::from_static("localhost"),
            ],
            nocreds_good_origins: &[
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static("localhost"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("icanhazcheezburger.com"),
            ],
            nocreds_bad_origins: &[],
            good_headers: &[AUTHORIZATION, CONTENT_TYPE, ORIGIN],
            bad_headers: &[
                HeaderName::from_static("x-requested-with"),
                HeaderName::from_static("x-forwarded-for"),
            ],
        });
    }

    #[test]
    fn test_allowed_creds() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: Some(
                    pb::gateway::cors::AllowedOriginsWithCredentials::AllowedOrigins(
                        pb::gateway::CorsAllowedOrigins {
                            allowed_origins: vec![
                                String::from("localhost"),
                                String::from("ok.org"),
                            ],
                        },
                    ),
                ),
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[
                HeaderValue::from_static("localhost"),
                HeaderValue::from_static("ok.org"),
            ],
            creds_bad_origins: &[
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static("evil.com"),
            ],
            nocreds_good_origins: &[
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static("localhost"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("icanhazcheezburger.com"),
                HeaderValue::from_static("ok.org"),
            ],
            nocreds_bad_origins: &[],
            good_headers: &[],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_allowed_glob_creds() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: Some(
                    pb::gateway::cors::AllowedOriginsWithCredentials::AllowedOrigins(
                        pb::gateway::CorsAllowedOrigins {
                            allowed_origins: vec![
                                String::from("https://*.example.com"),
                                String::from("wss://ok1-*.example.com"),
                                String::from("https://*-ok2.example.com"),
                            ],
                        },
                    ),
                ),
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[
                HeaderValue::from_static("https://foo.example.com"),
                HeaderValue::from_static("wss://ok1-foo.example.com"),
                HeaderValue::from_static("https://foo-ok2.example.com"),
            ],
            creds_bad_origins: &[
                HeaderValue::from_static("http://foo.example.com"), // Wrong scheme
                HeaderValue::from_static("htts://.example.com"),    // No subdomain
                HeaderValue::from_static("ws://ok1-foo.example.com"), // Wrong scheme
                HeaderValue::from_static(".example.com"),           // No scheme
                HeaderValue::from_static("https://evil.com"),       // bad domain
            ],
            nocreds_good_origins: &[],
            nocreds_bad_origins: &[],
            good_headers: &[],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_allowed_nocreds() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: None,
                allowed_origins_without_credentials: Some(pb::gateway::CorsAllowedOrigins {
                    allowed_origins: vec![String::from("localhost"), String::from("ok.org")],
                }),
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[
                HeaderValue::from_static("localhost"),
                HeaderValue::from_static("ok.org"),
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static("evil.com"),
            ],
            nocreds_good_origins: &[
                HeaderValue::from_static("localhost"),
                HeaderValue::from_static("ok.org"),
            ],
            nocreds_bad_origins: &[
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("icanhazcheezburger.com"),
            ],
            good_headers: &[],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_allowed_disjoint_sets() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: Some(
                    pb::gateway::cors::AllowedOriginsWithCredentials::AllowedOrigins(
                        pb::gateway::CorsAllowedOrigins {
                            allowed_origins: vec![String::from("foo.com")],
                        },
                    ),
                ),
                allowed_origins_without_credentials: Some(pb::gateway::CorsAllowedOrigins {
                    allowed_origins: vec![String::from("bar.org")],
                }),
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[HeaderValue::from_static("foo.com")],
            creds_bad_origins: &[
                HeaderValue::from_static("bar.org"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("localhost"),
            ],
            nocreds_good_origins: &[HeaderValue::from_static("bar.org")],
            nocreds_bad_origins: &[
                HeaderValue::from_static("foo.com"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("localhost"),
            ],
            good_headers: &[],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_allowed_wildcard_without_creds() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: None,
                allowed_origins_without_credentials: Some(pb::gateway::CorsAllowedOrigins {
                    allowed_origins: vec![String::from("*")],
                }),
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[
                HeaderValue::from_static("bar.org"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("localhost"),
            ],
            nocreds_good_origins: &[
                HeaderValue::from_static("bar.org"),
                HeaderValue::from_static("bar.com"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("localhost"),
            ],
            nocreds_bad_origins: &[],
            good_headers: &[],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_allowed_unsafe_wildcard_with_creds() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: Some(
                    pb::gateway::cors::AllowedOriginsWithCredentials::UnsafeAllowAllOriginsWithCredentials(true),
                ),
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[
                HeaderValue::from_static("bar.org"),
                HeaderValue::from_static("bar.com"),
                HeaderValue::from_static(""),
                HeaderValue::from_static("localhost"),
                HeaderValue::from_static("unsafe.evil.com"),
            ],
            creds_bad_origins: &[],
            nocreds_good_origins: &[],
            nocreds_bad_origins: &[],
            good_headers: &[],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_extra_headers() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: None,
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec!["X-Forwarded-For".to_string(), "X-Real-Ip".to_string()],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[],
            nocreds_good_origins: &[],
            nocreds_bad_origins: &[],
            good_headers: &[
                AUTHORIZATION,
                CONTENT_TYPE,
                ORIGIN,
                HeaderName::from_static("x-requested-with"),
                HeaderName::from_static("x-real-ip"),
            ],
            bad_headers: &[
                HeaderName::from_static("x-forwarded-for"),
                HeaderName::from_static("x-evil-header"),
            ],
        });
    }

    #[test]
    fn test_extra_headers_wildcard() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: None,
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec![
                    "X-Forwarded-For".to_string(),
                    "*".to_string(),
                    "X-Real-Ip".to_string(),
                ],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[],
            nocreds_good_origins: &[],
            nocreds_bad_origins: &[],
            good_headers: &[
                AUTHORIZATION,
                CONTENT_TYPE,
                ORIGIN,
                HeaderName::from_static("x-requested-with"),
                HeaderName::from_static("x-real-ip"),
                HeaderName::from_static("x-forwarded-for"),
                HeaderName::from_static("x-evil-header"),
            ],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_static_headers() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: None,
                allowed_origins_without_credentials: None,
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[],
            nocreds_good_origins: &[],
            nocreds_bad_origins: &[],
            good_headers: &[
                AUTHORIZATION,
                CONTENT_TYPE,
                ORIGIN,
                HeaderName::from_static("x-static-test"),
            ],
            bad_headers: &[],
        });
    }

    #[test]
    fn test_wildcard_without_creds() {
        run_test_case(TestCase {
            cors_cfg: pb::gateway::Cors {
                debug: false,
                disable_credentials: false,
                allowed_origins_with_credentials: Some(
                    pb::gateway::cors::AllowedOriginsWithCredentials::AllowedOrigins(
                        pb::gateway::CorsAllowedOrigins {
                            allowed_origins: vec![String::from("https://vercel.app")],
                        },
                    ),
                ),
                allowed_origins_without_credentials: Some(pb::gateway::CorsAllowedOrigins {
                    allowed_origins: vec![String::from("https://*-foo.vercel.app")],
                }),
                extra_allowed_headers: vec![],
                extra_exposed_headers: vec![],
                allow_private_network_access: false,
            },
            creds_good_origins: &[],
            creds_bad_origins: &[HeaderValue::from_static("https://blah-foo.vercel.app")],
            nocreds_good_origins: &[HeaderValue::from_static("https://blah-foo.vercel.app")],
            nocreds_bad_origins: &[],
            good_headers: &[],
            bad_headers: &[],
        });
    }
}
