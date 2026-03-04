mod helpers;
use helpers::*;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;

// ── Helper: raw TCP connection for RESP3 wire format tests ──────────

async fn raw_connect(m: &miniredis_rs::Miniredis) -> TcpStream {
    TcpStream::connect(m.addr()).await.unwrap()
}

/// Send a RESP2 command (array of bulk strings) and return raw response bytes.
async fn raw_cmd(stream: &mut TcpStream, args: &[&str]) -> Vec<u8> {
    // Build RESP2 array
    let mut cmd = format!("*{}\r\n", args.len());
    for arg in args {
        cmd.push_str(&format!("${}\r\n{}\r\n", arg.len(), arg));
    }
    stream.write_all(cmd.as_bytes()).await.unwrap();
    stream.flush().await.unwrap();

    // Read response (wait a bit for it to arrive)
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;
    let mut buf = vec![0u8; 4096];
    let n = stream.read(&mut buf).await.unwrap();
    buf.truncate(n);
    buf
}

// ── HELLO command tests ─────────────────────────────────────────────

#[tokio::test]
async fn test_hello_2_returns_map_as_array() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // HELLO 2 should return a flat array (RESP2 encoding of Map)
    let resp = raw_cmd(&mut stream, &["HELLO", "2"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    // Should start with * (RESP2 array), containing 14 elements (7 key-value pairs)
    assert!(
        resp_str.starts_with("*14\r\n"),
        "expected RESP2 array *14, got: {:?}",
        resp_str
    );
}

#[tokio::test]
async fn test_hello_3_returns_resp3_map() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // HELLO 3 should return a RESP3 map
    let resp = raw_cmd(&mut stream, &["HELLO", "3"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    // Should start with % (RESP3 map) with 7 key-value pairs
    assert!(
        resp_str.starts_with("%7\r\n"),
        "expected RESP3 map %7, got: {:?}",
        resp_str
    );
}

#[tokio::test]
async fn test_hello_3_enables_resp3_null() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // Switch to RESP3
    let _ = raw_cmd(&mut stream, &["HELLO", "3"]).await;

    // GET on non-existent key should return RESP3 null: _\r\n
    let resp = raw_cmd(&mut stream, &["GET", "nosuch"]).await;
    assert_eq!(
        resp,
        b"_\r\n",
        "expected RESP3 null, got: {:?}",
        String::from_utf8_lossy(&resp)
    );
}

#[tokio::test]
async fn test_resp2_null_is_dollar_minus_one() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // Without HELLO 3, GET on non-existent key should return RESP2 null: $-1\r\n
    let resp = raw_cmd(&mut stream, &["GET", "nosuch"]).await;
    assert_eq!(
        resp,
        b"$-1\r\n",
        "expected RESP2 null, got: {:?}",
        String::from_utf8_lossy(&resp)
    );
}

// ── HGETALL RESP3 map ───────────────────────────────────────────────

