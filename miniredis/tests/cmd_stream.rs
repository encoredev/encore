mod helpers;
use helpers::*;

// ── XADD / XLEN ─────────────────────────────────────────────────────

#[tokio::test]
async fn test_xadd_basic() {
    let (_m, mut c) = start().await;

    // XADD with explicit ID
    let id: String = redis::cmd("XADD")
        .arg("s")
        .arg("1-1")
        .arg("name")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(id, "1-1");

    must_int!(c, "XLEN", "s"; 1);

    // Second entry
    let id2: String = redis::cmd("XADD")
        .arg("s")
        .arg("2-1")
        .arg("name")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(id2, "2-1");

    must_int!(c, "XLEN", "s"; 2);

    // TYPE
    must_str!(c, "TYPE", "s"; "stream");
}

#[tokio::test]
async fn test_xadd_auto_id() {
    let (_m, mut c) = start().await;

    // Auto-generated ID
    let id: String = redis::cmd("XADD")
        .arg("s")
        .arg("*")
        .arg("name")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(id.contains('-'), "expected id with '-', got {}", id);

    must_int!(c, "XLEN", "s"; 1);
}

#[tokio::test]
async fn test_xadd_partial_id() {
    let (_m, mut c) = start().await;

    // Partial auto-sequence: "123-*"
    let id: String = redis::cmd("XADD")
        .arg("s")
        .arg("123-*")
        .arg("name")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(
        id.starts_with("123-"),
        "expected id starting with '123-', got {}",
        id
    );

    // Second with same ms should increment seq
    let id2: String = redis::cmd("XADD")
        .arg("s")
        .arg("123-*")
        .arg("name")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(
        id2.starts_with("123-"),
        "expected id starting with '123-', got {}",
        id2
    );
    assert_ne!(id, id2);
}

#[tokio::test]
async fn test_xadd_maxlen() {
    let (_m, mut c) = start().await;

    // Add entries with MAXLEN trimming
    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg("MAXLEN")
            .arg("3")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_int!(c, "XLEN", "s"; 3);
}

#[tokio::test]
async fn test_xadd_minid() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // XADD with MINID trimming
    redis::cmd("XADD")
        .arg("s")
        .arg("MINID")
        .arg("4")
        .arg("6-0")
        .arg("v")
        .arg("6")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    // Only entries >= 4-0 should remain (4-0, 5-0, 6-0)
    must_int!(c, "XLEN", "s"; 3);
}

#[tokio::test]
async fn test_xadd_nomkstream() {
    let (_m, mut c) = start().await;

    // NOMKSTREAM on non-existing key should not create stream
    let result: redis::RedisResult<Option<String>> = redis::cmd("XADD")
        .arg("nosuch")
        .arg("NOMKSTREAM")
        .arg("*")
        .arg("field")
        .arg("value")
        .query_async(&mut c)
        .await;
    match result {
        Ok(None) => {} // Expected: nil response
        Ok(Some(v)) => panic!("expected nil, got {:?}", v),
        Err(e) => panic!("unexpected error: {:?}", e),
    }

    must_int!(c, "XLEN", "nosuch"; 0);
}

#[tokio::test]
async fn test_xadd_errors() {
    let (_m, mut c) = start().await;

    // Wrong number of args
    must_fail!(c, "XADD"; "wrong number of arguments");
    must_fail!(c, "XADD", "s"; "wrong number of arguments");
    must_fail!(c, "XADD", "s", "*"; "wrong number of arguments");

    // Odd field-value pairs
    must_fail!(c, "XADD", "s", "*", "field"; "wrong number of arguments");

    // Invalid ID (0-0)
    must_fail!(c, "XADD", "s", "0-0", "f", "v"; "must be greater than 0-0");

    // Wrong type
    must_ok!(c, "SET", "str", "value");
    must_fail!(c, "XADD", "str", "*", "f", "v"; "WRONGTYPE");
}

#[tokio::test]
async fn test_xadd_duplicate_id() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-1")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    // Same ID should fail
    must_fail!(c, "XADD", "s", "1-1", "f", "v"; "equal or smaller");

    // Smaller ID should also fail
    must_fail!(c, "XADD", "s", "1-0", "f", "v"; "equal or smaller");
}

