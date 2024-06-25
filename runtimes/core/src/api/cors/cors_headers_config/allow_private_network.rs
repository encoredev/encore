use std::{fmt, sync::Arc};

use http::{
    header::{HeaderName, HeaderValue},
    request::Parts as RequestParts,
};

/// Holds configuration for how to set the [`Access-Control-Allow-Private-Network`][wicg] header.
///
/// See [`CorsLayer::allow_private_network`] for more details.
///
/// [wicg]: https://wicg.github.io/private-network-access/
/// [`CorsLayer::allow_private_network`]: super::CorsLayer::allow_private_network
#[derive(Clone, Default)]
#[must_use]
pub struct AllowPrivateNetwork(AllowPrivateNetworkInner);

impl AllowPrivateNetwork {
    /// Allow requests via a more private network than the one used to access the origin
    ///
    /// See [`CorsLayer::allow_private_network`] for more details.
    ///
    /// [`CorsLayer::allow_private_network`]: super::CorsLayer::allow_private_network
    pub fn yes() -> Self {
        Self(AllowPrivateNetworkInner::Yes)
    }

    /// Allow requests via private network for some requests, based on a given predicate
    ///
    /// The first argument to the predicate is the request origin.
    ///
    /// See [`CorsLayer::allow_private_network`] for more details.
    ///
    /// [`CorsLayer::allow_private_network`]: super::CorsLayer::allow_private_network
    pub fn predicate<F>(f: F) -> Self
    where
        F: Fn(&HeaderValue, &RequestParts) -> bool + Send + Sync + 'static,
    {
        Self(AllowPrivateNetworkInner::Predicate(Arc::new(f)))
    }

    #[allow(
        clippy::declare_interior_mutable_const,
        clippy::borrow_interior_mutable_const
    )]
    pub(super) fn to_header(
        &self,
        origin: Option<&HeaderValue>,
        parts: &RequestParts,
    ) -> Option<(HeaderName, HeaderValue)> {
        #[allow(clippy::declare_interior_mutable_const)]
        const REQUEST_PRIVATE_NETWORK: HeaderName =
            HeaderName::from_static("access-control-request-private-network");

        #[allow(clippy::declare_interior_mutable_const)]
        const ALLOW_PRIVATE_NETWORK: HeaderName =
            HeaderName::from_static("access-control-allow-private-network");

        const TRUE: HeaderValue = HeaderValue::from_static("true");

        // Cheapest fallback: allow_private_network hasn't been set
        if let AllowPrivateNetworkInner::No = &self.0 {
            return None;
        }

        // Access-Control-Allow-Private-Network is only relevant if the request
        // has the Access-Control-Request-Private-Network header set, else skip
        if parts.headers.get(REQUEST_PRIVATE_NETWORK) != Some(&TRUE) {
            return None;
        }

        let allow_private_network = match &self.0 {
            AllowPrivateNetworkInner::Yes => true,
            AllowPrivateNetworkInner::No => false, // unreachable, but not harmful
            AllowPrivateNetworkInner::Predicate(c) => c(origin?, parts),
        };

        allow_private_network.then_some((ALLOW_PRIVATE_NETWORK, TRUE))
    }
}

impl From<bool> for AllowPrivateNetwork {
    fn from(v: bool) -> Self {
        match v {
            true => Self(AllowPrivateNetworkInner::Yes),
            false => Self(AllowPrivateNetworkInner::No),
        }
    }
}

impl fmt::Debug for AllowPrivateNetwork {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.0 {
            AllowPrivateNetworkInner::Yes => f.debug_tuple("Yes").finish(),
            AllowPrivateNetworkInner::No => f.debug_tuple("No").finish(),
            AllowPrivateNetworkInner::Predicate(_) => f.debug_tuple("Predicate").finish(),
        }
    }
}
type PredicateFn =
    Arc<dyn for<'a> Fn(&'a HeaderValue, &'a RequestParts) -> bool + Send + Sync + 'static>;

#[derive(Clone)]
enum AllowPrivateNetworkInner {
    Yes,
    No,
    Predicate(PredicateFn),
}

impl Default for AllowPrivateNetworkInner {
    fn default() -> Self {
        Self::No
    }
}

#[cfg(test)]
mod tests {
    #![allow(
        clippy::declare_interior_mutable_const,
        clippy::borrow_interior_mutable_const
    )]

    use super::AllowPrivateNetwork;
    use crate::api::cors::cors_headers_config::CorsHeadersConfig;

    use http::{header::ORIGIN, request::Parts, HeaderName, HeaderValue};
    use pingora::http::{RequestHeader, ResponseHeader};

    const REQUEST_PRIVATE_NETWORK: HeaderName =
        HeaderName::from_static("access-control-request-private-network");

    const ALLOW_PRIVATE_NETWORK: HeaderName =
        HeaderName::from_static("access-control-allow-private-network");

    const TRUE: HeaderValue = HeaderValue::from_static("true");

    #[tokio::test]
    async fn cors_private_network_header_is_added_correctly() {
        let conf = CorsHeadersConfig::new().allow_private_network(true);

        let mut req = RequestHeader::build(http::Method::POST, b"/some/path", None).unwrap();
        req.insert_header(REQUEST_PRIVATE_NETWORK, TRUE).unwrap();
        let mut resp = ResponseHeader::build(200, None).unwrap();

        conf.apply(&req, &mut resp).unwrap();
        assert_eq!(resp.headers.get(ALLOW_PRIVATE_NETWORK).unwrap(), TRUE);

        let req = RequestHeader::build(http::Method::POST, b"/some/path", None).unwrap();
        let mut resp = ResponseHeader::build(200, None).unwrap();

        conf.apply(&req, &mut resp).unwrap();
        assert!(resp.headers.get(ALLOW_PRIVATE_NETWORK).is_none());
    }

    #[tokio::test]
    async fn cors_private_network_header_is_added_correctly_with_predicate() {
        let allow_private_network =
            AllowPrivateNetwork::predicate(|origin: &HeaderValue, parts: &Parts| {
                parts.uri.path() == "/allow-private" && origin == "localhost"
            });

        let conf = CorsHeadersConfig::new().allow_private_network(allow_private_network);

        let mut req = RequestHeader::build(http::Method::POST, b"/allow-private", None).unwrap();
        req.insert_header(ORIGIN, "localhost").unwrap();
        req.insert_header(REQUEST_PRIVATE_NETWORK, TRUE).unwrap();
        let mut resp = ResponseHeader::build(200, None).unwrap();

        conf.apply(&req, &mut resp).unwrap();
        assert_eq!(resp.headers.get(ALLOW_PRIVATE_NETWORK).unwrap(), TRUE);

        let mut req = RequestHeader::build(http::Method::POST, b"/other", None).unwrap();
        req.insert_header(ORIGIN, "localhost").unwrap();
        req.insert_header(REQUEST_PRIVATE_NETWORK, TRUE).unwrap();
        let mut resp = ResponseHeader::build(200, None).unwrap();

        conf.apply(&req, &mut resp).unwrap();
        assert!(resp.headers.get(ALLOW_PRIVATE_NETWORK).is_none());

        let mut req = RequestHeader::build(http::Method::POST, b"/allow-private", None).unwrap();
        req.insert_header(ORIGIN, "not-localhost").unwrap();
        req.insert_header(REQUEST_PRIVATE_NETWORK, TRUE).unwrap();
        let mut resp = ResponseHeader::build(200, None).unwrap();

        conf.apply(&req, &mut resp).unwrap();
        assert!(resp.headers.get(ALLOW_PRIVATE_NETWORK).is_none());
    }
}
