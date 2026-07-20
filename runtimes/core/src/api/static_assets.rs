use std::{
    convert::Infallible,
    future::Future,
    io,
    path::PathBuf,
    pin::Pin,
    sync::Arc,
    task::{Context, Poll},
    time::SystemTime,
};

use http::{HeaderName, HeaderValue, StatusCode};
use http_body_util::Empty;
use percent_encoding::{utf8_percent_encode, AsciiSet, NON_ALPHANUMERIC};
use std::fmt::Debug;
use tokio::io::{AsyncRead, AsyncSeek, ReadBuf};
use tower_http::services::fs::{
    Backend, File, Metadata, ServeDir, ServeFileSystemResponseBody, TokioBackend, TokioFile,
};
use tower_service::Service;

use crate::{
    encore::parser::meta::v1 as meta,
    model::{self, RequestData},
};

use super::{BoxedHandler, Error, HandlerCall, HandlerRequest, ResponseData};

#[derive(Clone, Debug)]
pub struct StaticAssetsHandler {
    service: Arc<dyn FileServer>,
    // Serves the configured not-found/SPA-fallback file when the main service
    // returns 404. `None` when no fallback is configured.
    fallback: Option<Arc<dyn FileServer>>,
    // The request URI used to address the fallback file within `fallback`.
    fallback_uri: Option<String>,
    not_found_status: StatusCode,
    headers: Vec<(HeaderName, HeaderValue)>,
}

impl StaticAssetsHandler {
    pub fn new(
        cfg: &meta::rpc::StaticAssets,
        build_time: Option<chrono::DateTime<chrono::Utc>>,
    ) -> Self {
        // On-disk mtimes are zeroed for reproducible container builds, which
        // makes the `Last-Modified` header and the mtime-derived `ETag` that
        // `ServeDir` generates useless for cache validation (they'd be identical
        // across deploys). When a build time is available we serve files through
        // a backend that reports it as every file's modification time, giving
        // each deploy distinct validators. Without one (e.g. local dev) the
        // real on-disk mtimes are meaningful, so we use the default backend.
        let backend = build_time.map(|bt| BuildTimeBackend::new(bt.into()));

        let service: Arc<dyn FileServer> = match &backend {
            Some(backend) => Arc::new(ServeDir::with_backend(
                PathBuf::from(&cfg.dir_rel_path),
                backend.clone(),
            )),
            None => Arc::new(ServeDir::new(PathBuf::from(&cfg.dir_rel_path))),
        };

        let not_found_status = cfg
            .not_found_status
            .and_then(|c| StatusCode::from_u16(c as u16).ok())
            .unwrap_or(StatusCode::NOT_FOUND);

        // Serve the fallback file through the same backend as the main directory
        // so it gets identical, build-time-based validators. `ServeFile` cannot
        // take a custom `Backend`, so we root a `ServeDir` at the current
        // directory and address the file by its (constant, non-user-controlled)
        // path — matching how `ServeFile::new` resolved it relative to the cwd.
        let (fallback, fallback_uri): (Option<Arc<dyn FileServer>>, Option<String>) =
            match cfg.not_found_rel_path.as_ref() {
                Some(path) => {
                    let fb: Arc<dyn FileServer> = match &backend {
                        Some(backend) => Arc::new(ServeDir::with_backend(".", backend.clone())),
                        None => Arc::new(ServeDir::new(".")),
                    };
                    (Some(fb), Some(fallback_uri_for(path)))
                }
                None => (None, None),
            };

        let headers: Vec<(HeaderName, HeaderValue)> = cfg
            .headers
            .iter()
            .flat_map(|(key, header_values)| {
                HeaderName::from_bytes(key.as_bytes())
                    .inspect_err(|e| {
                        log::error!("skipping header: '{}' - {}", key, e);
                    })
                    .ok()
                    .map(|header_name| {
                        header_values.values.iter().filter_map(move |value| {
                            HeaderValue::from_bytes(value.as_bytes())
                                .inspect_err(|e| {
                                    log::error!("skipping header '{}': '{}' - {}", key, value, e);
                                })
                                .ok()
                                .map(|header_value| (header_name.clone(), header_value))
                        })
                    })
                    .into_iter()
                    .flatten()
            })
            .collect();

        StaticAssetsHandler {
            service,
            fallback,
            fallback_uri,
            not_found_status,
            headers,
        }
    }

