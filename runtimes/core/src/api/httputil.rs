use std::borrow::Cow;

use anyhow::Context;

pub fn convert_method(method: axum::http::Method) -> reqwest::Method {
    use axum::http::Method as I;
    use reqwest::Method as O;
    match method {
        I::GET => O::GET,
        I::POST => O::POST,
        I::PUT => O::PUT,
        I::DELETE => O::DELETE,
        I::HEAD => O::HEAD,
        I::OPTIONS => O::OPTIONS,
        I::CONNECT => O::CONNECT,
        I::PATCH => O::PATCH,
        I::TRACE => O::TRACE,
        _ => unreachable!("invalid method: {:?}", method),
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

pub fn join_request_url(inbound: &axum::http::Uri, target: &mut reqwest::Url) {
    if let Some(target_path) = join_url_path(target.path(), inbound.path()) {
        target.set_path(target_path.as_ref());
    }
    if let Some(target_query) = merge_query(target.query(), inbound.query()) {
        target.set_query(Some(target_query.as_ref()));
    }
}

pub fn merge_query<'b>(
    target: Option<&str>,
    inbound: Option<&'b str>,
) -> Option<Cow<'b, str>> {
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
