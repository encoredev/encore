// Ported from ../miniredis/cmd_generic_test.go
mod helpers;
use std::time::Duration;

#[tokio::test]
async fn test_ttl_expire() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    // No TTL by default
    must_int!(c, "TTL", "foo"; -1);
    must_int!(c, "PTTL", "foo"; -1);
    assert!(m.ttl("foo").is_none());

    // Set TTL with EXPIRE
    must_int!(c, "EXPIRE", "foo", "100"; 1);
    let ttl = m.ttl("foo");
    assert!(ttl.is_some());
    assert!(ttl.unwrap() <= Duration::from_secs(100));

    // TTL command
    let ttl_val: i64 = redis::cmd("TTL")
        .arg("foo")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(ttl_val > 0 && ttl_val <= 100);

    // PTTL command
    let pttl_val: i64 = redis::cmd("PTTL")
        .arg("foo")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(pttl_val > 0 && pttl_val <= 100_000);

    // PERSIST removes TTL
    must_int!(c, "PERSIST", "foo"; 1);
    must_int!(c, "TTL", "foo"; -1);
    assert!(m.ttl("foo").is_none());

    // PERSIST on key without TTL
    must_int!(c, "PERSIST", "foo"; 0);

    // EXPIRE on non-existing key
    must_int!(c, "EXPIRE", "nosuch", "100"; 0);

    // PERSIST on non-existing key
    must_int!(c, "PERSIST", "nosuch"; 0);

    // TTL/PTTL on non-existing key
    must_int!(c, "TTL", "nosuch"; -2);
    must_int!(c, "PTTL", "nosuch"; -2);

    // Errors
    must_fail!(c, "EXPIRE"; "wrong number of arguments");
    must_fail!(c, "EXPIRE", "foo"; "wrong number of arguments");
    must_fail!(c, "TTL"; "wrong number of arguments");
    must_fail!(c, "PTTL"; "wrong number of arguments");
    must_fail!(c, "PERSIST"; "wrong number of arguments");
}

#[tokio::test]
async fn test_expireat() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    // EXPIREAT with future timestamp
    let future_ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
        + 100;
    must_int!(c, "EXPIREAT", "foo", &future_ts.to_string(); 1);

    let ttl = m.ttl("foo");
    assert!(ttl.is_some());

    // Non-existing key
    must_int!(c, "EXPIREAT", "nosuch", &future_ts.to_string(); 0);
}

#[tokio::test]
async fn test_pexpire() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    must_int!(c, "PEXPIRE", "foo", "50000"; 1);
    let ttl = m.ttl("foo");
    assert!(ttl.is_some());
    assert!(ttl.unwrap() <= Duration::from_secs(50));

    // Non-existing key
    must_int!(c, "PEXPIRE", "nosuch", "50000"; 0);
}

#[tokio::test]
async fn test_pexpireat() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    let future_ts_ms = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_millis()
        + 100_000;
    must_int!(c, "PEXPIREAT", "foo", &future_ts_ms.to_string(); 1);

    let ttl = m.ttl("foo");
    assert!(ttl.is_some());
}

#[tokio::test]
async fn test_type() {
    let (_m, mut c) = helpers::start().await;

    // String
    must_ok!(c, "SET", "str", "val");
    let t: String = redis::cmd("TYPE")
        .arg("str")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(t, "string");

    // Hash
    must_int!(c, "HSET", "h", "f", "v"; 1);
    let t: String = redis::cmd("TYPE")
        .arg("h")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(t, "hash");

    // List
    must_int!(c, "RPUSH", "l", "a"; 1);
    let t: String = redis::cmd("TYPE")
        .arg("l")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(t, "list");

    // Set
    must_int!(c, "SADD", "s", "a"; 1);
    let t: String = redis::cmd("TYPE")
        .arg("s")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(t, "set");

    // Non-existing
    let t: String = redis::cmd("TYPE")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(t, "none");

    // Errors
    must_fail!(c, "TYPE"; "wrong number of arguments");
}

#[tokio::test]
async fn test_rename() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "val");

    must_ok!(c, "RENAME", "a", "b");
    must_nil!(c, "GET", "a");
    must_str!(c, "GET", "b"; "val");

    // Overwrite existing
    must_ok!(c, "SET", "c", "other");
    must_ok!(c, "RENAME", "b", "c");
    must_str!(c, "GET", "c"; "val");

    // Non-existing source
    must_fail!(c, "RENAME", "nosuch", "dst"; "no such key");

    // Same key
    must_ok!(c, "SET", "x", "v");
    must_ok!(c, "RENAME", "x", "x");
    must_str!(c, "GET", "x"; "v");

    // Errors
    must_fail!(c, "RENAME"; "wrong number of arguments");
    must_fail!(c, "RENAME", "a"; "wrong number of arguments");
}

