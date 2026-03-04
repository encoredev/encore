// Ported from ../miniredis/cmd_connection_test.go
mod helpers;

#[tokio::test]
async fn test_ping() {
    let (_m, mut c) = helpers::start().await;

    // No args → PONG
    must_str!(c, "PING"; "PONG");

    // With arg → echo
    must_str!(c, "PING", "hi"; "hi");

    // Too many args → error
    must_fail!(c, "PING", "foo", "bar"; "wrong number of arguments");
}

#[tokio::test]
async fn test_echo() {
    let (_m, mut c) = helpers::start().await;

    must_str!(c, "ECHO", "hello\nworld"; "hello\nworld");

    // Wrong number of args
    must_fail!(c, "ECHO"; "wrong number of arguments");
}

#[tokio::test]
async fn test_select() {
    let (m, mut c) = helpers::start().await;

    // Set in db 0
    must_ok!(c, "SET", "foo", "bar");
    // Switch to db 5, set different value
    must_ok!(c, "SELECT", "5");
    must_ok!(c, "SET", "foo", "baz");

    // Direct API: db 0 should have "bar"
    assert_eq!(m.get("foo"), Some("bar".to_owned()));

    // SELECT out of range
    must_fail!(c, "SELECT", "16"; "DB index is out of range");
    must_fail!(c, "SELECT", "-1"; "DB index is out of range");
    must_fail!(c, "SELECT", "notanumber"; "DB index is out of range");

    // Wrong number of args
    must_fail!(c, "SELECT"; "wrong number of arguments");
}

#[tokio::test]
async fn test_dbsize() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "DBSIZE"; 0);
    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");
    must_int!(c, "DBSIZE"; 2);
}

#[tokio::test]
async fn test_flushdb() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "a", "1");
    must_ok!(c, "SET", "b", "2");
    must_int!(c, "DBSIZE"; 2);
    must_ok!(c, "FLUSHDB");
    must_int!(c, "DBSIZE"; 0);
}
