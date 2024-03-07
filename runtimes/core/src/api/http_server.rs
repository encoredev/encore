use std::convert::Infallible;
use std::task::{Context, Poll};

use axum::body::HttpBody;
use axum::http::Request;
use axum::response::Response;
use axum::routing::future::RouteFuture;
use axum::serve::IncomingStream;
use axum::Router;
use tower_service::Service;

use crate::api::reqauth::meta::MetaKey;

#[derive(Clone)]
pub struct HttpServer {
    encore_routes: Router,
    gateway: Option<Router>,
    api: Option<Router>,
    fallback: Router,
}

impl HttpServer {
    pub fn new(
        encore_routes: Router,
        gateway: Option<Router>,
        api: Option<Router>,
        fallback: Router,
    ) -> Self {
        Self {
            encore_routes,
            gateway,
            api,
            fallback,
        }
    }
}

impl Service<IncomingStream<'_>> for HttpServer {
    type Response = Self;
    type Error = Infallible;
    type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

    #[inline]
    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        Poll::Ready(Ok(()))
    }

    #[inline]
    fn call(&mut self, _req: IncomingStream<'_>) -> Self::Future {
        std::future::ready(Ok(self.clone()))
    }
}

impl<B> Service<Request<B>> for HttpServer
where
    B: HttpBody<Data = bytes::Bytes> + Send + 'static,
    B::Error: Into<axum::BoxError>,
{
    type Response = Response;
    type Error = Infallible;
    type Future = RouteFuture<Infallible>;

    #[inline]
    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        Poll::Ready(Ok(()))
    }

    #[inline]
    fn call(&mut self, req: Request<B>) -> Self::Future {
        if req.uri().path().starts_with("/__encore/") {
            return self.encore_routes.call(req);
        }

        let router = match (self.gateway.as_mut(), self.api.as_mut()) {
            (_, Some(api))
                if req
                    .headers()
                    .contains_key(MetaKey::SvcAuthMethod.header_key()) =>
            {
                api
            }
            (Some(gateway), _) => gateway,
            (_, Some(api)) => api,
            (None, None) => &mut self.fallback,
        };
        router.call(req)
    }
}
