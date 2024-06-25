#![allow(clippy::enum_variant_names)]

use bytes::{BufMut, BytesMut};
use http::{
    header::{self, HeaderName},
    HeaderValue, Method,
};
use pingora::http::{RequestHeader, ResponseHeader};

mod allow_credentials;
mod allow_headers;
mod allow_methods;
mod allow_origin;
mod allow_private_network;
mod expose_headers;
mod max_age;
mod vary;

pub use self::{
    allow_credentials::AllowCredentials, allow_headers::AllowHeaders, allow_methods::AllowMethods,
    allow_origin::AllowOrigin, allow_private_network::AllowPrivateNetwork,
    expose_headers::ExposeHeaders, max_age::MaxAge, vary::Vary,
};

/// Configuration for how cors headers should be appied
///
/// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
#[derive(Debug, Clone)]
#[must_use]
pub struct CorsHeadersConfig {
    allow_credentials: AllowCredentials,
    allow_headers: AllowHeaders,
    allow_methods: AllowMethods,
    allow_origin: AllowOrigin,
    allow_private_network: AllowPrivateNetwork,
    expose_headers: ExposeHeaders,
    max_age: MaxAge,
    vary: Vary,
}

#[allow(clippy::declare_interior_mutable_const)]
const WILDCARD: HeaderValue = HeaderValue::from_static("*");

impl CorsHeadersConfig {
    /// Create a new `CorsHeadersConfig`.
    ///
    /// No headers are sent by default. Use the builder methods to customize
    /// the behavior.
    ///
    /// You need to set at least an allowed origin for browsers to make
    /// successful cross-origin requests to your service.
    pub fn new() -> Self {
        Self {
            allow_credentials: Default::default(),
            allow_headers: Default::default(),
            allow_methods: Default::default(),
            allow_origin: Default::default(),
            allow_private_network: Default::default(),
            expose_headers: Default::default(),
            max_age: Default::default(),
            vary: Default::default(),
        }
    }

    /// A permissive configuration:
    ///
    /// - All request headers allowed.
    /// - All methods allowed.
    /// - All origins allowed.
    /// - All headers exposed.
    pub fn permissive() -> Self {
        Self::new()
            .allow_headers(Any)
            .allow_methods(Any)
            .allow_origin(Any)
            .expose_headers(Any)
    }

    /// A very permissive configuration:
    ///
    /// - **Credentials allowed.**
    /// - The method received in `Access-Control-Request-Method` is sent back
    ///   as an allowed method.
    /// - The origin of the preflight request is sent back as an allowed origin.
    /// - The header names received in `Access-Control-Request-Headers` are sent
    ///   back as allowed headers.
    /// - No headers are currently exposed, but this may change in the future.
    pub fn very_permissive() -> Self {
        Self::new()
            .allow_credentials(true)
            .allow_headers(AllowHeaders::mirror_request())
            .allow_methods(AllowMethods::mirror_request())
            .allow_origin(AllowOrigin::mirror_request())
    }

    /// Set the [`Access-Control-Allow-Credentials`][mdn] header.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Credentials
    pub fn allow_credentials<T>(mut self, allow_credentials: T) -> Self
    where
        T: Into<AllowCredentials>,
    {
        self.allow_credentials = allow_credentials.into();
        self
    }

    /// Set the value of the [`Access-Control-Allow-Headers`][mdn] header.
    ///
    /// Note that multiple calls to this method will override any previous
    /// calls.
    ///
    /// Also note that `Access-Control-Allow-Headers` is required for requests that have
    /// `Access-Control-Request-Headers`.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers
    pub fn allow_headers<T>(mut self, headers: T) -> Self
    where
        T: Into<AllowHeaders>,
    {
        self.allow_headers = headers.into();
        self
    }

    /// Set the value of the [`Access-Control-Max-Age`][mdn] header.
    ///
    /// By default the header will not be set which disables caching and will
    /// require a preflight call for all requests.
    ///
    /// Note that each browser has a maximum internal value that takes
    /// precedence when the Access-Control-Max-Age is greater. For more details
    /// see [mdn].
    ///
    /// If you need more flexibility, you can use supply a function which can
    /// dynamically decide the max-age based on the origin and other parts of
    /// each preflight request, using `MaxAge::dynamic`.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age
    pub fn max_age<T>(mut self, max_age: T) -> Self
    where
        T: Into<MaxAge>,
    {
        self.max_age = max_age.into();
        self
    }

    /// Set the value of the [`Access-Control-Allow-Methods`][mdn] header.
    ///
    /// Note that multiple calls to this method will override any previous
    /// calls.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Methods
    pub fn allow_methods<T>(mut self, methods: T) -> Self
    where
        T: Into<AllowMethods>,
    {
        self.allow_methods = methods.into();
        self
    }

    /// Set the value of the [`Access-Control-Allow-Origin`][mdn] header.
    ///
    /// You can also use a closure with `AllowOrigin::predicate`
    ///
    /// Note that multiple calls to this method will override any previous
    /// calls.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin
    pub fn allow_origin<T>(mut self, origin: T) -> Self
    where
        T: Into<AllowOrigin>,
    {
        self.allow_origin = origin.into();
        self
    }

