// Ported from ../miniredis/cmd_string_test.go
mod helpers;
use std::time::Duration;

#[tokio::test]
async fn test_set() {
    let (m, mut c) = helpers::start().await;

    // Basic SET/GET
    must_ok!(c, "SET", "foo", "bar");
    must_str!(c, "GET", "foo"; "bar");
    m.check_get("foo", "bar");

    // Overwrite
    must_ok!(c, "SET", "foo", "baz");
    must_str!(c, "GET", "foo"; "baz");

    // Non-existent
    must_nil!(c, "GET", "nosuch");

    // Wrong number of args
    must_fail!(c, "SET"; "wrong number of arguments");
    must_fail!(c, "SET", "foo"; "wrong number of arguments");
    must_fail!(c, "GET"; "wrong number of arguments");
}

#[tokio::test]
async fn test_set_nx() {
    let (_m, mut c) = helpers::start().await;

    // NX: set only if not exists
    must_ok!(c, "SET", "foo", "bar", "NX");
    must_str!(c, "GET", "foo"; "bar");

    // Second NX should fail (return nil)
    must_nil!(c, "SET", "foo", "baz", "NX");
    // Value unchanged
    must_str!(c, "GET", "foo"; "bar");
}

#[tokio::test]
async fn test_set_xx() {
    let (_m, mut c) = helpers::start().await;

    // XX: set only if exists — key doesn't exist yet
    must_nil!(c, "SET", "foo", "bar", "XX");
    must_nil!(c, "GET", "foo");

    // Now create it
    must_ok!(c, "SET", "foo", "bar");
    // XX should work now
    must_ok!(c, "SET", "foo", "baz", "XX");
    must_str!(c, "GET", "foo"; "baz");
}

#[tokio::test]
async fn test_set_ex() {
    let (m, mut c) = helpers::start().await;

    // SET with EX
    must_ok!(c, "SET", "foo", "bar", "EX", "10");

    // TTL should be set
    let ttl = m.ttl("foo");
    assert!(ttl.is_some());
    assert!(ttl.unwrap() <= Duration::from_secs(10));

    // Invalid EX
    must_fail!(c, "SET", "foo", "bar", "EX", "0"; "invalid expire time");
    must_fail!(c, "SET", "foo", "bar", "EX", "-1"; "invalid expire time");
    must_fail!(c, "SET", "foo", "bar", "EX", "notanumber"; "not an integer");
}

#[tokio::test]
async fn test_set_px() {
    let (m, mut c) = helpers::start().await;

    // SET with PX
    must_ok!(c, "SET", "foo", "bar", "PX", "10000");

    let ttl = m.ttl("foo");
    assert!(ttl.is_some());
    assert!(ttl.unwrap() <= Duration::from_secs(10));

    // Invalid PX
    must_fail!(c, "SET", "foo", "bar", "PX", "0"; "invalid expire time");
    must_fail!(c, "SET", "foo", "bar", "PX", "-1"; "invalid expire time");
}

#[tokio::test]
async fn test_set_keepttl() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar", "EX", "100");
    // Overwrite with KEEPTTL
    must_ok!(c, "SET", "foo", "baz", "KEEPTTL");

    must_str!(c, "GET", "foo"; "baz");
    let ttl = m.ttl("foo");
    assert!(ttl.is_some(), "TTL should be preserved");
}

#[tokio::test]
async fn test_set_get() {
    let (_m, mut c) = helpers::start().await;

    // GET option — return old value
    must_nil!(c, "SET", "foo", "bar", "GET");

    must_str!(c, "SET", "foo", "baz", "GET"; "bar");
    must_str!(c, "GET", "foo"; "baz");
}

#[tokio::test]
async fn test_setnx() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SETNX", "foo", "bar"; 1);
    must_str!(c, "GET", "foo"; "bar");

    must_int!(c, "SETNX", "foo", "baz"; 0);
    must_str!(c, "GET", "foo"; "bar");

    // Wrong number of args
    must_fail!(c, "SETNX"; "wrong number of arguments");
    must_fail!(c, "SETNX", "foo"; "wrong number of arguments");
}

#[tokio::test]
async fn test_getset() {
    let (_m, mut c) = helpers::start().await;

    must_nil!(c, "GETSET", "foo", "bar");
    must_str!(c, "GETSET", "foo", "baz"; "bar");
    must_str!(c, "GET", "foo"; "baz");

    // Wrong number of args
    must_fail!(c, "GETSET"; "wrong number of arguments");
    must_fail!(c, "GETSET", "foo"; "wrong number of arguments");
}

#[tokio::test]
async fn test_del() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");
    must_ok!(c, "SET", "c", "3");

    // DEL multiple
    must_int!(c, "DEL", "a", "b"; 2);
    must_nil!(c, "GET", "a");
    must_nil!(c, "GET", "b");
    must_str!(c, "GET", "c"; "3");

    // DEL non-existent
    must_int!(c, "DEL", "nosuch"; 0);

    // Wrong number of args
    must_fail!(c, "DEL"; "wrong number of arguments");
}

