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
            req.insert_header(ACCESS_CONTROL_REQUEST_HEADERS, AUTHORIZATION)
                .expect("insert access-control-request-headers");
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

    if !allowed_headers.is_empty() && want_ok {
        // check that the CORS response is valid (e.g if its a request with credentials)
        assert!(
            resp.headers.get(ACCESS_CONTROL_ALLOW_ORIGIN).is_some(),
            "expected cors request to be valid, but access-control-allow-origin is not set"
        )
    }

    // check allowed headers.
    let allow_headers_val = resp
        .headers
        .get(ACCESS_CONTROL_ALLOW_HEADERS)
        .expect("access-control-allow-headers to be present")
        .to_str()
        .expect("convert to str");
    let allow_headers: HashSet<HeaderName> = allow_headers_val
        .split(",")
        .map(|val| HeaderName::from_bytes(val.trim().as_bytes()).expect("construct header name"))
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
        good_headers: &[CONTENT_TYPE, ORIGIN],
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
                        allowed_origins: vec![String::from("localhost"), String::from("ok.org")],
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
            extra_allowed_headers: vec![
                "Not-Authorization".to_string(),
                "X-Forwarded-For".to_string(),
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
            CONTENT_TYPE,
            ORIGIN,
            HeaderName::from_static("x-requested-with"),
            HeaderName::from_static("x-real-ip"),
            HeaderName::from_static("not-authorization"),
        ],
        bad_headers: &[
            HeaderName::from_static("x-forwarded-for"),
            HeaderName::from_static("x-evil-header"),
            HeaderName::from_static("authorization"),
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
            CONTENT_TYPE,
            ORIGIN,
            HeaderName::from_static("x-requested-with"),
            HeaderName::from_static("x-real-ip"),
            HeaderName::from_static("x-forwarded-for"),
            HeaderName::from_static("x-evil-header"),
            HeaderName::from_static("not-authorization"),
        ],
        bad_headers: &[HeaderName::from_static("authorization")],
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