    /// Builds the request passed to a [`FileServer`]. When `forward_conditionals`
    /// is set, conditional and range headers are forwarded so `ServeDir` can
    /// natively evaluate them against the ETag/Last-Modified it derives from the
    /// build time; otherwise they are dropped so the full body is always served.
    fn build_request(
        &self,
        data: &model::RPCRequestData,
        uri: &str,
        forward_conditionals: bool,
    ) -> Result<FileReq, Error> {
        let mut b = axum::http::request::Request::builder();
        {
            let headers = b.headers_mut().unwrap();
            for (k, v) in &data.req_headers {
                if !forward_conditionals && is_conditional_header(k) {
                    continue;
                }
                headers.append(k.clone(), v.clone());
            }
        }
        b.method(data.method)
            .uri(uri)
            .body(Empty::<bytes::Bytes>::new())
            .map_err(|e| Error::invalid_argument("invalid file path", e))
    }

    /// Appends the configured extra headers and turns a file-server response
    /// into a raw handler response.
    fn finish(&self, mut resp: FileRes) -> ResponseData {
        let resp_headers = resp.headers_mut();
        for (name, value) in &self.headers {
            resp_headers.append(name.clone(), value.clone());
        }
        ResponseData::Raw(resp.map(axum::body::Body::new))
    }

    /// Serves the configured not-found/SPA-fallback file. Called when the main
    /// service reports a 404.
    async fn serve_fallback(&self, data: &model::RPCRequestData) -> ResponseData {
        let (Some(fallback), Some(uri)) = (&self.fallback, &self.fallback_uri) else {
            return ResponseData::Typed(Err(Error::not_found("file not found")));
        };

        // For SPA fallbacks (configured status 200) forward conditional headers
        // so an unchanged build revalidates to `304 Not Modified`. For a custom
        // error page we want the full body every time under the configured
        // status, so we don't forward conditionals (which could yield a bodyless
        // 304 that we'd then relabel).
        let spa = self.not_found_status == StatusCode::OK;
        let req = match self.build_request(data, uri, spa) {
            Ok(r) => r,
            Err(e) => return ResponseData::Typed(Err(e)),
        };

        match fallback.serve(req).await {
            Ok(mut resp) => {
                let servable =
                    resp.status().is_success() || resp.status() == StatusCode::NOT_MODIFIED;
                if !servable {
                    // The configured fallback file is itself missing/unreadable.
                    return ResponseData::Typed(Err(Error::not_found("file not found")));
                }
                if !spa {
                    *resp.status_mut() = self.not_found_status;
                }
                self.finish(resp)
            }
            Err(e) => ResponseData::Typed(Err(Error::internal(e))),
        }
    }
}

