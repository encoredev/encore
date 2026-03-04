mod helpers;
use helpers::*;

// ── AUTH ──────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_auth_no_password_configured() {
    let (_m, mut c) = start().await;

    // AUTH without any password configured → specific error
    must_fail!(c, "AUTH", "foo"; "AUTH <password> called without any password configured");
}

#[tokio::test]
async fn test_auth_wrong_args() {
    let (_m, mut c) = start().await;

    // No args
    must_fail!(c, "AUTH"; "wrong number of arguments");

    // Too many args
    must_fail!(c, "AUTH", "a", "b", "c"; "syntax error");
}

#[tokio::test]
async fn test_auth_default_user() {
    let (m, mut c) = start().await;

    m.require_auth("secret");

    // Without auth, commands should fail
    must_fail!(c, "PING"; "NOAUTH");

    // Wrong password
    must_fail!(c, "AUTH", "wrongpass"; "WRONGPASS");

    // Correct password
    must_ok!(c, "AUTH", "secret");

    // Now commands work
    let v: String = redis::cmd("PING").query_async(&mut c).await.unwrap();
    assert_eq!(v, "PONG");
}

#[tokio::test]
async fn test_auth_user_password() {
    let (m, mut c) = start().await;

    m.require_user_auth("hello", "world");

    // Without auth, commands should fail
    must_fail!(c, "PING"; "NOAUTH");

    // Wrong password
    must_fail!(c, "AUTH", "hello", "wrongpass"; "WRONGPASS");

    // Wrong username
    must_fail!(c, "AUTH", "goodbye", "world"; "WRONGPASS");

    // Correct user + password
    must_ok!(c, "AUTH", "hello", "world");

    // Now commands work
    let v: String = redis::cmd("PING").query_async(&mut c).await.unwrap();
    assert_eq!(v, "PONG");
}

// ── HELLO ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_hello_basic() {
    let (_m, mut c) = start().await;

    // HELLO 2 should return server info
    let v: redis::Value = redis::cmd("HELLO")
        .arg("2")
        .query_async(&mut c)
        .await
        .unwrap();

    // Should be an array with key-value pairs
    match v {
        redis::Value::Array(ref items) => {
            assert!(items.len() >= 12); // at least 6 key-value pairs
        }
        _ => panic!("expected array from HELLO, got {:?}", v),
    }
}

#[tokio::test]
async fn test_hello_errors() {
    let (_m, mut c) = start().await;

    // No args
    must_fail!(c, "HELLO"; "wrong number of arguments");

    // Non-integer version
    must_fail!(c, "HELLO", "foo"; "Protocol version is not an integer");

    // Unsupported version
    must_fail!(c, "HELLO", "1"; "NOPROTO");
    must_fail!(c, "HELLO", "4"; "NOPROTO");
}

#[tokio::test]
async fn test_hello_auth() {
    let (m, mut c) = start().await;

    m.require_auth("secret");

    // HELLO with AUTH should authenticate
    let v: redis::Value = redis::cmd("HELLO")
        .arg("2")
        .arg("AUTH")
        .arg("default")
        .arg("secret")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(_) => {} // success
        _ => panic!("expected array from HELLO, got {:?}", v),
    }

    // Should be authenticated now
    let v: String = redis::cmd("PING").query_async(&mut c).await.unwrap();
    assert_eq!(v, "PONG");
}

#[tokio::test]
async fn test_hello_auth_wrong_password() {
    let (m, mut c) = start().await;

    m.require_auth("secret");

    // HELLO with wrong AUTH should fail
    must_fail!(c, "HELLO", "2", "AUTH", "default", "wrong"; "WRONGPASS");
}

#[tokio::test]
async fn test_hello_syntax_errors() {
    let (_m, mut c) = start().await;

    // AUTH with missing args
    must_fail!(c, "HELLO", "2", "AUTH", "foo"; "Syntax error in HELLO option");

    // SETNAME with missing arg
    must_fail!(c, "HELLO", "2", "AUTH", "foo", "bar", "SETNAME"; "Syntax error in HELLO option");

    // Unknown option
    must_fail!(c, "HELLO", "2", "BOGUS"; "Syntax error in HELLO option");
}
