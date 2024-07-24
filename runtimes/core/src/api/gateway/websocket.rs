use std::{collections::HashMap, str::FromStr};

use anyhow::anyhow;
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine};
use http::{header::SEC_WEBSOCKET_PROTOCOL, HeaderName, HeaderValue};
use pingora::http::RequestHeader;
use serde::{
    de::{self, Visitor},
    Deserialize,
};

const ENCORE_DEV_HEADERS: &str = "encore.dev.headers.";

// hack to be able to have browsers send request headers when setting up a websocket
// inspired by https://github.com/kubernetes/kubernetes/commit/714f97d7baf4975ad3aa47735a868a81a984d1f0
pub fn update_headers_from_websocket_protocol(
    upstream_request: &mut RequestHeader,
) -> anyhow::Result<()> {
    let headers = upstream_request
        .headers
        .get_all(SEC_WEBSOCKET_PROTOCOL)
        .into_iter()
        .cloned()
        .collect::<Vec<_>>();

    if upstream_request
        .remove_header(&SEC_WEBSOCKET_PROTOCOL)
        .is_none()
    {
        return Ok(());
    }

    for header_value in headers {
        let mut filterd_protocols = Vec::new();

        for protocol in header_value.to_str()?.split(',') {
            let protocol = protocol.trim();
            if protocol.starts_with(ENCORE_DEV_HEADERS) {
                let data = protocol.strip_prefix(ENCORE_DEV_HEADERS).unwrap();
                let decoded = URL_SAFE_NO_PAD.decode(data)?;
                let auth_data: AuthHeaderMap = serde_json::from_slice(&decoded)?;

                for (name, value) in auth_data.0 {
                    if is_forbidden_request_header(&name) {
                        return Err(anyhow!("header {name} not allowed to be set"));
                    }
                    upstream_request.append_header(name, value)?;
                }
            } else {
                filterd_protocols.push(protocol);
            }
        }

        if !filterd_protocols.is_empty() {
            upstream_request.append_header(SEC_WEBSOCKET_PROTOCOL, filterd_protocols.join(", "))?;
        }
    }

    Ok(())
}

struct AuthHeaderMap(HashMap<HeaderName, HeaderValue>);

impl<'de> Deserialize<'de> for AuthHeaderMap {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        deserializer.deserialize_map(AuthHeaderMapVisitor)
    }
}

struct AuthHeaderMapVisitor;

impl<'de> Visitor<'de> for AuthHeaderMapVisitor {
    type Value = AuthHeaderMap;

    fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
        formatter.write_str("AuthHeaderMap")
    }

    fn visit_map<A>(self, mut access: A) -> Result<Self::Value, A::Error>
    where
        A: serde::de::MapAccess<'de>,
    {
        let mut map = HashMap::with_capacity(access.size_hint().unwrap_or_default());

        while let Some((key, value)) = access.next_entry::<&str, &str>()? {
            let name = HeaderName::from_str(key).map_err(de::Error::custom)?;
            let value = HeaderValue::from_str(value).map_err(de::Error::custom)?;

            map.insert(name, value);
        }

        Ok(AuthHeaderMap(map))
    }
}

// see https://developer.mozilla.org/en-US/docs/Glossary/Forbidden_header_name
fn is_forbidden_request_header(name: &HeaderName) -> bool {
    use http::header::*;
    match *name {
        ACCEPT_CHARSET
        | ACCEPT_ENCODING
        | ACCESS_CONTROL_REQUEST_HEADERS
        | ACCESS_CONTROL_REQUEST_METHOD
        | CONNECTION
        | CONTENT_LENGTH
        | COOKIE
        | DATE
        | DNT
        | EXPECT
        | HOST
        | ORIGIN
        | REFERER
        | TE
        | TRAILER
        | TRANSFER_ENCODING
        | UPGRADE
        | VIA => true,
        ref n if n == HeaderName::from_static("keep-alive") => true,
        ref n if n == HeaderName::from_static("permissions-policy") => true,
        ref n => {
            let name = n.to_string();
            name.strip_prefix("sec-").is_some() || name.strip_prefix("proxy-").is_some()
        }
    }
}

#[cfg(test)]
mod tests {
    use http::{header::AUTHORIZATION, HeaderMap, HeaderName, HeaderValue};
    use serde_json::json;

    use super::*;

