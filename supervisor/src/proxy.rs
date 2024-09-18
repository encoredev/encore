use anyhow::{Context, Result};
use axum::async_trait;
use bytes::Bytes;
use hyper::header;
use pingora::http::ResponseHeader;
use pingora::protocols::http::error_resp;
use pingora::proxy::{http_proxy_service, ProxyHttp, Session};
use pingora::server::configuration::{Opt, ServerConf};
use pingora::services::Service;
use pingora::upstreams::peer::HttpPeer;
use pingora::{Error, ErrorSource, ErrorType, OrErr};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::sync::watch;
use tokio_util::sync::CancellationToken;

#[derive(Clone)]
pub struct GatewayProxy {
    services: HashMap<String, u16>,
    upstream: SocketAddr,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct HealthzResponse {
    pub code: String,
    pub message: String,
    pub details: HealthzDetails,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct HealthzDetails {
    pub app_revision: String,
    pub encore_compiler: String,
    pub deploy_id: String,
    pub checks: Vec<HealthzCheckResult>,
    pub enabled_experiments: Vec<String>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct HealthzCheckResult {
    pub name: String,
    pub passed: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

impl GatewayProxy {
    pub fn new(upstream: SocketAddr, services: HashMap<String, u16>) -> Self {
        GatewayProxy { upstream, services }
    }

    pub async fn serve(self, listen_addr: String, token: &CancellationToken) {
        let conf = Arc::new(
            ServerConf::new_with_opt_override(&Opt {
                upgrade: false,
                daemon: false,
                nocapture: false,
                test: false,
                conf: None,
            })
            .unwrap(),
        );
        let mut proxy = http_proxy_service(&conf, self);

        proxy.add_tcp(listen_addr.as_str());

        let (tx, rx) = watch::channel(false);

        tokio::select! {
            _ = proxy.start_service(
                #[cfg(unix)]
                None,
                rx,
            ) => {},
            _ = token.cancelled() => {
                log::info!("Shutting down pingora proxy");
                tx.send(true).expect("failed to shutdown pingora");
            }
        }
    }

    // concurrently calls /__encore/healthz for all services. Returns "unhealthy" if any of them
    // does not return "ok".
    pub async fn health_check(&self) -> Result<HealthzResponse> {
        let handles = self.services.clone().into_iter().map(|(svc, port)| {
            let url = format!("http://127.0.0.1:{}/__encore/healthz", port);
            tokio::spawn(async move {
                let client = reqwest::Client::new();
                let res = client
                    .get(url.as_str())
                    .send()
                    .await
                    .context(format!("Failed to send request to {}", svc))?;
                if res.status().is_success() {
                    let body = res
                        .json::<HealthzResponse>()
                        .await
                        .context("Failed to parse response body")?;
                    Ok::<(String, Option<HealthzResponse>), anyhow::Error>((svc, Some(body)))
                } else {
                    Ok::<(String, Option<HealthzResponse>), anyhow::Error>((svc, None))
                }
            })
        });

        let results: Vec<(String, Option<HealthzResponse>)> = futures::future::join_all(handles)
            .await
            .into_iter()
            .map(|result| result.context("future failed")?)
            .map(|result| {
                Ok::<(String, Option<HealthzResponse>), anyhow::Error>(
                    result.context("health check failed")?,
                )
            })
            .collect::<Result<Vec<(String, Option<HealthzResponse>)>>>()?;

        let failed_checks: Vec<HealthzCheckResult> = results
            .iter()
            .filter_map(|(svc, resp)| {
                if resp.is_none() || resp.as_ref().unwrap().code != "ok" {
                    Some(HealthzCheckResult {
                        name: format!("service.{}.initialized", svc),
                        passed: false,
                        error: None,
                    })
                } else {
                    None
                }
            })
            .collect();

        if failed_checks.is_empty() {
            let (_, response) = results.first().context("no response found")?;
            let resp = response.as_ref().context("no response found")?;
            Ok(resp.clone())
        } else {
            Ok(HealthzResponse {
                code: "unhealthy".into(),
                message: "waiting for processes to start".to_string(),
                details: HealthzDetails {
                    app_revision: "".to_string(),
                    encore_compiler: "".to_string(),
                    deploy_id: "".to_string(),
                    checks: failed_checks,
                    enabled_experiments: vec![],
                },
            })
        }
    }
}

#[async_trait]
impl ProxyHttp for GatewayProxy {
    type CTX = Option<String>;

    fn new_ctx(&self) -> Self::CTX {
        None
    }

    // see https://github.com/cloudflare/pingora/blob/main/docs/user_guide/internals.md for
    // details on when different filters are called.

    async fn request_filter(
        &self,
        session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> pingora::Result<bool>
    where
        Self::CTX: Send + Sync,
    {
        if session.req_header().uri.path() == "/__encore/healthz" {
            let healthz_resp = self
                .health_check()
                .await
                .or_err(ErrorType::HTTPStatus(503), "failed to run health check")?;
            let healthz_bytes: Vec<u8> = serde_json::to_vec(&healthz_resp)
                .or_err(ErrorType::HTTPStatus(503), "could not encode response")?;

            let code = if healthz_resp.code == "ok" { 200 } else { 503 };
            let mut header = ResponseHeader::build(code, None)?;
            header.insert_header(header::CONTENT_LENGTH, healthz_bytes.len())?;
            header.insert_header(header::CONTENT_TYPE, "application/json")?;
            session
                .write_response_header(Box::new(header), false)
                .await?;
            session
                .write_response_body(Some(Bytes::from(healthz_bytes)), true)
                .await?;

            return Ok(true);
        }
        Ok(false)
    }

    async fn upstream_peer(
        &self,
        _session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> pingora::Result<Box<HttpPeer>> {
        let peer: HttpPeer = HttpPeer::new(&self.upstream, false, "localhost".to_string());
        Ok(Box::new(peer))
    }

    async fn fail_to_proxy(&self, session: &mut Session, e: &Error, _ctx: &mut Self::CTX) -> u16
    where
        Self::CTX: Send + Sync,
    {
        // modified version of `Session::respond_error`

        let code = match e.etype() {
            ErrorType::HTTPStatus(code) => *code,
            _ => {
                match e.esource() {
                    ErrorSource::Upstream => 502,
                    ErrorSource::Downstream => {
                        match e.etype() {
                            ErrorType::WriteError
                            | ErrorType::ReadError
                            | ErrorType::ConnectionClosed => {
                                /* conn already dead */
                                return 0;
                            }
                            _ => 400,
                        }
                    }
                    ErrorSource::Internal | ErrorSource::Unset => 500,
                }
            }
        };

        let (resp, body) = (
            match code {
                /* common error responses are pre-generated */
                502 => error_resp::HTTP_502_RESPONSE.clone(),
                400 => error_resp::HTTP_400_RESPONSE.clone(),
                _ => error_resp::gen_error_response(code),
            },
            None,
        );
        session.set_keepalive(None);
        session
            .write_response_header(Box::new(resp), false)
            .await
            .unwrap_or_else(|e| {
                log::error!("failed to send error response to downstream: {e}");
            });

        session
            .write_response_body(body, true)
            .await
            .unwrap_or_else(|e| log::error!("failed to write body: {e}"));

        code
    }
}