#[tokio::test]
async fn test_renamenx() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "val");

    must_int!(c, "RENAMENX", "a", "b"; 1);
    must_nil!(c, "GET", "a");
    must_str!(c, "GET", "b"; "val");

    // Target exists
    must_ok!(c, "SET", "c", "other");
    must_int!(c, "RENAMENX", "b", "c"; 0);
    must_str!(c, "GET", "b"; "val");
    must_str!(c, "GET", "c"; "other");

    // Non-existing source
    must_fail!(c, "RENAMENX", "nosuch", "dst"; "no such key");

    // Errors
    must_fail!(c, "RENAMENX"; "wrong number of arguments");
}

#[tokio::test]
async fn test_keys() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "alpha", "1");
    must_ok!(c, "SET", "beta", "2");
    must_ok!(c, "SET", "gamma", "3");
    must_ok!(c, "SET", "abc", "4");

    // Match all
    let mut result: Vec<String> = redis::cmd("KEYS")
        .arg("*")
        .query_async(&mut c)
        .await
        .unwrap();
    result.sort();
    assert_eq!(result, vec!["abc", "alpha", "beta", "gamma"]);

    // Pattern match
    let mut result: Vec<String> = redis::cmd("KEYS")
        .arg("a*")
        .query_async(&mut c)
        .await
        .unwrap();
    result.sort();
    assert_eq!(result, vec!["abc", "alpha"]);

    // Single character wildcard
    let result: Vec<String> = redis::cmd("KEYS")
        .arg("ab?")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["abc"]);

    // No match
    let result: Vec<String> = redis::cmd("KEYS")
        .arg("nosuch*")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());

    // Errors
    must_fail!(c, "KEYS"; "wrong number of arguments");
}

#[tokio::test]
async fn test_scan() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "key1", "1");
    must_ok!(c, "SET", "key2", "2");
    must_ok!(c, "SET", "other", "3");

    // Basic scan
    let (cursor, mut keys): (String, Vec<String>) = redis::cmd("SCAN")
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    keys.sort();
    assert_eq!(keys, vec!["key1", "key2", "other"]);

    // MATCH pattern
    let (cursor, mut keys): (String, Vec<String>) = redis::cmd("SCAN")
        .arg("0")
        .arg("MATCH")
        .arg("key*")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    keys.sort();
    assert_eq!(keys, vec!["key1", "key2"]);

    // TYPE filter
    must_int!(c, "SADD", "myset", "a"; 1);
    let (cursor, keys): (String, Vec<String>) = redis::cmd("SCAN")
        .arg("0")
        .arg("TYPE")
        .arg("set")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    assert_eq!(keys, vec!["myset"]);
}

#[tokio::test]
async fn test_touch() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");

    must_int!(c, "TOUCH", "a"; 1);
    must_int!(c, "TOUCH", "a", "b"; 2);
    must_int!(c, "TOUCH", "a", "b", "nosuch"; 2);
    must_int!(c, "TOUCH", "nosuch"; 0);

    // Errors
    must_fail!(c, "TOUCH"; "wrong number of arguments");
}

#[tokio::test]
async fn test_unlink() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");

    must_int!(c, "UNLINK", "a", "b", "nosuch"; 2);
    must_nil!(c, "GET", "a");
    must_nil!(c, "GET", "b");
}

#[tokio::test]
async fn test_randomkey() {
    let (_m, mut c) = helpers::start().await;

    // Empty database
    must_nil!(c, "RANDOMKEY");

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");

    // Should return one of the keys
    let key: String = redis::cmd("RANDOMKEY").query_async(&mut c).await.unwrap();
    assert!(key == "a" || key == "b");
}

#[tokio::test]
async fn test_wait() {
    let (_m, mut c) = helpers::start().await;

    // WAIT always returns 0 (standalone mode)
    must_int!(c, "WAIT", "0", "0"; 0);
    must_int!(c, "WAIT", "1", "100"; 0);

    // Errors
    must_fail!(c, "WAIT"; "wrong number of arguments");
}

#[tokio::test]
async fn test_object() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    let enc: String = redis::cmd("OBJECT")
        .arg("ENCODING")
        .arg("foo")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(enc, "raw");

    must_int!(c, "OBJECT", "IDLETIME", "foo"; 0);
    must_int!(c, "OBJECT", "REFCOUNT", "foo"; 1);
    must_int!(c, "OBJECT", "FREQ", "foo"; 0);

    // Errors
    must_fail!(c, "OBJECT"; "wrong number of arguments");
}

#[tokio::test]
async fn test_expire_flags() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    // NX: only if no TTL
    must_int!(c, "EXPIRE", "foo", "100", "NX"; 1);
    assert!(m.ttl("foo").is_some());

    // NX again should fail (TTL already set)
    must_int!(c, "EXPIRE", "foo", "200", "NX"; 0);

    // XX: only if TTL exists
    must_int!(c, "EXPIRE", "foo", "200", "XX"; 1);

    // XX on key without TTL
    must_ok!(c, "SET", "bar", "baz");
    must_int!(c, "EXPIRE", "bar", "100", "XX"; 0);

    // GT: only if new TTL > old
    must_ok!(c, "SET", "gt", "val");
    must_int!(c, "EXPIRE", "gt", "100"; 1);
    must_int!(c, "EXPIRE", "gt", "200", "GT"; 1); // 200 > 100
    must_int!(c, "EXPIRE", "gt", "50", "GT"; 0); // 50 < 200

    // LT: only if new TTL < old
    must_ok!(c, "SET", "lt", "val");
    must_int!(c, "EXPIRE", "lt", "200"; 1);
    must_int!(c, "EXPIRE", "lt", "100", "LT"; 1); // 100 < 200
    must_int!(c, "EXPIRE", "lt", "300", "LT"; 0); // 300 > 100
}