impl BoxedHandler for StaticAssetsHandler {
    fn call(self: Arc<Self>, req: HandlerRequest) -> HandlerCall {
        HandlerCall::inline(Box::pin(async move {
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
                    .map(|s| format!("/{s}"))
                    .unwrap_or("/".to_string()),
                None => "/".to_string(),
            };

            // Forward all headers (including conditional ones) so `ServeDir`
            // natively evaluates `If-None-Match` / `If-Modified-Since` against
            // the build-time-based ETag and Last-Modified it generates.
            let httpreq = match self.build_request(data, &file_path, true) {
                Ok(r) => r,
                Err(e) => return ResponseData::Typed(Err(e)),
            };

            let resp = match self.service.serve(httpreq).await {
                Ok(resp) => resp,
                Err(e) => return ResponseData::Typed(Err(Error::internal(e))),
            };

            match resp.status() {
                // 1xx, 2xx, 3xx (including 304 Not Modified) are served as-is.
                code if code.is_informational() || code.is_success() || code.is_redirection() => {
                    self.finish(resp)
                }
                // 412 from a failed `If-Match` / `If-Unmodified-Since` is a valid
                // conditional-request outcome; pass it through unchanged.
                StatusCode::PRECONDITION_FAILED => self.finish(resp),
                StatusCode::NOT_FOUND => self.serve_fallback(data).await,
                StatusCode::METHOD_NOT_ALLOWED => ResponseData::Typed(Err(Error {
                    code: super::ErrCode::InvalidArgument,
                    internal_message: None,
                    message: "method not allowed".to_string(),
                    stack: None,
                    details: None,
                })),
                StatusCode::INTERNAL_SERVER_ERROR => ResponseData::Typed(Err(Error {
                    code: super::ErrCode::Internal,
                    internal_message: None,
                    message: "failed to serve static asset".to_string(),
                    stack: None,
                    details: None,
                })),
                code => ResponseData::Typed(Err(Error::internal(anyhow::anyhow!(
                    "failed to serve static asset: {}",
                    code,
                )))),
            }
        }))
    }
}

/// Set of characters percent-encoded when turning a filesystem path into a
/// request URI: everything except the RFC 3986 unreserved characters. This is
/// the inverse of the percent-decoding `ServeDir` applies to the request path,
/// so a not-found path containing spaces or other URI-unsafe bytes still
/// resolves to the same file.
const PATH_SEGMENT: &AsciiSet = &NON_ALPHANUMERIC
    .remove(b'-')
    .remove(b'_')
    .remove(b'.')
    .remove(b'~');

/// Builds the request URI addressing the not-found file within a `ServeDir`
/// rooted at the current directory. Each path segment is percent-encoded so it
/// round-trips through `ServeDir`'s percent-decoding unchanged.
fn fallback_uri_for(path: &str) -> String {
    let mut uri = String::new();
    for segment in path.trim_start_matches('/').split('/') {
        if segment.is_empty() {
            continue;
        }
        uri.push('/');
        uri.extend(utf8_percent_encode(segment, PATH_SEGMENT));
    }
    if uri.is_empty() {
        uri.push('/');
    }
    uri
}

/// Returns whether `name` is a conditional or range request header whose
/// evaluation we may want to leave to `ServeDir` or suppress entirely.
fn is_conditional_header(name: &HeaderName) -> bool {
    *name == http::header::IF_NONE_MATCH
        || *name == http::header::IF_MODIFIED_SINCE
        || *name == http::header::IF_MATCH
        || *name == http::header::IF_UNMODIFIED_SINCE
        || *name == http::header::RANGE
}

trait FileServer: Sync + Send + Debug {
    fn serve(
        &self,
        req: FileReq,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>>;
}

type FileReq = axum::http::Request<Empty<bytes::Bytes>>;
type FileRes = axum::http::Response<ServeFileSystemResponseBody>;

impl<F, B> FileServer for ServeDir<F, B>
where
    B: Backend + Debug,
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
        req: FileReq,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>> {
        let mut this = self.clone();
        Box::pin(async move { this.try_call(req).await })
    }
}

/// A [`Backend`] that serves files from the local filesystem via [`TokioBackend`]
/// but reports a fixed build time as every file's modification time, so the
/// `Last-Modified` header and the mtime-derived `ETag` produced by `ServeDir`
/// change with each deploy instead of reflecting the zeroed on-disk mtime.
#[derive(Clone, Debug)]
struct BuildTimeBackend {
    inner: TokioBackend,
    build_time: SystemTime,
}

impl BuildTimeBackend {
    fn new(build_time: SystemTime) -> Self {
        Self {
            inner: TokioBackend,
            build_time,
        }
    }
}

