mod helpers;
use helpers::*;

// ── Error cases ──────────────────────────────────────────────────────

#[tokio::test]
async fn test_exec_without_multi() {
    let (_m, mut c) = start().await;
    must_fail!(c, "EXEC"; "EXEC without MULTI");
}

#[tokio::test]
async fn test_discard_without_multi() {
    let (_m, mut c) = start().await;
    must_fail!(c, "DISCARD"; "DISCARD without MULTI");
}

#[tokio::test]
async fn test_multi_nested() {
    let (_m, mut c) = start().await;

    // First MULTI → OK
    must_ok!(c, "MULTI");

    // Second MULTI → error
    must_fail!(c, "MULTI"; "MULTI calls can not be nested");

    // Clean up
    must_ok!(c, "DISCARD");
}

// ── Basic MULTI / EXEC ──────────────────────────────────────────────

#[tokio::test]
async fn test_multi_basic() {
    let (_m, mut c) = start().await;
    must_ok!(c, "MULTI");
    // Clean up
    must_ok!(c, "DISCARD");
}

#[tokio::test]
async fn test_simple_transaction() {
    let (_m, mut c) = start().await;

    // MULTI
    must_ok!(c, "MULTI");

    // SET is queued
    let v: String = redis::cmd("SET")
        .arg("aap")
        .arg("1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // EXEC returns array of results
    let v: Vec<String> = redis::cmd("EXEC").query_async(&mut c).await.unwrap();
    assert_eq!(v, vec!["OK"]);

    // Key should now be set
    must_str!(c, "GET", "aap"; "1");

    // Commands should be back to normal mode
    must_ok!(c, "SET", "aap", "2");
    must_str!(c, "GET", "aap"; "2");
}

#[tokio::test]
async fn test_multi_exec_multiple_commands() {
    let (_m, mut c) = start().await;

    must_ok!(c, "MULTI");

    let v: String = redis::cmd("SET")
        .arg("k1")
        .arg("v1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    let v: String = redis::cmd("SET")
        .arg("k2")
        .arg("v2")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    let v: String = redis::cmd("GET")
        .arg("k1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // EXEC
    let v: redis::Value = redis::cmd("EXEC").query_async(&mut c).await.unwrap();

    // Should be Array [OK, OK, "v1"]
    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 3);
        }
        _ => panic!("expected array from EXEC, got {:?}", v),
    }

    // Verify
    must_str!(c, "GET", "k1"; "v1");
    must_str!(c, "GET", "k2"; "v2");
}

// ── DISCARD ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_discard_transaction() {
    let (_m, mut c) = start().await;

    // Pre-set a key
    must_ok!(c, "SET", "aap", "noot");

    // MULTI + queue a change
    must_ok!(c, "MULTI");
    let v: String = redis::cmd("SET")
        .arg("aap")
        .arg("mies")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // DISCARD
    must_ok!(c, "DISCARD");

    // Key should still have original value
    must_str!(c, "GET", "aap"; "noot");
}

// ── WATCH ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_watch_basic() {
    let (_m, mut c) = start().await;
    must_ok!(c, "WATCH", "foo");
}

#[tokio::test]
async fn test_watch_in_multi() {
    let (_m, mut c) = start().await;

    must_ok!(c, "MULTI");

    // WATCH inside MULTI should error
    must_fail!(c, "WATCH", "foo"; "WATCH inside MULTI");

    must_ok!(c, "DISCARD");
}

#[tokio::test]
async fn test_watch_exec_success() {
    let (_m, mut c) = start().await;

    // Set initial value
    must_ok!(c, "SET", "one", "two");

    // WATCH the key
    must_ok!(c, "WATCH", "one");

    // MULTI + GET
    must_ok!(c, "MULTI");
    let v: String = redis::cmd("GET")
        .arg("one")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // EXEC — no changes to watched key, so it should succeed
    let v: Vec<String> = redis::cmd("EXEC").query_async(&mut c).await.unwrap();
    assert_eq!(v, vec!["two"]);
}

