use std::{
    future::Future,
    path::PathBuf,
    pin::Pin,
    sync::Arc,
    time::{Duration, SystemTime},
};

use http::{HeaderMap, HeaderName, HeaderValue, StatusCode};
use http_body_util::Empty;
use httpdate::HttpDate;
use std::fmt::Debug;
use std::io;
use tower_http::services::{fs::ServeDir, ServeFile};

use crate::{
    encore::parser::meta::v1 as meta,
    model::{self, RequestData},
};

use super::{BoxedHandler, Error, HandlerCall, HandlerRequest, ResponseData};

/// Modification times at or below this instant are treated as normalized rather
/// than real: image builds stamp every file with a fixed epoch so unchanged
/// layers keep identical digests, leaving the mtime saying nothing about the
/// file. Serving it as `Last-Modified` is worse than serving none, since caches
/// derive heuristic freshness from its age.
const NORMALIZED_MTIME_CUTOFF_SECS: u64 = 315_532_800; // 1980-01-01

fn unix_time(secs: u64) -> SystemTime {
    SystemTime::UNIX_EPOCH + Duration::from_secs(secs)
}

#[derive(Clone, Debug)]
pub struct StaticAssetsHandler {
    service: Arc<dyn FileServer>,
    // Serves the configured not-found/SPA-fallback file when the main service
    // returns 404. `None` when no fallback is configured.
    fallback: Option<Arc<dyn FileServer>>,
    not_found_status: StatusCode,
    headers: Vec<(HeaderName, HeaderValue)>,
    // Served for assets whose mtime was normalized away. `None` disables
    // entity-tag validation entirely.
    etag: Option<HeaderValue>,
    // If set, `ServeDir`'s mtime-derived `Last-Modified` is dropped in favour of
    // the app's.
    app_last_modified: bool,
}

impl StaticAssetsHandler {
    pub fn new(cfg: &meta::rpc::StaticAssets, etag: Option<&str>) -> Self {
        let service: Arc<dyn FileServer> =
            Arc::new(ServeDir::new(PathBuf::from(&cfg.dir_rel_path)));

        let not_found_status = cfg
            .not_found_status
            .and_then(|c| StatusCode::from_u16(c as u16).ok())
            .unwrap_or(StatusCode::NOT_FOUND);

        let fallback: Option<Arc<dyn FileServer>> = cfg
            .not_found_rel_path
            .as_ref()
            .map(|p| Arc::new(ServeFile::new(PathBuf::from(p))) as Arc<dyn FileServer>);

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

        // `ETag` and `Last-Modified` are singleton fields, so where the app
        // configures one we don't add a second. Theirs goes out as-is, but we
        // never revalidate against a validator we don't control.
        let configures = |name: &HeaderName| headers.iter().any(|(n, _)| n == name);
        let app_last_modified = configures(&http::header::LAST_MODIFIED);
        // A VCS revision, so quoting it always yields a valid header value.
        let etag = etag
            .filter(|_| !configures(&http::header::ETAG))
            .and_then(|etag| HeaderValue::from_str(&format!("\"{etag}\"")).ok());

        StaticAssetsHandler {
            service,
            fallback,
            not_found_status,
            headers,
            etag,
            app_last_modified,
        }
    }

    /// Builds the request passed to a [`FileServer`].
    fn build_request(
        &self,
        data: &model::RPCRequestData,
        uri: &str,
        allow_range: bool,
    ) -> Result<FileReq, Error> {
        let mut b = axum::http::request::Request::builder();
        {
            if let Some(headers) = b.headers_mut() {
                for (k, v) in self.forwarded_headers(&data.req_headers, allow_range) {
                    headers.append(k.clone(), v.clone());
                }
            };
        }
        b.method(data.method)
            .uri(uri)
            .body(Empty::<bytes::Bytes>::new())
            .map_err(|e| Error::invalid_argument("invalid file path", e))
    }