#[tokio::test]
async fn test_copy() {
    let (_m, mut c) = helpers::start().await;

    // Basic
    must_ok!(c, "SET", "key1", "value");
    must_int!(c, "COPY", "key1", "key2"; 1);
    must_str!(c, "GET", "key2"; "value");

    // Nonexistent source
    must_int!(c, "COPY", "nosuch", "to"; 0);

    // Existing destination (no overwrite by default)
    must_ok!(c, "SET", "existingkey", "value");
    must_ok!(c, "SET", "newkey", "newvalue");
    must_int!(c, "COPY", "newkey", "existingkey"; 0);
    must_str!(c, "GET", "existingkey"; "value");

    // REPLACE
    must_ok!(c, "SET", "rkey1", "value");
    must_ok!(c, "SET", "rkey2", "another");
    must_int!(c, "COPY", "rkey1", "rkey2", "REPLACE"; 1);
    must_str!(c, "GET", "rkey2"; "value");

    // List copy (deep copy)
    must_int!(c, "LPUSH", "l1", "original"; 1);
    must_int!(c, "COPY", "l1", "l2"; 1);
    must_int!(c, "LPUSH", "l1", "new"; 2);
    must_int!(c, "LLEN", "l2"; 1);

    // Errors
    must_fail!(c, "COPY"; "wrong number of arguments");
    must_fail!(c, "COPY", "foo"; "wrong number of arguments");
}

#[tokio::test]
async fn test_move_cmd() {
    let (_m, mut c) = helpers::start().await;

    // Basic
    must_ok!(c, "SET", "foo", "bar!");
    must_int!(c, "MOVE", "foo", "1"; 1);
    must_nil!(c, "GET", "foo"); // Gone from DB 0

    // Source doesn't exist
    must_int!(c, "MOVE", "nosuch", "1"; 0);

    // Errors
    must_fail!(c, "MOVE"; "wrong number of arguments");
    must_fail!(c, "MOVE", "foo"; "wrong number of arguments");
    must_fail!(c, "MOVE", "foo", "noint"; "DB index is out of range");
}

#[tokio::test]
async fn test_expiretime() {
    let (_m, mut c) = helpers::start().await;

    // Nonexistent key
    must_int!(c, "EXPIRETIME", "nosuch"; -2);

    // No expire
    must_ok!(c, "SET", "noexpire", "");
    must_int!(c, "EXPIRETIME", "noexpire"; -1);

    // With expire
    must_ok!(c, "SET", "foo", "");
    must_int!(c, "EXPIREAT", "foo", "10413792000"; 1);
    must_int!(c, "EXPIRETIME", "foo"; 10413792000);
}

#[tokio::test]
async fn test_pexpiretime() {
    let (_m, mut c) = helpers::start().await;

    // Nonexistent key
    must_int!(c, "PEXPIRETIME", "nosuch"; -2);

    // No expire
    must_ok!(c, "SET", "noexpire", "");
    must_int!(c, "PEXPIRETIME", "noexpire"; -1);

    // With expire
    must_ok!(c, "SET", "foo", "");
    must_int!(c, "PEXPIREAT", "foo", "10413792000123"; 1);
    must_int!(c, "PEXPIRETIME", "foo"; 10413792000123);
}

#[tokio::test]
async fn test_dump() {
    let (_m, mut c) = helpers::start().await;

    // Missing key
    must_nil!(c, "DUMP", "missing-key");

    // Existing key (stub returns raw string)
    must_ok!(c, "SET", "existing-key", "value");
    must_str!(c, "DUMP", "existing-key"; "value");

    // Non-string type returns nil (stub behavior)
    let _: i64 = redis::cmd("HSET")
        .arg("set-key")
        .arg("a")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    must_nil!(c, "DUMP", "set-key");

    // Errors
    must_fail!(c, "DUMP"; "wrong number of arguments");
}

#[tokio::test]
async fn test_restore() {
    let (_m, mut c) = helpers::start().await;

    // New key no TTL
    must_ok!(c, "RESTORE", "key-a", "0", "value-a");
    must_str!(c, "GET", "key-a"; "value-a");

    // Busy key
    must_ok!(c, "SET", "existing", "value");
    must_fail!(c, "RESTORE", "existing", "0", "other"; "BUSYKEY");

    // Overwrite with REPLACE
    must_ok!(c, "RESTORE", "existing", "0", "new-value", "REPLACE");
    must_str!(c, "GET", "existing"; "new-value");

    // Errors
    must_fail!(c, "RESTORE"; "wrong number of arguments");
    must_fail!(c, "RESTORE", "key"; "wrong number of arguments");
    must_fail!(c, "RESTORE", "key", "argh", "val"; "not an integer");
}