#[tokio::test]
async fn test_watch_exec_fail() {
    let (_m, mut c1, mut c2) = start_two_clients().await;

    // Set initial value
    must_ok!(c1, "SET", "one", "two");

    // c1: WATCH the key
    must_ok!(c1, "WATCH", "one");

    // c2: Modify the watched key
    must_ok!(c2, "SET", "one", "three");

    // c1: MULTI + GET + EXEC → should return nil (WATCH abort)
    must_ok!(c1, "MULTI");
    let v: String = redis::cmd("GET")
        .arg("one")
        .query_async(&mut c1)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // EXEC should return nil because the watched key was modified
    let v: redis::Value = redis::cmd("EXEC").query_async(&mut c1).await.unwrap();
    assert_eq!(v, redis::Value::Nil);

    // We're no longer in a transaction; key has the value set by c2
    must_str!(c1, "GET", "one"; "three");
}

// ── UNWATCH ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_unwatch() {
    let (_m, mut c1, mut c2) = start_two_clients().await;

    // Set initial value
    must_ok!(c1, "SET", "one", "two");

    // c1: WATCH the key
    must_ok!(c1, "WATCH", "one");

    // c1: UNWATCH — cancels the watch
    must_ok!(c1, "UNWATCH");

    // c2: Modify the key (would have triggered WATCH failure, but we unwatched)
    must_ok!(c2, "SET", "one", "three");

    // c1: MULTI + SET + EXEC → should succeed because we unwatched
    must_ok!(c1, "MULTI");
    let v: String = redis::cmd("SET")
        .arg("one")
        .arg("four")
        .query_async(&mut c1)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    let v: Vec<String> = redis::cmd("EXEC").query_async(&mut c1).await.unwrap();
    assert_eq!(v, vec!["OK"]);

    // Key should have the value from our transaction
    must_str!(c1, "GET", "one"; "four");
}

// ── Transaction with pipe().atomic() ─────────────────────────────────

#[tokio::test]
async fn test_pipe_atomic() {
    let (_m, mut c) = start().await;

    // Use the redis crate's built-in atomic pipe (MULTI/EXEC wrapper)
    let (v1, v2): (String, i64) = redis::pipe()
        .atomic()
        .cmd("SET")
        .arg("k")
        .arg("hello")
        .cmd("INCR")
        .arg("counter")
        .query_async(&mut c)
        .await
        .unwrap();

    assert_eq!(v1, "OK");
    assert_eq!(v2, 1);

    must_str!(c, "GET", "k"; "hello");
    must_int!(c, "GET", "counter"; 1);
}

// ── MULTI with unknown command → EXECABORT ───────────────────────────

#[tokio::test]
async fn test_tx_queue_err() {
    let (_m, mut c) = start().await;

    must_ok!(c, "MULTI");

    // Valid command
    let v: String = redis::cmd("SET")
        .arg("aap")
        .arg("mies")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // Unknown command → error and dirty transaction
    must_fail!(c, "NOSUCHCOMMAND", "arg"; "unknown command");

    // Another valid command still queues
    let v: String = redis::cmd("SET")
        .arg("noot")
        .arg("vuur")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    // EXEC should fail with EXECABORT because of the unknown command
    must_fail!(c, "EXEC"; "Transaction discarded");

    // Nothing should have been executed
    must_nil!(c, "GET", "aap");
}

// ── EVAL/EVALSHA inside MULTI/EXEC ───────────────────────────────────

#[tokio::test]
async fn test_lua_tx_eval() {
    let (_m, mut c) = start().await;

    must_ok!(c, "MULTI");

    let v: String = redis::cmd("EVAL")
        .arg("return {ARGV[1]}")
        .arg("0")
        .arg("key1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    let v: redis::Value = redis::cmd("EXEC").query_async(&mut c).await.unwrap();
    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 1);
        }
        _ => panic!("expected array from EXEC, got {:?}", v),
    }
}

#[tokio::test]
async fn test_lua_tx_evalsha() {
    let (_m, mut c) = start().await;

    must_ok!(c, "MULTI");

    // SCRIPT LOAD inside MULTI
    let v: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg("return {KEYS[1],ARGV[1]}")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    let script_sha = "bfbf458525d6a0b19200bfd6db3af481156b367b";
    let v: String = redis::cmd("EVALSHA")
        .arg(script_sha)
        .arg("1")
        .arg("key1")
        .arg("key2")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");

    let v: redis::Value = redis::cmd("EXEC").query_async(&mut c).await.unwrap();
    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 2);
        }
        _ => panic!("expected array from EXEC, got {:?}", v),
    }
}