// ── XLEN ─────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xlen() {
    let (_m, mut c) = start().await;

    must_int!(c, "XLEN", "nosuch"; 0);

    redis::cmd("XADD")
        .arg("s")
        .arg("1-1")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_int!(c, "XLEN", "s"; 1);

    // Wrong type
    must_ok!(c, "SET", "str", "value");
    must_fail!(c, "XLEN", "str"; "WRONGTYPE");
}

// ── XRANGE / XREVRANGE ──────────────────────────────────────────────

#[tokio::test]
async fn test_xrange() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // Full range
    let result: redis::Value = redis::cmd("XRANGE")
        .arg("s")
        .arg("-")
        .arg("+")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 5);

    // Partial range
    let result: redis::Value = redis::cmd("XRANGE")
        .arg("s")
        .arg("2")
        .arg("4")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 3);

    // With COUNT
    let result: redis::Value = redis::cmd("XRANGE")
        .arg("s")
        .arg("-")
        .arg("+")
        .arg("COUNT")
        .arg("2")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 2);
}

#[tokio::test]
async fn test_xrevrange() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // Full reverse range
    let result: redis::Value = redis::cmd("XREVRANGE")
        .arg("s")
        .arg("+")
        .arg("-")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 5);

    // First entry should be the highest ID
    let first = as_array(&entries[0]);
    let id = as_string(&first[0]);
    assert_eq!(id, "5-0");

    // With COUNT
    let result: redis::Value = redis::cmd("XREVRANGE")
        .arg("s")
        .arg("+")
        .arg("-")
        .arg("COUNT")
        .arg("2")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 2);
}

#[tokio::test]
async fn test_xrange_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "XRANGE"; "wrong number of arguments");
    must_fail!(c, "XRANGE", "s"; "wrong number of arguments");
    must_fail!(c, "XRANGE", "s", "-"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "value");
    must_fail!(c, "XRANGE", "str", "-", "+"; "WRONGTYPE");
}

// ── XREAD ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xread() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // Read all from beginning
    let result: redis::Value = redis::cmd("XREAD")
        .arg("STREAMS")
        .arg("s")
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    assert_eq!(streams.len(), 1);

    // stream = [name, entries]
    let stream = as_array(&streams[0]);
    let entries = as_array(&stream[1]);
    assert_eq!(entries.len(), 3);

    // Read from after 1-0
    let result: redis::Value = redis::cmd("XREAD")
        .arg("STREAMS")
        .arg("s")
        .arg("1-0")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    let stream = as_array(&streams[0]);
    let entries = as_array(&stream[1]);
    assert_eq!(entries.len(), 2);
}

#[tokio::test]
async fn test_xread_count() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    let result: redis::Value = redis::cmd("XREAD")
        .arg("COUNT")
        .arg("2")
        .arg("STREAMS")
        .arg("s")
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    let stream = as_array(&streams[0]);
    let entries = as_array(&stream[1]);
    assert_eq!(entries.len(), 2);
}

#[tokio::test]
async fn test_xread_multi_streams() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s1")
        .arg("1-0")
        .arg("v")
        .arg("1")
        .query_async::<String>(&mut c)
        .await
        .unwrap();
    redis::cmd("XADD")
        .arg("s2")
        .arg("1-0")
        .arg("v")
        .arg("2")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    let result: redis::Value = redis::cmd("XREAD")
        .arg("STREAMS")
        .arg("s1")
        .arg("s2")
        .arg("0")
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    assert_eq!(streams.len(), 2);
}

#[tokio::test]
async fn test_xread_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "XREAD"; "wrong number of arguments");
    must_fail!(c, "XREAD", "STREAMS"; "wrong number of arguments");
}

// ── XDEL ─────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xdel() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // Delete one entry
    must_int!(c, "XDEL", "s", "2-0"; 1);
    must_int!(c, "XLEN", "s"; 2);

    // Delete already-deleted
    must_int!(c, "XDEL", "s", "2-0"; 0);

    // Delete non-existing key
    must_int!(c, "XDEL", "nosuch", "1-0"; 0);
}

// ── XTRIM ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xtrim_maxlen() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // Trim to 3
    must_int!(c, "XTRIM", "s", "MAXLEN", "3"; 2);
    must_int!(c, "XLEN", "s"; 3);

    // Check first remaining entry is 3-0
    let result: redis::Value = redis::cmd("XRANGE")
        .arg("s")
        .arg("-")
        .arg("+")
        .arg("COUNT")
        .arg("1")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    let first = as_array(&entries[0]);
    assert_eq!(as_string(&first[0]), "3-0");
}

