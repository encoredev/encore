mod helpers;
use helpers::*;

// ── DBSIZE ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_dbsize() {
    let (_m, mut c) = start().await;

    must_int!(c, "DBSIZE"; 0);

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");
    must_int!(c, "DBSIZE"; 2);

    must_int!(c, "DEL", "a"; 1);
    must_int!(c, "DBSIZE"; 1);
}

// ── FLUSHDB ─────────────────────────────────────────────────────────

#[tokio::test]
async fn test_flushdb() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");
    must_int!(c, "DBSIZE"; 2);

    must_ok!(c, "FLUSHDB");
    must_int!(c, "DBSIZE"; 0);
}

// ── FLUSHALL ────────────────────────────────────────────────────────

#[tokio::test]
async fn test_flushall() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SELECT", "1");
    must_ok!(c, "SET", "b", "2");

    must_ok!(c, "FLUSHALL");

    must_int!(c, "DBSIZE"; 0);
    must_ok!(c, "SELECT", "0");
    must_int!(c, "DBSIZE"; 0);
}

// ── TIME ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_time() {
    let (_m, mut c) = start().await;

    let v: redis::Value = redis::cmd("TIME").query_async(&mut c).await.unwrap();

    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 2);
        }
        _ => panic!("expected array from TIME, got {:?}", v),
    }

    // Too many args
    must_fail!(c, "TIME", "extra"; "wrong number of arguments");
}

// ── INFO ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_info() {
    let (_m, mut c) = start().await;

    // No section → returns both clients and stats
    let v: String = redis::cmd("INFO").query_async(&mut c).await.unwrap();
    assert!(
        v.contains("# Clients"),
        "expected Clients section, got: {}",
        v
    );
    assert!(
        v.contains("connected_clients"),
        "expected connected_clients, got: {}",
        v
    );

    // Specific section
    let v: String = redis::cmd("INFO")
        .arg("clients")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(
        v.contains("connected_clients"),
        "expected connected_clients, got: {}",
        v
    );

    let v: String = redis::cmd("INFO")
        .arg("stats")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(
        v.contains("total_connections_received"),
        "expected total_connections_received, got: {}",
        v
    );

    // Invalid section
    must_fail!(c, "INFO", "bogus"; "not supported");
}

// ── SWAPDB ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_swapdb() {
    let (_m, mut c) = start().await;

    // Set key in DB 0
    must_ok!(c, "SET", "key0", "val0");
    // Switch to DB 1 and set key
    must_ok!(c, "SELECT", "1");
    must_ok!(c, "SET", "key1", "val1");

    // Swap DB 0 and DB 1
    must_ok!(c, "SWAPDB", "0", "1");

    // Now DB 1 should have key0
    must_str!(c, "GET", "key0"; "val0");
    must_nil!(c, "GET", "key1");

    // Switch to DB 0, should have key1
    must_ok!(c, "SELECT", "0");
    must_str!(c, "GET", "key1"; "val1");
    must_nil!(c, "GET", "key0");
}

#[tokio::test]
async fn test_swapdb_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "SWAPDB"; "wrong number of arguments");
    must_fail!(c, "SWAPDB", "0"; "wrong number of arguments");
    must_fail!(c, "SWAPDB", "abc", "0"; "invalid first DB index");
    must_fail!(c, "SWAPDB", "0", "abc"; "invalid second DB index");
    must_fail!(c, "SWAPDB", "0", "99"; "DB index is out of range");
    must_fail!(c, "SWAPDB", "-1", "0"; "DB index is out of range");
}

// ── MEMORY USAGE ────────────────────────────────────────────────────

#[tokio::test]
async fn test_memory_usage() {
    let (_m, mut c) = start().await;

    // Non-existent key → nil
    must_nil!(c, "MEMORY", "USAGE", "nosuch");

    // String key
    must_ok!(c, "SET", "foo", "bar");
    let v: i64 = redis::cmd("MEMORY")
        .arg("USAGE")
        .arg("foo")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v > 0, "expected positive memory usage, got {}", v);
}

#[tokio::test]
async fn test_memory_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "MEMORY"; "wrong number of arguments");
    must_fail!(c, "MEMORY", "USAGE"; "wrong number of arguments");
    must_fail!(c, "MEMORY", "BOGUS"; "unknown subcommand");
}
