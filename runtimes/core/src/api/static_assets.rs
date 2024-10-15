use std::{convert::Infallible, future::Future, path::PathBuf, pin::Pin, sync::Arc};

use http_body_util::Empty;
use std::fmt::Debug;
use std::io;
use tower_http::services::{fs::ServeDir, ServeFile};
use tower_service::Service;

use crate::{encore::parser::meta::v1 as meta, model::RequestData};

use super::{BoxedHandler, Error, HandlerRequest, ResponseData};

#[derive(Clone, Debug)]
pub struct StaticAssetsHandler {
    service: Arc<dyn FileServer>,
    not_found_handler: bool,
}

impl StaticAssetsHandler {
    pub fn new(cfg: &meta::rpc::StaticAssets) -> Self {
        let service = ServeDir::new(PathBuf::from(&cfg.dir_rel_path));

        let not_found = cfg
            .not_found_rel_path
            .as_ref()
            .map(|p| ServeFile::new(PathBuf::from(p)));
        let not_found_handler = not_found.is_some();

        let service: Arc<dyn FileServer> = match not_found {
            Some(not_found) => Arc::new(service.not_found_service(not_found)),
            None => Arc::new(service),
        };
        StaticAssetsHandler {
            service,
            not_found_handler,
        }
    }
}

impl BoxedHandler for StaticAssetsHandler {
    fn call(
        self: Arc<Self>,
        req: HandlerRequest,
    ) -> Pin<Box<dyn Future<Output = ResponseData> + Send + 'static>> {
        Box::pin(async move {
            let RequestData::RPC(data) = &req.data else {
                return ResponseData::Typed(Err(Error::internal(anyhow::anyhow!(
                    "invalid request data type"
                ))));
            };

            // Find the file path from the request.
            let file_path = match &data.path_params {
                Some(params) => params
                    .values()
                    .next()
                    .and_then(|v| v.as_str())
                    .map(|s| format!("/{}", s))
                    .unwrap_or("/".to_string()),
                None => "/".to_string(),
            };

            let httpreq = {
                let mut b = axum::http::request::Request::builder();
                {
                    // Copy headers into request.
                    let headers = b.headers_mut().unwrap();
                    for (k, v) in &data.req_headers {
                        headers.append(k.clone(), v.clone());
                    }
                }
                match b
                    .method(data.method)
                    .uri(file_path)
                    .body(Empty::<bytes::Bytes>::new())
                {
                    Ok(req) => req,
                    Err(e) => {
                        return ResponseData::Typed(Err(Error::invalid_argument(
                            "invalid file path",
                            e,
                        )));
                    }
                }
            };

            match self.service.serve(httpreq).await {
                Ok(resp) => match resp.status() {
                    // 1xx, 2xx, 3xx are all considered successful.
                    code if code.is_informational()
                        || code.is_success()
                        || code.is_redirection() =>
                    {
                        ResponseData::Raw(resp.map(axum::body::Body::new))
                    }
                    axum::http::StatusCode::NOT_FOUND => {
                        // If we have a not found handler, use that directly.
                        if self.not_found_handler {
                            ResponseData::Raw(resp.map(axum::body::Body::new))
                        } else {
                            // Otherwise return our standard not found error.
                            ResponseData::Typed(Err(Error::not_found("file not found")))
                        }
                    }
                    axum::http::StatusCode::METHOD_NOT_ALLOWED => ResponseData::Typed(Err(Error {
                        code: super::ErrCode::InvalidArgument,
                        internal_message: None,
                        message: "method not allowed".to_string(),
                        stack: None,
                        details: None,
                    })),
                    axum::http::StatusCode::INTERNAL_SERVER_ERROR => {
                        ResponseData::Typed(Err(Error {
                            code: super::ErrCode::Internal,
                            internal_message: None,
                            message: "failed to serve static asset".to_string(),
                            stack: None,
                            details: None,
                        }))
                    }
                    code => ResponseData::Typed(Err(Error::internal(anyhow::anyhow!(
                        "failed to serve static asset: {}",
                        code,
                    )))),
                },
                Err(e) => ResponseData::Typed(Err(Error::internal(e))),
            }
        })
    }
}

trait FileServer: Sync + Send + Debug {
    fn serve(
        &self,
        req: axum::http::Request<Empty<bytes::Bytes>>,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>>;
}

type FileReq = axum::http::Request<Empty<bytes::Bytes>>;
type FileRes = axum::http::Response<tower_http::services::fs::ServeFileSystemResponseBody>;

impl<F> FileServer for ServeDir<F>
where
    F: Service<FileReq, Response = FileRes, Error = Infallible>
        + Debug
        + Clone
        + Sync
        + Send
        + 'static,
    F::Future: Send + 'static,
{
    fn serve(
        &self,
        req: axum::http::Request<Empty<bytes::Bytes>>,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>> {
        let mut this = self.clone();
        Box::pin(async move { this.try_call(req).await })
    }
}