#[tokio::test]
async fn test_hgetall_resp2_flat_array() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // Set up a hash
    let _ = raw_cmd(&mut stream, &["HSET", "h", "f1", "v1"]).await;

    // HGETALL in RESP2 mode returns flat array
    let resp = raw_cmd(&mut stream, &["HGETALL", "h"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    assert!(
        resp_str.starts_with("*2\r\n"),
        "expected RESP2 array *2, got: {:?}",
        resp_str
    );
}

#[tokio::test]
async fn test_hgetall_resp3_map() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // Set up a hash
    let _ = raw_cmd(&mut stream, &["HSET", "h", "f1", "v1"]).await;

    // Switch to RESP3
    let _ = raw_cmd(&mut stream, &["HELLO", "3"]).await;

    // HGETALL in RESP3 mode returns map
    let resp = raw_cmd(&mut stream, &["HGETALL", "h"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    assert!(
        resp_str.starts_with("%1\r\n"),
        "expected RESP3 map %1, got: {:?}",
        resp_str
    );
    // Verify it contains the field-value pair
    assert!(
        resp_str.contains("f1"),
        "response should contain field 'f1'"
    );
    assert!(
        resp_str.contains("v1"),
        "response should contain value 'v1'"
    );
}

// ── SMEMBERS RESP3 set ──────────────────────────────────────────────

#[tokio::test]
async fn test_smembers_resp2_array() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    let _ = raw_cmd(&mut stream, &["SADD", "s", "a", "b"]).await;

    // SMEMBERS in RESP2 mode returns array
    let resp = raw_cmd(&mut stream, &["SMEMBERS", "s"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    assert!(
        resp_str.starts_with("*2\r\n"),
        "expected RESP2 array *2, got: {:?}",
        resp_str
    );
}

#[tokio::test]
async fn test_smembers_resp3_set() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    let _ = raw_cmd(&mut stream, &["SADD", "s", "a", "b"]).await;

    // Switch to RESP3
    let _ = raw_cmd(&mut stream, &["HELLO", "3"]).await;

    // SMEMBERS in RESP3 mode returns set (~)
    let resp = raw_cmd(&mut stream, &["SMEMBERS", "s"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    assert!(
        resp_str.starts_with("~2\r\n"),
        "expected RESP3 set ~2, got: {:?}",
        resp_str
    );
}

// ── Commands still work after HELLO 3 ───────────────────────────────

#[tokio::test]
async fn test_commands_work_after_hello_3() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // Switch to RESP3
    let _ = raw_cmd(&mut stream, &["HELLO", "3"]).await;

    // SET should still return +OK
    let resp = raw_cmd(&mut stream, &["SET", "key", "value"]).await;
    assert_eq!(resp, b"+OK\r\n");

    // GET should return bulk string
    let resp = raw_cmd(&mut stream, &["GET", "key"]).await;
    assert_eq!(resp, b"$5\r\nvalue\r\n");

    // Integer commands work
    let resp = raw_cmd(&mut stream, &["DEL", "key"]).await;
    assert_eq!(resp, b":1\r\n");
}

#[tokio::test]
async fn test_hello_3_then_hello_2_resets() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let mut stream = raw_connect(&m).await;

    // Switch to RESP3
    let resp = raw_cmd(&mut stream, &["HELLO", "3"]).await;
    assert!(String::from_utf8_lossy(&resp).starts_with("%"));

    // Switch back to RESP2
    let resp = raw_cmd(&mut stream, &["HELLO", "2"]).await;
    // HELLO 2 returns Map but serialized as RESP2 flat array since we just set resp2
    assert!(String::from_utf8_lossy(&resp).starts_with("*"));

    // GET on non-existent key should now return RESP2 null
    let resp = raw_cmd(&mut stream, &["GET", "nosuch"]).await;
    assert_eq!(resp, b"$-1\r\n");
}

// ── HELLO with AUTH ─────────────────────────────────────────────────

#[tokio::test]
async fn test_hello_3_with_auth() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    m.require_auth("password");

    let mut stream = raw_connect(&m).await;

    // HELLO 3 AUTH default password
    let resp = raw_cmd(&mut stream, &["HELLO", "3", "AUTH", "default", "password"]).await;
    let resp_str = String::from_utf8_lossy(&resp);
    assert!(
        resp_str.starts_with("%7\r\n"),
        "expected RESP3 map, got: {:?}",
        resp_str
    );

    // Should be authenticated and in RESP3 mode
    let resp = raw_cmd(&mut stream, &["SET", "k", "v"]).await;
    assert_eq!(resp, b"+OK\r\n");
}

// ── Via redis-rs client ─────────────────────────────────────────────

#[tokio::test]
async fn test_hello_via_redis_rs() {
    let (_m, mut c) = start().await;

    // HELLO 2 should work via redis-rs
    let result: Vec<redis::Value> = redis::cmd("HELLO")
        .arg("2")
        .query_async(&mut c)
        .await
        .unwrap();
    // Returns as flat array in RESP2 mode
    assert!(!result.is_empty());

    // Commands should still work
    must_ok!(c, "SET", "k", "v");
    must_str!(c, "GET", "k"; "v");
}

#[tokio::test]
async fn test_hello_invalid_version() {
    let (_m, mut c) = start().await;

    must_fail!(c, "HELLO", "4"; "NOPROTO");
    must_fail!(c, "HELLO", "0"; "NOPROTO");
}
