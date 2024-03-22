use std::future::Future;
use std::pin::Pin;
use std::str::FromStr;
use std::sync::Arc;

use crate::api::httputil::{convert_headers, convert_method, join_request_url};
use crate::api::reqauth::CallMeta;
use crate::api::{auth, APIResult, IntoResponse};
use crate::{api, model};

#[derive(Debug)]
pub struct ReverseProxy<D> {
    inner: Arc<Inner<D>>,
}

impl<D> ReverseProxy<D> {
    pub fn new(director: D, client: reqwest::Client) -> Self {
        Self {
            inner: Arc::new(Inner { director, client }),
        }
    }
}

impl<D> Clone for ReverseProxy<D> {
    fn clone(&self) -> Self {
        Self {
            inner: self.inner.clone(),
        }
    }
}

#[derive(Debug)]
struct Inner<D> {
    director: D,
    client: reqwest::Client,
}

#[derive(Debug)]
pub struct InboundRequest {
    pub method: axum::http::Method,
    pub url: axum::http::Uri,
    pub headers: axum::http::HeaderMap,

    modified_query: Option<String>,
}

#[derive(Debug)]
pub struct ProxyRequest {
    pub method: reqwest::Method,
    pub url: reqwest::Url,
    pub headers: reqwest::header::HeaderMap,
}

impl InboundRequest {
    pub fn set_query(&mut self, query: String) {
        self.modified_query = Some(query);
    }

    pub fn build<U>(self, target: U) -> APIResult<ProxyRequest>
    where
        U: reqwest::IntoUrl,
    {
        let mut dest = target.into_url().map_err(api::Error::internal)?;

        if let Some(query) = self.modified_query {
            dest.set_query(Some(&query));
        }

        join_request_url(&self.url, &mut dest);

        let proxy = ProxyRequest {
            method: convert_method(self.method),
            headers: convert_headers(&self.headers),
            url: dest,
        };
        Ok(proxy)
    }
}

impl auth::InboundRequest for InboundRequest {
    fn headers(&self) -> &axum::http::HeaderMap {
        &self.headers
    }

    fn query(&self) -> Option<&str> {
        self.url.query()
    }
}

pub trait Director: Clone + Send + Sync + 'static {
    type Future: Future<Output = APIResult<ProxyRequest>> + Send + 'static;

    fn direct(self, req: InboundRequest) -> Self::Future;
}

impl<D> Inner<D>
where
    D: Director,
{
    async fn handle(
        self: Arc<Self>,
        inbound: axum::extract::Request,
    ) -> axum::http::Response<axum::body::Body> {
        let res = self.proxy(inbound).await;
        res.unwrap_or_else(|err| err.into_response())
    }

    async fn proxy(
        self: Arc<Self>,
        inbound: axum::extract::Request,
    ) -> APIResult<axum::http::Response<axum::body::Body>> {
        let req = self.route(inbound).await?;

        let res = self
            .client
            .execute(req)
            .await
            .map_err(api::Error::internal)?;

        let mut outbound = axum::http::Response::builder().status(res.status().as_u16());
        if let Some(headers) = outbound.headers_mut() {
            for (k, v) in res.headers() {
                let k =
                    axum::http::HeaderName::from_str(k.as_str()).map_err(api::Error::internal)?;
                let v = axum::http::HeaderValue::from_bytes(v.as_bytes())
                    .map_err(api::Error::internal)?;
                headers.insert(k, v);
            }
        }

        let resp = outbound
            .body(axum::body::Body::from_stream(res.bytes_stream()))
            .map_err(api::Error::internal)?;
        Ok(resp)
    }

    /// Process an inbound request into an outbound request.
    async fn route(&self, inbound: axum::extract::Request) -> APIResult<reqwest::Request> {
        let (parts, body) = inbound.into_parts();

        let proxy = self
            .director
            .clone()
            .direct(InboundRequest {
                method: parts.method,
                url: parts.uri,
                headers: parts.headers,
                modified_query: None,
            })
            .await?;

        // Note: this drops non-data frames like trailers.
        // We don't support them at the moment, but will in the future.
        let stream = body.into_data_stream();

        // Make the stream implement Sync. See https://github.com/seanmonstar/reqwest/pull/2088.
        let stream = sync_wrapper::SyncStream::new(stream);

        let req = self
            .client
            .request(proxy.method, proxy.url)
            .headers(proxy.headers)
            .body(reqwest::Body::wrap_stream(stream))
            .build()
            .map_err(api::Error::internal)?;

        Ok(req)
    }
}

impl<D> axum::handler::Handler<(), ()> for ReverseProxy<D>
where
    D: Director,
{
    type Future =
        Pin<Box<dyn Future<Output = axum::http::Response<axum::body::Body>> + Send + 'static>>;

    fn call(self, axum_req: axum::extract::Request, _state: ()) -> Self::Future {
        Box::pin(self.inner.handle(axum_req))
    }
}