#[tokio::test]
async fn test_xtrim_minid() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    // Trim to MINID 3
    must_int!(c, "XTRIM", "s", "MINID", "3"; 2);
    must_int!(c, "XLEN", "s"; 3);
}

// ── XINFO ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xinfo_stream() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    let result: redis::Value = redis::cmd("XINFO")
        .arg("STREAM")
        .arg("s")
        .query_async(&mut c)
        .await
        .unwrap();
    let items = as_array(&result);
    // Should contain key-value pairs: length, groups, last-generated-id, ...
    assert!(
        items.len() >= 6,
        "expected at least 6 items, got {}",
        items.len()
    );

    // Find "length" key
    let mut found_length = false;
    for chunk in items.chunks(2) {
        if as_string(&chunk[0]) == "length" {
            assert_eq!(as_int(&chunk[1]), 3);
            found_length = true;
        }
    }
    assert!(found_length, "expected 'length' field in XINFO STREAM");
}

#[tokio::test]
async fn test_xinfo_groups() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    let result: redis::Value = redis::cmd("XINFO")
        .arg("GROUPS")
        .arg("s")
        .query_async(&mut c)
        .await
        .unwrap();
    let groups = as_array(&result);
    assert_eq!(groups.len(), 1);
}

#[tokio::test]
async fn test_xinfo_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "XINFO"; "wrong number of arguments");
    must_fail!(c, "XINFO", "STREAM", "nosuch"; "no such key");
}

// ── XGROUP ───────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xgroup_create() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // Duplicate group
    must_fail!(c, "XGROUP", "CREATE", "s", "g1", "0"; "BUSYGROUP");
}

#[tokio::test]
async fn test_xgroup_create_mkstream() {
    let (_m, mut c) = start().await;

    // Create group on non-existing stream with MKSTREAM
    must_ok!(c, "XGROUP", "CREATE", "nosuch", "g1", "0", "MKSTREAM");
    must_int!(c, "XLEN", "nosuch"; 0);

    // Stream exists now, even though empty
    must_str!(c, "TYPE", "nosuch"; "stream");
}

#[tokio::test]
async fn test_xgroup_destroy() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");
    must_int!(c, "XGROUP", "DESTROY", "s", "g1"; 1);
    must_int!(c, "XGROUP", "DESTROY", "s", "g1"; 0);
}

#[tokio::test]
async fn test_xgroup_createconsumer() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");
    must_int!(c, "XGROUP", "CREATECONSUMER", "s", "g1", "c1"; 1);
    // Already exists
    must_int!(c, "XGROUP", "CREATECONSUMER", "s", "g1", "c1"; 0);
}

#[tokio::test]
async fn test_xgroup_delconsumer() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");
    must_int!(c, "XGROUP", "CREATECONSUMER", "s", "g1", "c1"; 1);
    must_int!(c, "XGROUP", "DELCONSUMER", "s", "g1", "c1"; 0); // 0 pending
}

#[tokio::test]
async fn test_xgroup_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "XGROUP"; "wrong number of arguments");

    // Non-existing stream without MKSTREAM
    must_fail!(c, "XGROUP", "CREATE", "nosuch", "g1", "0"; "requires the key to exist");
}

// ── XREADGROUP ───────────────────────────────────────────────────────

#[tokio::test]
async fn test_xreadgroup() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // Read new entries
    let result: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    assert_eq!(streams.len(), 1);
    let stream = as_array(&streams[0]);
    let entries = as_array(&stream[1]);
    assert_eq!(entries.len(), 3);
}

#[tokio::test]
async fn test_xreadgroup_count() {
    let (_m, mut c) = start().await;

    for i in 1..=5 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // Read with COUNT
    let result: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("COUNT")
        .arg("2")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    let stream = as_array(&streams[0]);
    let entries = as_array(&stream[1]);
    assert_eq!(entries.len(), 2);
}

#[tokio::test]
async fn test_xreadgroup_redelivery() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("v")
        .arg("1")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // First read - new message
    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // Re-read from PEL
    let result: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    let streams = as_array(&result);
    let stream = as_array(&streams[0]);
    let entries = as_array(&stream[1]);
    assert_eq!(entries.len(), 1);
}

#[tokio::test]
async fn test_xreadgroup_nogroup() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_fail!(c, "XREADGROUP", "GROUP", "nosuch", "c1", "STREAMS", "s", ">"; "NOGROUP");
}