impl Backend for BuildTimeBackend {
    type File = BuildTimeFile;
    type Metadata = BuildTimeMetadata;
    type OpenFuture = Pin<Box<dyn Future<Output = io::Result<BuildTimeFile>> + Send>>;
    type MetadataFuture = Pin<Box<dyn Future<Output = io::Result<BuildTimeMetadata>> + Send>>;

    fn open(&self, path: PathBuf) -> Self::OpenFuture {
        let inner = self.inner.clone();
        let build_time = self.build_time;
        Box::pin(async move {
            let file = inner.open(path).await?;
            Ok(BuildTimeFile {
                inner: file,
                build_time,
            })
        })
    }

    fn metadata(&self, path: PathBuf) -> Self::MetadataFuture {
        let inner = self.inner.clone();
        let build_time = self.build_time;
        Box::pin(async move {
            let meta = inner.metadata(path).await?;
            Ok(BuildTimeMetadata {
                inner: meta,
                build_time,
            })
        })
    }
}

/// [`File`] wrapper that delegates all I/O to a [`TokioFile`] but reports the
/// build time as its modification time.
#[derive(Debug)]
struct BuildTimeFile {
    inner: TokioFile,
    build_time: SystemTime,
}

impl AsyncRead for BuildTimeFile {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<io::Result<()>> {
        Pin::new(&mut self.inner).poll_read(cx, buf)
    }
}

impl AsyncSeek for BuildTimeFile {
    fn start_seek(mut self: Pin<&mut Self>, position: io::SeekFrom) -> io::Result<()> {
        Pin::new(&mut self.inner).start_seek(position)
    }

    fn poll_complete(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<u64>> {
        Pin::new(&mut self.inner).poll_complete(cx)
    }
}

impl File for BuildTimeFile {
    type Metadata = BuildTimeMetadata;
    type MetadataFuture<'a> =
        Pin<Box<dyn Future<Output = io::Result<BuildTimeMetadata>> + Send + 'a>>;

    fn metadata(&self) -> Self::MetadataFuture<'_> {
        let build_time = self.build_time;
        Box::pin(async move {
            let meta = self.inner.metadata().await?;
            Ok(BuildTimeMetadata {
                inner: meta,
                build_time,
            })
        })
    }
}

/// [`Metadata`] wrapper that overrides the modification time with the build time
/// while delegating everything else to the real filesystem metadata.
#[derive(Debug)]
struct BuildTimeMetadata {
    inner: std::fs::Metadata,
    build_time: SystemTime,
}

impl Metadata for BuildTimeMetadata {
    fn is_dir(&self) -> bool {
        self.inner.is_dir()
    }

    fn modified(&self) -> io::Result<SystemTime> {
        Ok(self.build_time)
    }

    fn len(&self) -> u64 {
        self.inner.len()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use http::header;
    use std::path::Path;
    use std::time::Duration;
    use tower::ServiceExt;

    fn build_time(secs: u64) -> SystemTime {
        SystemTime::UNIX_EPOCH + Duration::from_secs(secs)
    }

    /// Serves `uri` from `dir` through a `BuildTimeBackend` pinned to `bt`,
    /// optionally with a single conditional request header.
    async fn get(
        dir: &Path,
        bt: SystemTime,
        uri: &str,
        conditional: Option<(HeaderName, HeaderValue)>,
    ) -> FileRes {
        let mut builder = axum::http::Request::builder().method("GET").uri(uri);
        if let Some((k, v)) = conditional {
            builder = builder.header(k, v);
        }
        let req = builder.body(Empty::<bytes::Bytes>::new()).unwrap();
        let svc = ServeDir::with_backend(dir, BuildTimeBackend::new(bt));
        svc.oneshot(req).await.unwrap()
    }

    #[tokio::test]
    async fn build_time_overrides_disk_mtime() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("app.js"), b"console.log(1)").unwrap();

        // Build time deliberately different from the file's real (now) mtime.
        let bt = build_time(1_000_000_000); // 2001-09-09
        let resp = get(dir.path(), bt, "/app.js", None).await;
        assert_eq!(resp.status(), StatusCode::OK);