#[tokio::test]
async fn test_exists() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");

    must_int!(c, "EXISTS", "a"; 1);
    must_int!(c, "EXISTS", "nosuch"; 0);

    // Multiple keys
    must_int!(c, "EXISTS", "a", "b", "nosuch"; 2);

    // Wrong number of args
    must_fail!(c, "EXISTS"; "wrong number of arguments");
}

#[tokio::test]
async fn test_setex() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SETEX", "foo", "10", "bar");
    must_str!(c, "GET", "foo"; "bar");
    let ttl = m.ttl("foo");
    assert!(ttl.is_some());

    // Errors
    must_fail!(c, "SETEX", "foo", "0", "bar"; "invalid expire time");
    must_fail!(c, "SETEX", "foo", "-1", "bar"; "invalid expire time");
    must_fail!(c, "SETEX"; "wrong number of arguments");
}

#[tokio::test]
async fn test_psetex() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "PSETEX", "foo", "10000", "bar");
    must_str!(c, "GET", "foo"; "bar");
    let ttl = m.ttl("foo");
    assert!(ttl.is_some());

    must_fail!(c, "PSETEX", "foo", "0", "bar"; "invalid expire time");
}

#[tokio::test]
async fn test_incr_decr() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "INCR", "counter"; 1);
    must_int!(c, "INCR", "counter"; 2);
    must_int!(c, "INCRBY", "counter", "5"; 7);
    must_int!(c, "DECR", "counter"; 6);
    must_int!(c, "DECRBY", "counter", "3"; 3);

    // Non-integer value
    must_ok!(c, "SET", "str", "notanumber");
    must_fail!(c, "INCR", "str"; "not an integer");

    // Errors
    must_fail!(c, "INCR"; "wrong number of arguments");
    must_fail!(c, "INCRBY"; "wrong number of arguments");
}

#[tokio::test]
async fn test_incrbyfloat() {
    let (_m, mut c) = helpers::start().await;

    must_str!(c, "INCRBYFLOAT", "f", "1.5"; "1.5");
    must_str!(c, "INCRBYFLOAT", "f", "2.5"; "4");
    must_str!(c, "INCRBYFLOAT", "f", "-1"; "3");
}

#[tokio::test]
async fn test_mget_mset() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "MSET", "a", "1", "b", "2", "c", "3");
    must_strs!(c, "MGET", "a", "b", "c"; ["1", "2", "3"]);

    // Missing key returns nil
    let result: Vec<Option<String>> = redis::cmd("MGET")
        .arg("a")
        .arg("nosuch")
        .arg("c")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(
        result,
        vec![Some("1".to_string()), None, Some("3".to_string())]
    );
}

#[tokio::test]
async fn test_msetnx() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "MSETNX", "a", "1", "b", "2"; 1);
    must_str!(c, "GET", "a"; "1");

    // Any key exists → all fail
    must_int!(c, "MSETNX", "a", "x", "c", "3"; 0);
    must_str!(c, "GET", "a"; "1");
    must_nil!(c, "GET", "c"); // c was NOT set
}

#[tokio::test]
async fn test_strlen() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "hello");
    must_int!(c, "STRLEN", "foo"; 5);
    must_int!(c, "STRLEN", "nosuch"; 0);
}

#[tokio::test]
async fn test_append() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "APPEND", "key", "hello"; 5);
    must_int!(c, "APPEND", "key", " world"; 11);
    must_str!(c, "GET", "key"; "hello world");
}

#[tokio::test]
async fn test_getrange() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "hello world");
    must_str!(c, "GETRANGE", "foo", "0", "4"; "hello");
    must_str!(c, "GETRANGE", "foo", "-5", "-1"; "world");
    must_str!(c, "GETRANGE", "foo", "0", "-1"; "hello world");
}

#[tokio::test]
async fn test_setrange() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "hello world");
    must_int!(c, "SETRANGE", "foo", "6", "Redis"; 11);
    must_str!(c, "GET", "foo"; "hello Redis");

    // Extending
    must_int!(c, "SETRANGE", "bar", "5", "hi"; 7);
    // Should be zero-padded
    let val: Vec<u8> = redis::cmd("GET")
        .arg("bar")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(val, vec![0, 0, 0, 0, 0, b'h', b'i']);
}

#[tokio::test]
async fn test_getdel() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");
    must_str!(c, "GETDEL", "foo"; "bar");
    must_nil!(c, "GET", "foo");

    // Non-existent
    must_nil!(c, "GETDEL", "nosuch");
}

#[tokio::test]
async fn test_getex() {
    let (m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "foo", "bar");

    // GETEX with EX
    must_str!(c, "GETEX", "foo", "EX", "100"; "bar");
    assert!(m.ttl("foo").is_some());

    // GETEX with PERSIST
    must_str!(c, "GETEX", "foo", "PERSIST"; "bar");
    assert!(m.ttl("foo").is_none());

    // Non-existent
    must_nil!(c, "GETEX", "nosuch");
}