// ── XACK ─────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xack() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // Read all
    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // ACK one
    must_int!(c, "XACK", "s", "g1", "1-0"; 1);

    // ACK same again = 0
    must_int!(c, "XACK", "s", "g1", "1-0"; 0);

    // ACK multiple
    must_int!(c, "XACK", "s", "g1", "2-0", "3-0"; 2);
}

// ── XPENDING ─────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xpending_summary() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // Read all
    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // Summary mode
    let result: redis::Value = redis::cmd("XPENDING")
        .arg("s")
        .arg("g1")
        .query_async(&mut c)
        .await
        .unwrap();
    let items = as_array(&result);
    assert_eq!(items.len(), 4);
    // First item: count
    assert_eq!(as_int(&items[0]), 3);
}

#[tokio::test]
async fn test_xpending_detail() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // Detail mode
    let result: redis::Value = redis::cmd("XPENDING")
        .arg("s")
        .arg("g1")
        .arg("-")
        .arg("+")
        .arg("10")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 3);
}

// ── XCLAIM ───────────────────────────────────────────────────────────

#[tokio::test]
async fn test_xclaim() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // c1 reads all
    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // c2 claims 1-0 from c1
    let result: redis::Value = redis::cmd("XCLAIM")
        .arg("s")
        .arg("g1")
        .arg("c2")
        .arg("0")
        .arg("1-0")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 1);
}

#[tokio::test]
async fn test_xclaim_justid() {
    let (_m, mut c) = start().await;

    redis::cmd("XADD")
        .arg("s")
        .arg("1-0")
        .arg("f")
        .arg("v")
        .query_async::<String>(&mut c)
        .await
        .unwrap();

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // JUSTID - return only IDs, not full entries
    let result: redis::Value = redis::cmd("XCLAIM")
        .arg("s")
        .arg("g1")
        .arg("c2")
        .arg("0")
        .arg("1-0")
        .arg("JUSTID")
        .query_async(&mut c)
        .await
        .unwrap();
    let entries = as_array(&result);
    assert_eq!(entries.len(), 1);
    // Should be a string (ID), not an array
    assert!(matches!(entries[0], redis::Value::BulkString(_)));
}

// ── XAUTOCLAIM ───────────────────────────────────────────────────────

#[tokio::test]
async fn test_xautoclaim() {
    let (_m, mut c) = start().await;

    for i in 1..=3 {
        redis::cmd("XADD")
            .arg("s")
            .arg(format!("{}-0", i))
            .arg("v")
            .arg(format!("{}", i))
            .query_async::<String>(&mut c)
            .await
            .unwrap();
    }

    must_ok!(c, "XGROUP", "CREATE", "s", "g1", "0");

    // c1 reads all
    let _: redis::Value = redis::cmd("XREADGROUP")
        .arg("GROUP")
        .arg("g1")
        .arg("c1")
        .arg("STREAMS")
        .arg("s")
        .arg(">")
        .query_async(&mut c)
        .await
        .unwrap();

    // XAUTOCLAIM with 0 min-idle-time (claims all pending)
    let result: redis::Value = redis::cmd("XAUTOCLAIM")
        .arg("s")
        .arg("g1")
        .arg("c2")
        .arg("0")
        .arg("0-0")
        .query_async(&mut c)
        .await
        .unwrap();
    let items = as_array(&result);
    assert!(
        items.len() >= 2,
        "expected at least 2 items (next-id, entries), got {}",
        items.len()
    );

    // Second item: claimed entries
    let entries = as_array(&items[1]);
    assert_eq!(entries.len(), 3);
}

// ── Helper functions ─────────────────────────────────────────────────

fn as_array(v: &redis::Value) -> &Vec<redis::Value> {
    match v {
        redis::Value::Array(a) => a,
        _ => panic!("expected array, got {:?}", v),
    }
}

fn as_string(v: &redis::Value) -> String {
    match v {
        redis::Value::BulkString(b) => String::from_utf8_lossy(b).to_string(),
        redis::Value::SimpleString(s) => s.clone(),
        _ => panic!("expected string, got {:?}", v),
    }
}

fn as_int(v: &redis::Value) -> i64 {
    match v {
        redis::Value::Int(i) => *i,
        _ => panic!("expected int, got {:?}", v),
    }
}
