use std::borrow::Cow;

use anyhow::Context;

use crate::api::ErrCode;

/// Computes the human-readable `code` label used in metrics and logs for a raw
/// endpoint that responded with the given HTTP status.
///
/// 200 status maps to "ok", statuses that correspond to a known error code map
/// to that code's name (e.g. 404 -> "not_found"), and any other status falls
/// back to "http_<status>".
pub fn code_for_http_status(status: axum::http::StatusCode) -> String {
    if status.as_u16() == 200 {
        return "ok".to_string();
    }
    match ErrCode::from(status) {
        ErrCode::Unknown => format!("http_{}", status.as_u16()),
        code => code.to_string(),
    }
}

pub fn convert_headers(headers: &axum::http::HeaderMap) -> reqwest::header::HeaderMap {
    let mut out = reqwest::header::HeaderMap::with_capacity(headers.len());
    for (k, v) in headers {
        let Ok((k, v)) = convert_header(k, v) else {
            continue;
        };
        out.insert(k, v);
    }
    out
}

pub fn convert_header(
    key: &axum::http::HeaderName,
    value: &axum::http::HeaderValue,
) -> anyhow::Result<(reqwest::header::HeaderName, reqwest::header::HeaderValue)> {
    let k = reqwest::header::HeaderName::from_bytes(key.as_str().as_bytes())
        .context("invalid header name")?;
    let v = reqwest::header::HeaderValue::from_bytes(value.as_bytes())
        .context("invalid header value")?;
    Ok((k, v))
}

pub fn merge_query<'b>(target: Option<&str>, inbound: Option<&'b str>) -> Option<Cow<'b, str>> {
    match (target, inbound) {
        (Some(a), Some(b)) => {
            let mut s = String::with_capacity(a.len() + b.len() + 1);
            s.push_str(a);
            s.push('&');
            s.push_str(b);
            Some(Cow::Owned(s))
        }
        (None, Some(b)) => Some(Cow::Borrowed(b)),
        (_, None) => None,
    }
}

pub fn join_url_path<'b>(target: &str, inbound: &'b str) -> Option<Cow<'b, str>> {
    if inbound.is_empty() {
        return None;
    } else if target.is_empty() {
        return Some(Cow::Borrowed(inbound));
    }

    let a_slash = target.ends_with('/');
    let b_slash = inbound.starts_with('/');
    Some(match (a_slash, b_slash) {
        (true, true) => {
            let mut s = String::with_capacity(target.len() + inbound.len() - 1);
            s.push_str(target);
            s.push_str(&inbound[1..]);
            Cow::Owned(s)
        }
        (false, false) => {
            let mut s = String::with_capacity(target.len() + inbound.len() + 1);
            s.push_str(target);
            s.push('/');
            s.push_str(inbound);
            Cow::Owned(s)
        }
        _ => {
            let mut s = String::with_capacity(target.len() + inbound.len());
            s.push_str(target);
            s.push_str(inbound);
            Cow::Owned(s)
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::http::StatusCode;

    #[test]
    fn test_code_for_http_status() {
        // 200 maps to "ok" rather than "unknown".
        assert_eq!(code_for_http_status(StatusCode::OK), "ok");

        // Known error statuses map to their canonical code name.
        assert_eq!(code_for_http_status(StatusCode::NOT_FOUND), "not_found");
        assert_eq!(
            code_for_http_status(StatusCode::INTERNAL_SERVER_ERROR),
            "internal"
        );
        assert_eq!(
            code_for_http_status(StatusCode::TOO_MANY_REQUESTS),
            "resource_exhausted"
        );

        // Other statuses fall back to "http_<status>" instead of "unknown".
        assert_eq!(code_for_http_status(StatusCode::CREATED), "http_201");
        assert_eq!(code_for_http_status(StatusCode::NO_CONTENT), "http_204");
        assert_eq!(
            code_for_http_status(StatusCode::MOVED_PERMANENTLY),
            "http_301"
        );
        assert_eq!(code_for_http_status(StatusCode::IM_A_TEAPOT), "http_418");
    }
}
