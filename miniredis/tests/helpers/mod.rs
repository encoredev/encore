use miniredis_rs::Miniredis;
use redis::aio::MultiplexedConnection;

/// Spin up a server + connected client — equivalent to Go's `runWithClient(t)`.
pub async fn start() -> (Miniredis, MultiplexedConnection) {
    let m = Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let conn = client.get_multiplexed_async_connection().await.unwrap();
    (m, conn)
}

/// Spin up a server + two connected clients.
#[allow(dead_code)]
pub async fn start_two_clients() -> (Miniredis, MultiplexedConnection, MultiplexedConnection) {
    let m = Miniredis::run().await.unwrap();
    let c1 = redis::Client::open(m.redis_url())
        .unwrap()
        .get_multiplexed_async_connection()
        .await
        .unwrap();
    let c2 = redis::Client::open(m.redis_url())
        .unwrap()
        .get_multiplexed_async_connection()
        .await
        .unwrap();
    (m, c1, c2)
}

// ── Assertion macros ─────────────────────────────────────────────────

/// Execute a command and assert it returns "OK".
#[macro_export]
macro_rules! must_ok {
    ($conn:expr, $cmd:expr $(, $arg:expr)*) => {{
        let result: String = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await
            .unwrap();
        assert_eq!(result, "OK");
    }};
}

/// Execute a command and assert it returns a specific string.
#[macro_export]
macro_rules! must_str {
    ($conn:expr, $cmd:expr $(, $arg:expr)* ; $expected:expr) => {{
        let result: String = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await
            .unwrap();
        assert_eq!(result, $expected);
    }};
}

/// Execute a command and assert it returns a specific integer.
#[macro_export]
macro_rules! must_int {
    ($conn:expr, $cmd:expr $(, $arg:expr)* ; $expected:expr) => {{
        let result: i64 = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await
            .unwrap();
        assert_eq!(result, $expected as i64);
    }};
}

/// Execute a command and assert it returns nil/null.
#[macro_export]
macro_rules! must_nil {
    ($conn:expr, $cmd:expr $(, $arg:expr)*) => {{
        let result: Option<String> = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await
            .unwrap();
        assert_eq!(result, None);
    }};
}

/// Execute a command and assert it returns an error containing the given substring.
#[macro_export]
macro_rules! must_fail {
    ($conn:expr, $cmd:expr $(, $arg:expr)* ; $expected:expr) => {{
        let result: redis::RedisResult<redis::Value> = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await;
        match result {
            Err(e) => {
                let msg = e.to_string();
                assert!(
                    msg.contains($expected),
                    "error {msg:?} does not contain {:?}",
                    $expected,
                );
            }
            Ok(v) => {
                panic!(
                    "expected error containing {:?}, got {:?}",
                    $expected,
                    v,
                );
            }
        }
    }};
}

/// Execute a command and assert it returns a list of strings (sorted before comparison).
#[macro_export]
macro_rules! must_strs_sorted {
    ($conn:expr, $cmd:expr $(, $arg:expr)* ; $expected:expr) => {{
        let mut result: Vec<String> = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await
            .unwrap();
        result.sort();
        let mut expected: Vec<String> = $expected.iter().map(|s: &&str| s.to_string()).collect();
        expected.sort();
        assert_eq!(result, expected);
    }};
}

/// Execute a command and assert it returns a list of strings (order preserved).
#[macro_export]
macro_rules! must_strs {
    ($conn:expr, $cmd:expr $(, $arg:expr)* ; $expected:expr) => {{
        let result: Vec<String> = redis::cmd($cmd)
            $(.arg($arg))*
            .query_async(&mut $conn)
            .await
            .unwrap();
        let expected: Vec<String> = $expected.iter().map(|s: &&str| s.to_string()).collect();
        assert_eq!(result, expected);
    }};
}

/// Execute a command and assert boolean result (1 = true, 0 = false).
#[macro_export]
macro_rules! must_1 {
    ($conn:expr, $cmd:expr $(, $arg:expr)*) => {
        must_int!($conn, $cmd $(, $arg)* ; 1);
    };
}

#[macro_export]
macro_rules! must_0 {
    ($conn:expr, $cmd:expr $(, $arg:expr)*) => {
        must_int!($conn, $cmd $(, $arg)* ; 0);
    };
}