    /// The subset of the request's headers forwarded to the file server.
    ///
    /// Only `If-Modified-Since` is withheld: `ServeDir` would evaluate it
    /// against a normalized epoch mtime and answer every request with a bogus
    /// `304`, so [`Self::finish_validated`] answers it instead. The other
    /// preconditions `ServeDir` either ignores or judges correctly.
    fn forwarded_headers<'h>(
        &self,
        req_headers: &'h HeaderMap,
        allow_range: bool,
    ) -> impl Iterator<Item = (&'h HeaderName, &'h HeaderValue)> {
        let serve_range = allow_range && self.if_range_holds(req_headers);
        req_headers.iter().filter(move |(k, _)| {
            **k != http::header::IF_MODIFIED_SINCE && (serve_range || **k != http::header::RANGE)
        })
    }

    /// Evaluates `If-Range` (RFC 9110 §13.1.5), which `ServeDir` ignores
    /// entirely: without this, a client resuming a download across a deploy
    /// splices bytes from two different representations together.
    ///
    /// Only the entity-tag form can be judged. The date form names the
    /// `Last-Modified`, unknown until the file is opened — by which point the
    /// `Range` is already applied — so it counts as false and the resume
    /// degrades to a full download.
    fn if_range_holds(&self, req_headers: &HeaderMap) -> bool {
        match req_headers.get(http::header::IF_RANGE) {
            None => true,
            // Strong comparison, as §13.1.5 requires.
            Some(v) => self
                .etag
                .as_ref()
                .is_some_and(|etag| etag_list_matches(v, etag, false)),
        }
    }

    /// Appends the configured extra headers and turns a file-server response
    /// into a raw handler response.
    fn finish(&self, mut resp: axum::http::Response<axum::body::Body>) -> ResponseData {
        let resp_headers = resp.headers_mut();
        for (name, value) in &self.headers {
            resp_headers.append(name.clone(), value.clone());
        }
        ResponseData::Raw(resp)
    }

    /// Rewrites a successful response's cache validators and answers a
    /// conditional request against them with a `304` where one is called for.
    fn finish_validated(
        &self,
        req_headers: &HeaderMap,
        resp: FileRes,
    ) -> axum::http::Response<axum::body::Body> {
        let mut resp = resp.map(axum::body::Body::new);

        // Keep `ServeDir`'s mtime-derived `Last-Modified` when it says something
        // true about the file, otherwise validate with our own entity tag. Only
        // ever one of the two, so there is no precedence to resolve.
        let last_modified = resp
            .headers()
            .get(http::header::LAST_MODIFIED)
            .and_then(|v| v.to_str().ok())
            .and_then(|v| httpdate::parse_http_date(v).ok())
            .filter(|t| *t > unix_time(NORMALIZED_MTIME_CUTOFF_SECS))
            .filter(|_| !self.app_last_modified);

        let etag = match last_modified {
            Some(_) => None,
            None => {
                resp.headers_mut().remove(http::header::LAST_MODIFIED);
                self.etag.clone()
            }
        };
        if let Some(etag) = &etag {
            resp.headers_mut().insert(http::header::ETAG, etag.clone());
        }

        if is_not_modified(req_headers, etag.as_ref(), last_modified) {
            return into_bodyless(resp, StatusCode::NOT_MODIFIED);
        }
        resp
    }

    /// Serves the configured not-found/SPA-fallback file. Called when the main
    /// service reports a 404.
    async fn serve_fallback(&self, data: &model::RPCRequestData) -> ResponseData {
        let Some(fallback) = &self.fallback else {
            return ResponseData::Typed(Err(Error::not_found("file not found")));
        };

        // A fallback served as 200 OK is a genuine, cacheable representation
        // (SPA routing), so it gets validators like any other asset. A custom
        // error page instead gets the complete body under the configured status
        // every time. `ServeFile` ignores the request URI, so ours is arbitrary.
        let spa = self.not_found_status == StatusCode::OK;
        let req = match self.build_request(data, "/", spa) {
            Ok(r) => r,
            Err(e) => return ResponseData::Typed(Err(e)),
        };

        match fallback.serve(req).await {
            Ok(resp) if resp.status().is_success() => {
                if spa {
                    let resp = self.finish_validated(&data.req_headers, resp);
                    self.finish(resp)
                } else {
                    let mut resp = resp.map(axum::body::Body::new);
                    *resp.status_mut() = self.not_found_status;
                    resp.headers_mut().remove(http::header::LAST_MODIFIED);
                    self.finish(resp)
                }
            }
            // The configured fallback file is itself missing or unreadable.
            Ok(_) => ResponseData::Typed(Err(Error::not_found("file not found"))),
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

            let httpreq = match self.build_request(data, &file_path, true) {
                Ok(r) => r,
                Err(e) => return ResponseData::Typed(Err(e)),
            };

            let resp = match self.service.serve(httpreq).await {
                Ok(resp) => resp,
                Err(e) => return ResponseData::Typed(Err(Error::internal(e))),
            };

            match resp.status() {
                code if code.is_success() => {
                    let resp = self.finish_validated(&data.req_headers, resp);
                    self.finish(resp)
                }
                // 1xx and 3xx (directory redirects) pass through, as do 412 and
                // 416 — correct answers from `ServeDir` rather than faults. The
                // 416 is built like a 200, so it carries an mtime to strip.
                code if code.is_informational()
                    || code.is_redirection()
                    || code == StatusCode::PRECONDITION_FAILED
                    || code == StatusCode::RANGE_NOT_SATISFIABLE =>
                {
                    let mut resp = resp.map(axum::body::Body::new);
                    resp.headers_mut().remove(http::header::LAST_MODIFIED);
                    self.finish(resp)
                }
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

/// Strips a response of everything describing content it will no longer carry,
/// leaving the validators and metadata a `304` is allowed to keep
/// (RFC 9110 §15.4.5).
fn into_bodyless(
    mut resp: axum::http::Response<axum::body::Body>,
    status: StatusCode,
) -> axum::http::Response<axum::body::Body> {
    *resp.status_mut() = status;
    for name in [
        http::header::CONTENT_LENGTH,
        http::header::CONTENT_TYPE,
        http::header::CONTENT_RANGE,
    ] {
        resp.headers_mut().remove(name);
    }
    resp.map(|_| axum::body::Body::empty())
}

/// Whether the request's conditional-GET header matches the validator we serve,
/// making a `304` the correct answer.
///
/// A precondition naming the validator we *didn't* issue can't be evaluated, so
/// it is ignored rather than guessed at, failing towards the full body.
fn is_not_modified(
    req: &HeaderMap,
    etag: Option<&HeaderValue>,
    last_modified: Option<SystemTime>,
) -> bool {
    if let Some(etag) = etag {
        // Weak comparison, as a conditional GET requires. The field may arrive
        // split across several lines.
        return req
            .get_all(http::header::IF_NONE_MATCH)
            .iter()
            .any(|v| etag_list_matches(v, etag, true));
    }
    // `HttpDate` truncates both sides to the second granularity we served at.
    match (last_modified, req.get(http::header::IF_MODIFIED_SINCE)) {
        (Some(modified), Some(since)) => parse_http_date(since)
            .is_some_and(|since| HttpDate::from(modified) <= HttpDate::from(since)),
        _ => false,
    }
}

fn parse_http_date(value: &HeaderValue) -> Option<SystemTime> {
    httpdate::parse_http_date(value.to_str().ok()?).ok()
}

/// Returns whether the comma-separated entity-tag list in `header` contains
/// `etag`, or is the wildcard `*`. Since we only ever generate strong tags,
/// weak comparison amounts to accepting a `W/` prefix on the client's copy.
fn etag_list_matches(header: &HeaderValue, etag: &HeaderValue, weak: bool) -> bool {
    let Ok(list) = header.to_str() else {
        return false;
    };
    let list = list.trim();
    if list == "*" {
        return true;
    }
    list.split(',').any(|candidate| {
        let candidate = candidate.trim().as_bytes();
        match candidate.strip_prefix(b"W/") {
            Some(opaque) => weak && opaque == etag.as_bytes(),
            None => candidate == etag.as_bytes(),
        }
    })
}

trait FileServer: Sync + Send + Debug {
    fn serve(
        &self,
        req: FileReq,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>>;
}

type FileReq = axum::http::Request<Empty<bytes::Bytes>>;
type FileRes = axum::http::Response<tower_http::services::fs::ServeFileSystemResponseBody>;

impl FileServer for ServeDir {
    fn serve(
        &self,
        req: FileReq,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>> {
        let mut this = self.clone();
        Box::pin(async move { this.try_call(req).await })
    }
}

impl FileServer for ServeFile {
    fn serve(
        &self,
        req: FileReq,
    ) -> Pin<Box<dyn Future<Output = Result<FileRes, io::Error>> + Send + 'static>> {
        let mut this = self.clone();
        Box::pin(async move { this.try_call(req).await })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::Response;
    use http::header;
    use std::path::Path;

    fn req(headers: &[(HeaderName, &str)]) -> HeaderMap {
        let mut map = HeaderMap::new();
        for (name, value) in headers {
            map.append(name.clone(), HeaderValue::from_str(value).unwrap());
        }
        map
    }

    #[test]
    fn if_none_match_accepts_every_legal_form() {
        let tag = HeaderValue::from_static("\"abc\"");
        for value in ["\"abc\"", "*", "W/\"abc\"", "\"xyz\", \"abc\"", " \"abc\" "] {
            assert!(
                is_not_modified(&req(&[(header::IF_NONE_MATCH, value)]), Some(&tag), None),
                "If-None-Match: {value} should revalidate"
            );
        }
        // A tag from an earlier revision must not.
        assert!(!is_not_modified(
            &req(&[(header::IF_NONE_MATCH, "\"old\"")]),
            Some(&tag),
            None
        ));
        // The field may arrive split across several lines.
        let split = req(&[
            (header::IF_NONE_MATCH, "\"other\""),
            (header::IF_NONE_MATCH, "\"abc\""),
        ]);
        assert!(is_not_modified(&split, Some(&tag), None));
    }

    #[test]
    fn if_modified_since_is_answered_against_a_real_mtime() {
        let modified = unix_time(1_700_000_000);
        let at = httpdate::fmt_http_date(modified);
        let before = httpdate::fmt_http_date(unix_time(1_600_000_000));

        let case = |v: &str, m| is_not_modified(&req(&[(header::IF_MODIFIED_SINCE, v)]), None, m);
        assert!(case(&at, Some(modified)));
        assert!(!case(&before, Some(modified)));
        // Without a Last-Modified there is nothing to compare against.
        assert!(!case(&at, None));
        // A sub-second mtime is newer than the date we served for it, which must
        // not read as "modified since".
        let sub = modified + Duration::from_millis(500);
        assert!(case(&httpdate::fmt_http_date(sub), Some(sub)));
    }

    #[test]
    fn if_range_gates_partial_responses() {
        let dir = tempfile::tempdir().unwrap();
        let h = StaticAssetsHandler::new(&config(dir.path()), Some("rev2"));

        // Whether the `Range` survives into the request `ServeDir` sees, which
        // is the only place `If-Range` gets a say.
        let ranged = |if_range: Option<&str>| {
            let mut headers = vec![(header::RANGE, "bytes=0-3")];
            headers.extend(if_range.map(|v| (header::IF_RANGE, v)));
            h.forwarded_headers(&req(&headers), true)
                .any(|(k, _)| *k == header::RANGE)
        };

        assert!(ranged(None));
        assert!(ranged(Some("\"rev2\"")));
        // A tag from the previous deploy must not splice a partial response onto
        // bytes the client fetched from a different revision.
        assert!(!ranged(Some("\"rev1\"")));
        // Strong comparison, and a date we can't check in time.
        assert!(!ranged(Some("W/\"rev2\"")));
        assert!(!ranged(Some(&httpdate::fmt_http_date(unix_time(
            1_700_000_000
        )))));

        // Serving no ETag, an entity-tag If-Range can't have come from us.
        let h = StaticAssetsHandler::new(&config(dir.path()), None);
        assert!(!h.if_range_holds(&req(&[(header::IF_RANGE, "\"rev1\"")])));
    }

    #[test]
    fn if_modified_since_never_reaches_the_file_server() {
        let dir = tempfile::tempdir().unwrap();
        let h = StaticAssetsHandler::new(&config(dir.path()), Some("rev1"));

        let headers = req(&[
            (header::IF_MODIFIED_SINCE, "Wed, 21 Oct 2015 07:28:00 GMT"),
            (header::IF_NONE_MATCH, "\"rev1\""),
            (header::IF_UNMODIFIED_SINCE, "Wed, 21 Oct 2015 07:28:00 GMT"),
        ]);
        let forwarded: Vec<_> = h
            .forwarded_headers(&headers, true)
            .map(|(k, _)| k)
            .collect();
        assert_eq!(
            forwarded,
            vec![&header::IF_NONE_MATCH, &header::IF_UNMODIFIED_SINCE],
            "only If-Modified-Since is withheld"
        );
    }

    // -- end-to-end through the handler -----------------------------------

    fn config(dir: &Path) -> meta::rpc::StaticAssets {
        meta::rpc::StaticAssets {
            dir_rel_path: dir.to_str().unwrap().to_string(),
            not_found_rel_path: None,
            not_found_status: None,
            headers: Default::default(),
        }
    }

    /// Writes `name` into `dir`, stamping it with the epoch mtime an image
    /// build leaves behind when `normalized` is set.
    fn write_asset(dir: &Path, name: &str, normalized: bool) {
        let path = dir.join(name);
        std::fs::write(&path, b"console.log(1)").unwrap();
        if normalized {
            let f = std::fs::File::options().write(true).open(&path).unwrap();
            f.set_times(std::fs::FileTimes::new().set_modified(SystemTime::UNIX_EPOCH))
                .unwrap();
        }
    }

    /// Runs a request through `ServeDir` and the handler's response pipeline,
    /// the same way `call()` does for a successful lookup.
    async fn get(
        h: &StaticAssetsHandler,
        dir: &Path,
        headers: &[(HeaderName, &str)],
    ) -> Response<Body> {
        let file_req = axum::http::Request::builder()
            .method("GET")
            .uri("/app.js")
            .body(Empty::<bytes::Bytes>::new())
            .unwrap();
        let resp = ServeDir::new(dir).serve(file_req).await.unwrap();
        assert_eq!(resp.status(), StatusCode::OK, "fixture should be servable");

        let resp = h.finish_validated(&req(headers), resp);
        match h.finish(resp) {
            ResponseData::Raw(resp) => resp,
            _ => panic!("expected a raw response"),
        }
    }

    #[tokio::test]
    async fn normalized_mtime_is_replaced_by_an_etag() {
        let dir = tempfile::tempdir().unwrap();
        write_asset(dir.path(), "app.js", true);
        let h = StaticAssetsHandler::new(&config(dir.path()), Some("rev1"));

        let resp = get(&h, dir.path(), &[]).await;
        assert_eq!(resp.status(), StatusCode::OK);
        assert_eq!(
            resp.headers().get(header::LAST_MODIFIED),
            None,
            "a normalized mtime must not be advertised as a modification date"
        );
        assert_eq!(resp.headers().get(header::ETAG).unwrap(), "\"rev1\"");
    }

    #[tokio::test]
    async fn real_mtime_is_left_alone_and_still_revalidates() {
        // Local development: the mtime is meaningful, so it stays and no entity
        // tag is invented — otherwise editing a file mid-session would keep
        // revalidating to a stale 304.
        let dir = tempfile::tempdir().unwrap();
        write_asset(dir.path(), "app.js", false);
        let h = StaticAssetsHandler::new(&config(dir.path()), Some("rev1"));

        let resp = get(&h, dir.path(), &[]).await;
        assert_eq!(resp.headers().get(header::ETAG), None);
        let served = resp.headers().get(header::LAST_MODIFIED).unwrap().clone();

        let resp = get(
            &h,
            dir.path(),
            &[(header::IF_MODIFIED_SINCE, served.to_str().unwrap())],
        )
        .await;
        assert_eq!(resp.status(), StatusCode::NOT_MODIFIED);
    }

    #[tokio::test]
    async fn current_etag_revalidates_to_a_bodyless_304() {
        let dir = tempfile::tempdir().unwrap();
        write_asset(dir.path(), "app.js", true);
        let h = StaticAssetsHandler::new(&config(dir.path()), Some("rev1"));

        let resp = get(&h, dir.path(), &[(header::IF_NONE_MATCH, "\"rev1\"")]).await;
        assert_eq!(resp.status(), StatusCode::NOT_MODIFIED);
        assert_eq!(resp.headers().get(header::ETAG).unwrap(), "\"rev1\"");
        assert_eq!(resp.headers().get(header::CONTENT_LENGTH), None);
        assert_eq!(resp.headers().get(header::CONTENT_TYPE), None);
    }

    #[tokio::test]
    async fn a_new_revision_busts_the_client_cache() {
        // Identical bytes, two revisions: the stale 304 this all exists to stop.
        let dir = tempfile::tempdir().unwrap();
        write_asset(dir.path(), "app.js", true);
        let h = StaticAssetsHandler::new(&config(dir.path()), Some("rev2"));

        let resp = get(&h, dir.path(), &[(header::IF_NONE_MATCH, "\"rev1\"")]).await;
        assert_eq!(resp.status(), StatusCode::OK);
        assert_eq!(resp.headers().get(header::ETAG).unwrap(), "\"rev2\"");
    }

    #[tokio::test]
    async fn a_dirty_build_serves_no_validators_at_all() {
        // No ETag (an unclean tree), so nothing to revalidate against.
        let dir = tempfile::tempdir().unwrap();
        write_asset(dir.path(), "app.js", true);
        let h = StaticAssetsHandler::new(&config(dir.path()), None);

        let resp = get(&h, dir.path(), &[(header::IF_NONE_MATCH, "\"rev1\"")]).await;
        assert_eq!(resp.status(), StatusCode::OK);
        assert_eq!(resp.headers().get(header::ETAG), None);
        assert_eq!(resp.headers().get(header::LAST_MODIFIED), None);
    }

    #[tokio::test]
    async fn configured_validators_are_not_duplicated() {
        // Singleton fields, so a configured one must not end up alongside a
        // generated one. A real mtime, so `ServeDir` supplies one to collide.
        let dir = tempfile::tempdir().unwrap();
        write_asset(dir.path(), "app.js", false);

        let mut cfg = config(dir.path());
        for (name, value) in [
            ("etag", "\"pinned\""),
            ("last-modified", "Wed, 21 Oct 2015 07:28:00 GMT"),
        ] {
            cfg.headers.insert(
                name.to_string(),
                meta::rpc::static_assets::HeaderValues {
                    values: vec![value.to_string()],
                },
            );
        }
        let h = StaticAssetsHandler::new(&cfg, Some("rev1"));

        let resp = get(&h, dir.path(), &[]).await;
        let field = |n: HeaderName| -> Vec<_> { resp.headers().get_all(n).iter().collect() };
        assert_eq!(field(header::ETAG), vec!["\"pinned\""]);
        assert_eq!(
            field(header::LAST_MODIFIED),
            vec!["Wed, 21 Oct 2015 07:28:00 GMT"]
        );

        // We don't revalidate against a tag we didn't mint.
        let resp = get(&h, dir.path(), &[(header::IF_NONE_MATCH, "\"pinned\"")]).await;
        assert_eq!(resp.status(), StatusCode::OK);
    }
}