    /// Set the value of the [`Access-Control-Expose-Headers`][mdn] header.
    ///
    /// Note that multiple calls to this method will override any previous
    /// calls.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Expose-Headers
    pub fn expose_headers<T>(mut self, headers: T) -> Self
    where
        T: Into<ExposeHeaders>,
    {
        self.expose_headers = headers.into();
        self
    }

    /// Set the value of the [`Access-Control-Allow-Private-Network`][wicg] header.
    ///
    /// [wicg]: https://wicg.github.io/private-network-access/
    pub fn allow_private_network<T>(mut self, allow_private_network: T) -> Self
    where
        T: Into<AllowPrivateNetwork>,
    {
        self.allow_private_network = allow_private_network.into();
        self
    }

    /// Set the value(s) of the [`Vary`][mdn] header.
    ///
    /// In contrast to the other headers, this one has a non-empty default of
    /// [`preflight_request_headers()`].
    ///
    /// You only need to set this is you want to remove some of these defaults,
    /// or if you use a closure for one of the other headers and want to add a
    /// vary header accordingly.
    ///
    /// [mdn]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Vary
    pub fn vary<T>(mut self, headers: T) -> Self
    where
        T: Into<Vary>,
    {
        self.vary = headers.into();
        self
    }
}

/// Represents a wildcard value (`*`) used with some CORS headers such as
/// [`CorsHeadersConfig::allow_methods`].
#[derive(Debug, Clone, Copy)]
#[must_use]
pub struct Any;

fn separated_by_commas<I>(mut iter: I) -> Option<HeaderValue>
where
    I: Iterator<Item = HeaderValue>,
{
    match iter.next() {
        Some(fst) => {
            let mut result = BytesMut::from(fst.as_bytes());
            for val in iter {
                result.reserve(val.len() + 1);
                result.put_u8(b',');
                result.extend_from_slice(val.as_bytes());
            }

            Some(HeaderValue::from_maybe_shared(result.freeze()).unwrap())
        }
        None => None,
    }
}

impl Default for CorsHeadersConfig {
    fn default() -> Self {
        Self::new()
    }
}

impl CorsHeadersConfig {
    pub fn apply(&self, req: &RequestHeader, resp: &mut ResponseHeader) -> pingora::Result<()> {
        let origin = req.headers.get(&header::ORIGIN);

        // These headers are applied to both preflight and subsequent regular CORS requests:
        // https://fetch.spec.whatwg.org/#http-responses

        append_response_header(resp, self.allow_credentials.to_header(origin, req))?;
        append_response_header(resp, self.allow_private_network.to_header(origin, req))?;
        append_response_header(resp, self.vary.to_header())?;
        append_response_header(resp, self.allow_origin.to_header(origin, req))?;

        // Return results immediately upon preflight request
        if req.method == Method::OPTIONS {
            // These headers are applied only to preflight requests
            append_response_header(resp, self.allow_methods.to_header(req))?;
            append_response_header(resp, self.allow_headers.to_header(req))?;
            append_response_header(resp, self.max_age.to_header(origin, req))?;
        } else {
            // This header is applied only to non-preflight requests
            append_response_header(resp, self.expose_headers.to_header())?;
        }

        Ok(())
    }
}

fn append_response_header(
    resp: &mut ResponseHeader,
    header: Option<(HeaderName, HeaderValue)>,
) -> pingora::Result<()> {
    if let Some((key, value)) = header {
        resp.append_header(key, value)?;
    }

    Ok(())
}

pub fn ensure_usable_cors_rules(config: &CorsHeadersConfig) {
    if config.allow_credentials.is_true() {
        assert!(
            !config.allow_headers.is_wildcard(),
            "Invalid CORS configuration: Cannot combine `Access-Control-Allow-Credentials: true` \
             with `Access-Control-Allow-Headers: *`"
        );

        assert!(
            !config.allow_methods.is_wildcard(),
            "Invalid CORS configuration: Cannot combine `Access-Control-Allow-Credentials: true` \
             with `Access-Control-Allow-Methods: *`"
        );

        assert!(
            !config.allow_origin.is_wildcard(),
            "Invalid CORS configuration: Cannot combine `Access-Control-Allow-Credentials: true` \
             with `Access-Control-Allow-Origin: *`"
        );

        assert!(
            !config.expose_headers.is_wildcard(),
            "Invalid CORS configuration: Cannot combine `Access-Control-Allow-Credentials: true` \
             with `Access-Control-Expose-Headers: *`"
        );
    }
}

/// Returns an iterator over the three request headers that may be involved in a CORS preflight request.
///
/// This is the default set of header names returned in the `vary` header
pub fn preflight_request_headers() -> impl Iterator<Item = HeaderName> {
    [
        header::ORIGIN,
        header::ACCESS_CONTROL_REQUEST_METHOD,
        header::ACCESS_CONTROL_REQUEST_HEADERS,
    ]
    .into_iter()
}
