use std::sync::Arc;

use miniredis_rs::Miniredis;
use tokio::io::{AsyncReadExt, AsyncWriteExt};

/// Generate a self-signed TLS certificate and return (server_config, cert_der).
fn generate_tls_config() -> (Arc<rustls::ServerConfig>, Vec<u8>) {
    let cert = rcgen::generate_simple_self_signed(vec!["localhost".to_string()]).unwrap();
    let cert_der_bytes = cert.cert.der().to_vec();
    let cert_der = rustls::pki_types::CertificateDer::from(cert_der_bytes.clone());
    let key_der = rustls::pki_types::PrivateKeyDer::Pkcs8(cert.signing_key.serialize_der().into());

    let config = rustls::ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(vec![cert_der], key_der)
        .unwrap();
    (Arc::new(config), cert_der_bytes)
}

/// Create a TLS client connector that trusts the given cert.
fn make_tls_connector(cert_der: &[u8]) -> tokio_rustls::TlsConnector {
    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add(rustls::pki_types::CertificateDer::from(cert_der.to_vec()))
        .unwrap();

    let config = rustls::ClientConfig::builder()
        .with_root_certificates(root_store)
        .with_no_client_auth();

    tokio_rustls::TlsConnector::from(Arc::new(config))
}

/// Send a RESP2 command over a TLS stream and return the raw response.
async fn tls_cmd(
    stream: &mut tokio_rustls::client::TlsStream<tokio::net::TcpStream>,
    args: &[&str],
) -> Vec<u8> {
    let mut cmd = format!("*{}\r\n", args.len());
    for arg in args {
        cmd.push_str(&format!("${}\r\n{}\r\n", arg.len(), arg));
    }
    stream.write_all(cmd.as_bytes()).await.unwrap();
    stream.flush().await.unwrap();

    tokio::time::sleep(std::time::Duration::from_millis(50)).await;
    let mut buf = vec![0u8; 4096];
    let n = stream.read(&mut buf).await.unwrap();
    buf.truncate(n);
    buf
}

// ── Tests ───────────────────────────────────────────────────────────

#[tokio::test]
async fn test_tls_server_starts() {
    let (tls_config, _) = generate_tls_config();
    let m = Miniredis::run_tls(tls_config).await.unwrap();

    assert!(m.port() > 0);
    assert!(m.tls_url().starts_with("rediss://"));
}

#[tokio::test]
async fn test_tls_direct_api_works() {
    let (tls_config, _) = generate_tls_config();
    let m = Miniredis::run_tls(tls_config).await.unwrap();

    // Direct API works regardless of TLS
    m.set("key", "value");
    assert_eq!(m.get("key"), Some("value".to_string()));
}

#[tokio::test]
async fn test_tls_ping() {
    let (tls_config, cert_der) = generate_tls_config();
    let m = Miniredis::run_tls(tls_config).await.unwrap();

    let connector = make_tls_connector(&cert_der);
    let tcp = tokio::net::TcpStream::connect(m.addr()).await.unwrap();
    let server_name = rustls::pki_types::ServerName::try_from("localhost").unwrap();
    let mut tls = connector.connect(server_name, tcp).await.unwrap();

    let resp = tls_cmd(&mut tls, &["PING"]).await;
    assert_eq!(resp, b"+PONG\r\n");
}

#[tokio::test]
async fn test_tls_set_get() {
    let (tls_config, cert_der) = generate_tls_config();
    let m = Miniredis::run_tls(tls_config).await.unwrap();

    let connector = make_tls_connector(&cert_der);
    let tcp = tokio::net::TcpStream::connect(m.addr()).await.unwrap();
    let server_name = rustls::pki_types::ServerName::try_from("localhost").unwrap();
    let mut tls = connector.connect(server_name, tcp).await.unwrap();

    let resp = tls_cmd(&mut tls, &["SET", "foo", "bar"]).await;
    assert_eq!(resp, b"+OK\r\n");

    let resp = tls_cmd(&mut tls, &["GET", "foo"]).await;
    assert_eq!(resp, b"$3\r\nbar\r\n");

    // Verify via direct API
    assert_eq!(m.get("foo"), Some("bar".to_string()));
}

#[tokio::test]
async fn test_tls_multiple_commands() {
    let (tls_config, cert_der) = generate_tls_config();
    let m = Miniredis::run_tls(tls_config).await.unwrap();

    let connector = make_tls_connector(&cert_der);
    let tcp = tokio::net::TcpStream::connect(m.addr()).await.unwrap();
    let server_name = rustls::pki_types::ServerName::try_from("localhost").unwrap();
    let mut tls = connector.connect(server_name, tcp).await.unwrap();

    // Run several commands over TLS
    let resp = tls_cmd(&mut tls, &["SET", "k1", "v1"]).await;
    assert_eq!(resp, b"+OK\r\n");

    let resp = tls_cmd(&mut tls, &["SET", "k2", "v2"]).await;
    assert_eq!(resp, b"+OK\r\n");

    let resp = tls_cmd(&mut tls, &["DEL", "k1"]).await;
    assert_eq!(resp, b":1\r\n");

    let resp = tls_cmd(&mut tls, &["EXISTS", "k1"]).await;
    assert_eq!(resp, b":0\r\n");

    let resp = tls_cmd(&mut tls, &["GET", "k2"]).await;
    assert_eq!(resp, b"$2\r\nv2\r\n");
}

#[tokio::test]
async fn test_plain_tcp_to_tls_server_fails() {
    let (tls_config, _) = generate_tls_config();
    let m = Miniredis::run_tls(tls_config).await.unwrap();

    // Plain TCP should not get a valid response from TLS server
    let mut stream = tokio::net::TcpStream::connect(m.addr()).await.unwrap();
    let cmd = b"*1\r\n$4\r\nPING\r\n";
    stream.write_all(cmd).await.unwrap();
    stream.flush().await.unwrap();

    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    let mut buf = vec![0u8; 1024];
    let result =
        tokio::time::timeout(std::time::Duration::from_millis(200), stream.read(&mut buf)).await;

    match result {
        Ok(Ok(0)) => {} // connection closed - expected
        Ok(Ok(n)) => {
            // Got bytes but they shouldn't be a valid RESP response
            let resp = &buf[..n];
            assert_ne!(resp, b"+PONG\r\n", "should not get PONG without TLS");
        }
        Ok(Err(_)) => {} // error - expected
        Err(_) => {}     // timeout - expected
    }
}