    #[test]
    fn test_headers_from_websocket_protocol_preserve_order() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("protocol-a"),
        )
        .unwrap();
        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("protocol-b1, protocol-b2"),
        )
        .unwrap();
        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("protocol-c"),
        )
        .unwrap();

        let expected = req.headers.clone();

        update_headers_from_websocket_protocol(&mut req)
            .expect("update headers from websocket protocol");

        assert_eq!(expected, req.headers);
    }

    #[test]
    fn test_filter_encore_auth_data_header() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("main-protocol"),
        )
        .unwrap();
        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("protocol-1, encore.dev.auth_data.e30K, protocol-2"),
        )
        .unwrap();
        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("encore.dev.auth_data.e30K"),
        )
        .unwrap();

        update_headers_from_websocket_protocol(&mut req)
            .expect("update headers from websocket protocol");

        let expected = HeaderMap::from_iter(vec![
            (
                SEC_WEBSOCKET_PROTOCOL,
                HeaderValue::from_static("main-protocol"),
            ),
            (
                SEC_WEBSOCKET_PROTOCOL,
                HeaderValue::from_static("protocol-1, protocol-2"),
            ),
        ]);
        assert_eq!(expected, req.headers);
    }

    #[test]
    fn test_adds_auth_data_headers() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        let data = json!({
            "authorization": "token",
            "x-other-header": "value"
        });

        let bytes = serde_json::to_vec(&data).unwrap();
        let encoded = URL_SAFE_NO_PAD.encode(bytes);

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_str(&format!("main-protocol, encore.dev.auth_data.{encoded}"))
                .unwrap(),
        )
        .unwrap();

        update_headers_from_websocket_protocol(&mut req)
            .expect("update headers from websocket protocol");

        let expected = HeaderMap::from_iter(vec![
            (
                SEC_WEBSOCKET_PROTOCOL,
                HeaderValue::from_static("main-protocol"),
            ),
            (AUTHORIZATION, HeaderValue::from_static("token")),
            (
                HeaderName::from_static("x-other-header"),
                HeaderValue::from_static("value"),
            ),
        ]);
        assert_eq!(expected, req.headers);
    }

    #[test]
    fn test_appends_auth_data_headers() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        let data = json!({
            "x-other-header": "new-value"
        });

        let bytes = serde_json::to_vec(&data).unwrap();
        let encoded = URL_SAFE_NO_PAD.encode(bytes);

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_str(&format!("main-protocol, encore.dev.auth_data.{encoded}"))
                .unwrap(),
        )
        .unwrap();
        req.append_header(
            HeaderName::from_static("x-other-header"),
            HeaderValue::from_static("prev-value"),
        )
        .unwrap();

        update_headers_from_websocket_protocol(&mut req)
            .expect("update headers from websocket protocol");

        let expected = HeaderMap::from_iter(vec![
            (
                SEC_WEBSOCKET_PROTOCOL,
                HeaderValue::from_static("main-protocol"),
            ),
            (
                HeaderName::from_static("x-other-header"),
                HeaderValue::from_static("prev-value"),
            ),
            (
                HeaderName::from_static("x-other-header"),
                HeaderValue::from_static("new-value"),
            ),
        ]);
        assert_eq!(expected, req.headers);
    }

    #[test]
    fn test_invalid_auth_data_header() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("main-protocol, encore.dev.auth_data.invalid"),
        )
        .unwrap();

        assert!(update_headers_from_websocket_protocol(&mut req).is_err());
    }

    #[test]
    fn test_forbidden_request_headers() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        let data = json!({
            "cOOkie": "xyz",
        });

        let bytes = serde_json::to_vec(&data).unwrap();
        let encoded = URL_SAFE_NO_PAD.encode(bytes);

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_str(&format!("main-protocol, encore.dev.auth_data.{encoded}"))
                .unwrap(),
        )
        .unwrap();

        assert!(update_headers_from_websocket_protocol(&mut req).is_err());
    }
    #[test]
    fn test_forbidden_prefix_request_headers() {
        let mut req = RequestHeader::build(http::Method::GET, b"/some/path", None).unwrap();

        let data = json!({
            "SEC-anything": "xyz",
        });

        let bytes = serde_json::to_vec(&data).unwrap();
        let encoded = URL_SAFE_NO_PAD.encode(bytes);

        req.append_header(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_str(&format!("main-protocol, encore.dev.auth_data.{encoded}"))
                .unwrap(),
        )
        .unwrap();

        assert!(update_headers_from_websocket_protocol(&mut req).is_err());
    }
}
