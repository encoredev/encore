mod helpers;
use helpers::*;

// ── QUIT ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_quit() {
    let (_m, mut c) = start().await;

    // QUIT should return OK
    must_ok!(c, "QUIT");
}

// ── COMMAND ─────────────────────────────────────────────────────────

#[tokio::test]
async fn test_command() {
    let (_m, mut c) = start().await;

    let v: redis::Value = redis::cmd("COMMAND").query_async(&mut c).await.unwrap();

    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 200, "expected 200 command entries");
        }
        _ => panic!("expected array from COMMAND, got {:?}", v),
    }
}

// ── INFO ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_info_no_args() {
    let (_m, mut c) = start().await;

    let v: String = redis::cmd("INFO").query_async(&mut c).await.unwrap();

    assert!(v.contains("# Clients"), "expected Clients section");
    assert!(
        v.contains("connected_clients:"),
        "expected connected_clients"
    );
    assert!(v.contains("# Stats"), "expected Stats section");
    assert!(
        v.contains("total_connections_received:"),
        "expected total_connections_received"
    );
    assert!(
        v.contains("total_commands_processed:"),
        "expected total_commands_processed"
    );
}

#[tokio::test]
async fn test_info_clients() {
    let (_m, mut c) = start().await;

    let v: String = redis::cmd("INFO")
        .arg("clients")
        .query_async(&mut c)
        .await
        .unwrap();

    assert!(v.contains("# Clients"), "expected Clients section");
    assert!(
        v.contains("connected_clients:"),
        "expected connected_clients"
    );
    assert!(!v.contains("# Stats"), "should not contain Stats section");
}

#[tokio::test]
async fn test_info_stats() {
    let (_m, mut c) = start().await;

    let v: String = redis::cmd("INFO")
        .arg("stats")
        .query_async(&mut c)
        .await
        .unwrap();

    assert!(v.contains("# Stats"), "expected Stats section");
    assert!(
        v.contains("total_connections_received:"),
        "expected total_connections_received"
    );
    assert!(
        !v.contains("# Clients"),
        "should not contain Clients section"
    );
}

#[tokio::test]
async fn test_info_invalid_section() {
    let (_m, mut c) = start().await;

    must_fail!(c, "INFO", "bogus"; "not supported");
}

// ── CLIENT SETNAME/GETNAME ──────────────────────────────────────────

#[tokio::test]
async fn test_client_setname_getname() {
    let (_m, mut c) = start().await;

    // GETNAME before setting → nil
    must_nil!(c, "CLIENT", "GETNAME");

    // SETNAME
    must_ok!(c, "CLIENT", "SETNAME", "myconn");

    // GETNAME
    must_str!(c, "CLIENT", "GETNAME"; "myconn");

    // Reset name with empty string
    must_ok!(c, "CLIENT", "SETNAME", "");
    must_nil!(c, "CLIENT", "GETNAME");
}

#[tokio::test]
async fn test_client_setname_errors() {
    let (_m, mut c) = start().await;

    // Name with space
    must_fail!(c, "CLIENT", "SETNAME", "my name"; "cannot contain spaces");

    // Name with newline
    must_fail!(c, "CLIENT", "SETNAME", "my\nname"; "cannot contain spaces");

    // Wrong number of args
    must_fail!(c, "CLIENT", "SETNAME"; "wrong number of arguments");
    must_fail!(c, "CLIENT", "SETNAME", "a", "b"; "wrong number of arguments");
}

#[tokio::test]
async fn test_client_getname_errors() {
    let (_m, mut c) = start().await;

    // Wrong number of args
    must_fail!(c, "CLIENT", "GETNAME", "extra"; "wrong number of arguments");
}

// ── CLUSTER ─────────────────────────────────────────────────────────

#[tokio::test]
async fn test_cluster_slots() {
    let (_m, mut c) = start().await;

    let v: redis::Value = redis::cmd("CLUSTER")
        .arg("SLOTS")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 1, "expected 1 slot range");
        }
        _ => panic!("expected array from CLUSTER SLOTS, got {:?}", v),
    }
}

#[tokio::test]
async fn test_cluster_keyslot() {
    let (_m, mut c) = start().await;

    let v: i64 = redis::cmd("CLUSTER")
        .arg("KEYSLOT")
        .arg("foo")
        .query_async(&mut c)
        .await
        .unwrap();

    assert_eq!(v, 163);
}

#[tokio::test]
async fn test_cluster_nodes() {
    let (_m, mut c) = start().await;

    let v: String = redis::cmd("CLUSTER")
        .arg("NODES")
        .query_async(&mut c)
        .await
        .unwrap();

    assert!(
        v.contains("myself,master"),
        "expected myself,master, got: {}",
        v
    );
    assert!(
        v.contains("0-16383"),
        "expected 0-16383 slot range, got: {}",
        v
    );
}

#[tokio::test]
async fn test_cluster_shards() {
    let (_m, mut c) = start().await;

    let v: redis::Value = redis::cmd("CLUSTER")
        .arg("SHARDS")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(items) => {
            assert!(!items.is_empty(), "expected at least 1 shard");
        }
        _ => panic!("expected array from CLUSTER SHARDS, got {:?}", v),
    }
}

#[tokio::test]
async fn test_cluster_unknown() {
    let (_m, mut c) = start().await;

    must_fail!(c, "CLUSTER", "BOGUS"; "unknown subcommand");
}

// ── OBJECT IDLETIME ─────────────────────────────────────────────────

#[tokio::test]
async fn test_object_idletime() {
    let (_m, mut c) = start().await;

    // Non-existent key → nil
    must_nil!(c, "OBJECT", "IDLETIME", "nosuch");

    // Set a key, check idle time ≥ 0
    must_ok!(c, "SET", "key", "val");

    let v: i64 = redis::cmd("OBJECT")
        .arg("IDLETIME")
        .arg("key")
        .query_async(&mut c)
        .await
        .unwrap();

    assert!(v >= 0, "expected non-negative idle time, got {}", v);
}

#[tokio::test]
async fn test_object_idletime_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "OBJECT"; "wrong number of arguments");
    must_fail!(c, "OBJECT", "IDLETIME"; "wrong number of arguments");
    must_fail!(c, "OBJECT", "BOGUS"; "unknown subcommand");
}
