//! A thin CLI wrapper around miniredis-rs that speaks enough redis-server
//! config to be used as a drop-in replacement in the miniredis Go integration
//! test suite.
//!
//! Config lines are read from stdin (same as `redis-server -`).
//! Recognised directives:
//!   port <n>           – ignored (always binds to 0, prints actual port)
//!   bind <addr>        – bind address (default 127.0.0.1)
//!   requirepass <pw>   – set default-user password
//!   user <name> on +@all ~* ><password> – add ACL user
//!   tls-port <n>       – enable TLS listener (port is ignored, uses 0)
//!   tls-cert-file <p>  – server certificate path
//!   tls-key-file <p>   – server private key path
//!   tls-ca-cert-file <p> – CA / client certificate path
//!   appendonly …       – silently ignored
//!   cluster-enabled …  – silently ignored
//!   cluster-config-file … – silently ignored
//!
//! Once ready, the actual listening port is printed to stdout as a single line:
//!   PORT=<n>
//!
//! The process exits cleanly on SIGTERM or SIGINT.

use std::io::{self, BufRead};
use std::sync::Arc;

use miniredis_rs::Miniredis;
use tokio::signal::unix::{SignalKind, signal};

#[cfg(feature = "tls")]
use std::fs;

#[cfg(feature = "tls")]
fn load_tls_config(
    cert_path: &str,
    key_path: &str,
    ca_cert_path: &str,
) -> Arc<rustls::ServerConfig> {
    let cert_pem = fs::read(cert_path).expect("read cert file");
    let key_pem = fs::read(key_path).expect("read key file");
    let ca_pem = fs::read(ca_cert_path).expect("read CA cert file");

    let certs: Vec<_> = rustls_pemfile::certs(&mut &cert_pem[..])
        .collect::<Result<Vec<_>, _>>()
        .expect("parse certs");

    let key = rustls_pemfile::private_key(&mut &key_pem[..])
        .expect("parse key")
        .expect("no key found");

    let mut root_store = rustls::RootCertStore::empty();
    for cert in rustls_pemfile::certs(&mut &ca_pem[..]) {
        root_store
            .add(cert.expect("parse CA cert"))
            .expect("add CA cert");
    }

    let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(root_store))
        .build()
        .expect("build client verifier");

    let config = rustls::ServerConfig::builder()
        .with_client_cert_verifier(verifier)
        .with_single_cert(certs, key)
        .expect("build TLS config");

    Arc::new(config)
}

#[tokio::main]
async fn main() {
    let mut bind_addr = "127.0.0.1".to_string();
    let mut password: Option<String> = None;
    let mut users: Vec<(String, String)> = Vec::new();
    let mut tls_enabled = false;
    let mut tls_cert = String::new();
    let mut tls_key = String::new();
    let mut tls_ca_cert = String::new();

    // Read config from stdin
    let stdin = io::stdin();
    for line in stdin.lock().lines() {
        let line = line.expect("read stdin");
        let line = line.trim().to_string();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let parts: Vec<&str> = line.split_whitespace().collect();
        if parts.is_empty() {
            continue;
        }
        match parts[0].to_lowercase().as_str() {
            "port" => { /* ignored – always use port 0 */ }
            "bind" => {
                if parts.len() > 1 {
                    bind_addr = parts[1].to_string();
                }
            }
            "requirepass" => {
                if parts.len() > 1 {
                    password = Some(parts[1].to_string());
                }
            }
            "user" => {
                // user <name> on +@all ~* ><password>
                // or: user default on -@all +hello
                if parts.len() >= 2 {
                    let username = parts[1].to_string();
                    // Find the >password token
                    let mut pw = None;
                    for part in &parts[2..] {
                        if let Some(p) = part.strip_prefix('>') {
                            pw = Some(p.to_string());
                        }
                    }
                    if let Some(p) = pw {
                        users.push((username, p));
                    }
                    // "user default on -@all +hello" (no password) is ignored
                }
            }
            "tls-port" => {
                tls_enabled = true;
            }
            "tls-cert-file" => {
                if parts.len() > 1 {
                    tls_cert = parts[1].to_string();
                }
            }
            "tls-key-file" => {
                if parts.len() > 1 {
                    tls_key = parts[1].to_string();
                }
            }
            "tls-ca-cert-file" => {
                if parts.len() > 1 {
                    tls_ca_cert = parts[1].to_string();
                }
            }
            // Silently ignore everything else
            _ => {}
        }
    }

    let m = if tls_enabled {
        #[cfg(feature = "tls")]
        {
            let tls_config = load_tls_config(&tls_cert, &tls_key, &tls_ca_cert);
            Miniredis::run_tls_addr(&format!("{}:0", bind_addr), tls_config)
                .await
                .expect("start TLS server")
        }
        #[cfg(not(feature = "tls"))]
        {
            panic!("TLS requested but binary was compiled without tls feature");
        }
    } else {
        Miniredis::run_addr(&format!("{}:0", bind_addr))
            .await
            .expect("start server")
    };

    // Set up authentication
    if let Some(pw) = &password {
        m.require_auth(pw);
    }
    for (user, pw) in &users {
        m.require_user_auth(user, pw);
    }

    // Print the port – the Go test harness reads this as readiness signal.
    println!("PORT={}", m.port());

    // Wait for SIGTERM or SIGINT
    let mut sigterm = signal(SignalKind::terminate()).expect("signal handler");
    let mut sigint = signal(SignalKind::interrupt()).expect("signal handler");
    tokio::select! {
        _ = sigterm.recv() => {}
        _ = sigint.recv() => {}
    }

    m.close().await;
}