        let last_modified = resp.headers().get(header::LAST_MODIFIED).unwrap();
        assert_eq!(
            last_modified.to_str().unwrap(),
            httpdate::fmt_http_date(bt),
            "Last-Modified must reflect the build time, not the on-disk mtime"
        );
        assert!(
            resp.headers().get(header::ETAG).is_some(),
            "an ETag should be generated"
        );
    }

    #[tokio::test]
    async fn conditional_requests_yield_304() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("app.js"), b"console.log(1)").unwrap();
        let bt = build_time(1_000_000_000);

        let resp = get(dir.path(), bt, "/app.js", None).await;
        let etag = resp.headers().get(header::ETAG).unwrap().clone();

        // A matching ETag revalidates to 304.
        let resp = get(
            dir.path(),
            bt,
            "/app.js",
            Some((header::IF_NONE_MATCH, etag)),
        )
        .await;
        assert_eq!(resp.status(), StatusCode::NOT_MODIFIED);

        // If-Modified-Since at the build time also revalidates to 304.
        let ims = HeaderValue::from_str(&httpdate::fmt_http_date(bt)).unwrap();
        let resp = get(
            dir.path(),
            bt,
            "/app.js",
            Some((header::IF_MODIFIED_SINCE, ims)),
        )
        .await;
        assert_eq!(resp.status(), StatusCode::NOT_MODIFIED);
    }

    #[tokio::test]
    async fn new_deploy_busts_client_cache() {
        // Same bytes on disk, two different build times (i.e. two deploys). The
        // validators must differ so a client holding the old one refetches
        // instead of being served a stale 304 — the crux of the reported bug.
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("app.js"), b"console.log(1)").unwrap();

        let old_bt = build_time(1_000_000_000);
        let new_bt = build_time(2_000_000_000);

        let old = get(dir.path(), old_bt, "/app.js", None).await;
        let old_etag = old.headers().get(header::ETAG).unwrap().clone();

        // A client that cached the old deploy revalidates against the new one.
        let resp = get(
            dir.path(),
            new_bt,
            "/app.js",
            Some((header::IF_NONE_MATCH, old_etag.clone())),
        )
        .await;
        assert_eq!(
            resp.status(),
            StatusCode::OK,
            "a stale validator must not produce a 304 after a new deploy"
        );
        let new_etag = resp.headers().get(header::ETAG).unwrap();
        assert_ne!(&old_etag, new_etag, "ETag must change across deploys");
    }

    #[test]
    fn fallback_uri_percent_encodes_segments() {
        assert_eq!(fallback_uri_for("dist/index.html"), "/dist/index.html");
        assert_eq!(fallback_uri_for("/dist/index.html"), "/dist/index.html");
        // URI-unsafe characters are encoded so they survive ServeDir's decode.
        assert_eq!(fallback_uri_for("my dir/404.html"), "/my%20dir/404.html");
        assert_eq!(fallback_uri_for("a%b/index.html"), "/a%25b/index.html");
    }

    #[tokio::test]
    async fn default_backend_uses_disk_mtime() {
        // Sanity check: without the build-time backend, Last-Modified is the
        // on-disk mtime — the very thing that's unreliable in reproducible
        // container builds and motivates the override.
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("app.js"), b"console.log(1)").unwrap();

        let req = axum::http::Request::builder()
            .method("GET")
            .uri("/app.js")
            .body(Empty::<bytes::Bytes>::new())
            .unwrap();
        let resp = ServeDir::new(dir.path()).oneshot(req).await.unwrap();
        assert_eq!(resp.status(), StatusCode::OK);

        let bt = build_time(1_000_000_000);
        let last_modified = resp.headers().get(header::LAST_MODIFIED).unwrap();
        assert_ne!(last_modified.to_str().unwrap(), httpdate::fmt_http_date(bt));
    }
}
