// Ported from ../miniredis/cmd_list_test.go
mod helpers;

#[tokio::test]
async fn test_lpush_rpush() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "LPUSH", "l", "a"; 1);
    must_int!(c, "LPUSH", "l", "b"; 2);
    must_int!(c, "RPUSH", "l", "c"; 3);

    // List should be: b, a, c
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["b", "a", "c"]);

    // Multiple values
    must_int!(c, "RPUSH", "l2", "a", "b", "c"; 3);
    must_strs!(c, "LRANGE", "l2", "0", "-1"; ["a", "b", "c"]);

    must_int!(c, "LPUSH", "l3", "c", "b", "a"; 3);
    must_strs!(c, "LRANGE", "l3", "0", "-1"; ["a", "b", "c"]);

    // Errors
    must_fail!(c, "LPUSH"; "wrong number of arguments");
    must_fail!(c, "LPUSH", "key"; "wrong number of arguments");
}

#[tokio::test]
async fn test_lpop_rpop() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "c"; 3);

    must_str!(c, "LPOP", "l"; "a");
    must_str!(c, "RPOP", "l"; "c");
    must_str!(c, "LPOP", "l"; "b");

    // Empty list
    must_nil!(c, "LPOP", "l");
    must_nil!(c, "RPOP", "l");

    // Non-existent key
    must_nil!(c, "LPOP", "nosuch");
}

#[tokio::test]
async fn test_lpop_count() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "c", "d", "e"; 5);

    must_strs!(c, "LPOP", "l", "3"; ["a", "b", "c"]);
    must_strs!(c, "LPOP", "l", "10"; ["d", "e"]);
}

#[tokio::test]
async fn test_lpushx_rpushx() {
    let (_m, mut c) = helpers::start().await;

    // PUSHX on non-existing key
    must_int!(c, "LPUSHX", "l", "a"; 0);
    must_int!(c, "RPUSHX", "l", "a"; 0);

    // Create the list
    must_int!(c, "RPUSH", "l", "a"; 1);

    // Now PUSHX works
    must_int!(c, "LPUSHX", "l", "b"; 2);
    must_int!(c, "RPUSHX", "l", "c"; 3);

    must_strs!(c, "LRANGE", "l", "0", "-1"; ["b", "a", "c"]);
}

#[tokio::test]
async fn test_llen() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "LLEN", "l"; 0);
    must_int!(c, "RPUSH", "l", "a", "b", "c"; 3);
    must_int!(c, "LLEN", "l"; 3);
}

#[tokio::test]
async fn test_lindex() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "c"; 3);

    must_str!(c, "LINDEX", "l", "0"; "a");
    must_str!(c, "LINDEX", "l", "1"; "b");
    must_str!(c, "LINDEX", "l", "2"; "c");
    must_str!(c, "LINDEX", "l", "-1"; "c");
    must_str!(c, "LINDEX", "l", "-2"; "b");

    // Out of range
    must_nil!(c, "LINDEX", "l", "100");
    must_nil!(c, "LINDEX", "l", "-100");

    // Non-existent key
    must_nil!(c, "LINDEX", "nosuch", "0");
}

#[tokio::test]
async fn test_lrange() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "c", "d", "e"; 5);

    must_strs!(c, "LRANGE", "l", "0", "-1"; ["a", "b", "c", "d", "e"]);
    must_strs!(c, "LRANGE", "l", "1", "3"; ["b", "c", "d"]);
    must_strs!(c, "LRANGE", "l", "-3", "-1"; ["c", "d", "e"]);
    must_strs!(c, "LRANGE", "l", "0", "100"; ["a", "b", "c", "d", "e"]);

    // Empty result
    let result: Vec<String> = redis::cmd("LRANGE")
        .arg("l")
        .arg("10")
        .arg("20")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());

    // Non-existent key
    let result: Vec<String> = redis::cmd("LRANGE")
        .arg("nosuch")
        .arg("0")
        .arg("-1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());
}

#[tokio::test]
async fn test_lset() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "c"; 3);

    must_ok!(c, "LSET", "l", "1", "B");
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["a", "B", "c"]);

    // Negative index
    must_ok!(c, "LSET", "l", "-1", "C");
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["a", "B", "C"]);

    // Errors
    must_fail!(c, "LSET", "l", "100", "x"; "index out of range");
    must_fail!(c, "LSET", "nosuch", "0", "x"; "no such key");
}

#[tokio::test]
async fn test_linsert() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "c"; 2);

    must_int!(c, "LINSERT", "l", "BEFORE", "c", "b"; 3);
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["a", "b", "c"]);

    must_int!(c, "LINSERT", "l", "AFTER", "c", "d"; 4);
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["a", "b", "c", "d"]);

    // Pivot not found
    must_int!(c, "LINSERT", "l", "BEFORE", "nosuch", "x"; -1);

    // Non-existent key
    must_int!(c, "LINSERT", "nosuch", "BEFORE", "a", "x"; 0);
}

#[tokio::test]
async fn test_lrem() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "a", "c", "a"; 5);

    // Remove 2 from head
    must_int!(c, "LREM", "l", "2", "a"; 2);
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["b", "c", "a"]);

    // Remove from tail
    must_int!(c, "RPUSH", "l2", "a", "b", "a", "c", "a"; 5);
    must_int!(c, "LREM", "l2", "-2", "a"; 2);
    must_strs!(c, "LRANGE", "l2", "0", "-1"; ["a", "b", "c"]);

    // Remove all
    must_int!(c, "RPUSH", "l3", "a", "a", "a"; 3);
    must_int!(c, "LREM", "l3", "0", "a"; 3);
    must_int!(c, "LLEN", "l3"; 0);
}

#[tokio::test]
async fn test_ltrim() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "l", "a", "b", "c", "d", "e"; 5);

    must_ok!(c, "LTRIM", "l", "1", "3");
    must_strs!(c, "LRANGE", "l", "0", "-1"; ["b", "c", "d"]);
}

#[tokio::test]
async fn test_rpoplpush() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "src", "a", "b", "c"; 3);

    must_str!(c, "RPOPLPUSH", "src", "dst"; "c");
    must_strs!(c, "LRANGE", "src", "0", "-1"; ["a", "b"]);
    must_strs!(c, "LRANGE", "dst", "0", "-1"; ["c"]);

    // Empty source
    must_int!(c, "RPUSH", "empty", "x"; 1);
    must_str!(c, "LPOP", "empty"; "x");
    must_nil!(c, "RPOPLPUSH", "empty", "dst");
}

#[tokio::test]
async fn test_lmove() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "RPUSH", "src", "a", "b", "c"; 3);

    must_str!(c, "LMOVE", "src", "dst", "RIGHT", "LEFT"; "c");
    must_strs!(c, "LRANGE", "src", "0", "-1"; ["a", "b"]);
    must_strs!(c, "LRANGE", "dst", "0", "-1"; ["c"]);

    must_str!(c, "LMOVE", "src", "dst", "LEFT", "RIGHT"; "a");
    must_strs!(c, "LRANGE", "src", "0", "-1"; ["b"]);
    must_strs!(c, "LRANGE", "dst", "0", "-1"; ["c", "a"]);
}

#[tokio::test]
async fn test_list_wrongtype() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "LPUSH", "str", "x"; "WRONGTYPE");
    must_fail!(c, "LLEN", "str"; "WRONGTYPE");
    must_fail!(c, "LRANGE", "str", "0", "-1"; "WRONGTYPE");
}

#[tokio::test]
async fn test_lpos() {
    let (_m, mut c) = helpers::start().await;

    // Build list: [aap, aap, vuur, aap, mies, aap, noot, aap]
    // RPUSH to get them in left-to-right order
    let _: i64 = redis::cmd("RPUSH")
        .arg("l")
        .arg("aap")
        .arg("aap")
        .arg("vuur")
        .arg("aap")
        .arg("mies")
        .arg("aap")
        .arg("noot")
        .arg("aap")
        .query_async(&mut c)
        .await
        .unwrap();

    // Simple
    must_int!(c, "LPOS", "l", "aap"; 0);
    must_int!(c, "LPOS", "l", "vuur"; 2);
    must_int!(c, "LPOS", "l", "mies"; 4);
    must_nil!(c, "LPOS", "l", "wim");

    // RANK
    must_int!(c, "LPOS", "l", "aap", "RANK", "1"; 0);
    must_int!(c, "LPOS", "l", "aap", "RANK", "2"; 1);
    must_int!(c, "LPOS", "l", "aap", "RANK", "3"; 3);
    must_int!(c, "LPOS", "l", "aap", "RANK", "-1"; 7);
    must_int!(c, "LPOS", "l", "aap", "RANK", "-2"; 5);

    // COUNT
    let vals: Vec<i64> = redis::cmd("LPOS")
        .arg("l")
        .arg("aap")
        .arg("COUNT")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals, vec![0, 1, 3, 5, 7]);

    let vals: Vec<i64> = redis::cmd("LPOS")
        .arg("l")
        .arg("aap")
        .arg("COUNT")
        .arg(3)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals, vec![0, 1, 3]);

    let vals: Vec<i64> = redis::cmd("LPOS")
        .arg("l")
        .arg("wim")
        .arg("COUNT")
        .arg(1)
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(vals.is_empty());

    // RANK + COUNT
    let vals: Vec<i64> = redis::cmd("LPOS")
        .arg("l")
        .arg("aap")
        .arg("RANK")
        .arg(3)
        .arg("COUNT")
        .arg(2)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals, vec![3, 5]);

    // COUNT + MAXLEN
    let vals: Vec<i64> = redis::cmd("LPOS")
        .arg("l")
        .arg("aap")
        .arg("COUNT")
        .arg(0)
        .arg("MAXLEN")
        .arg(4)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals, vec![0, 1, 3]);

    // Errors
    must_fail!(c, "LPOS", "l"; "wrong number of arguments");
    must_fail!(c, "LPOS", "l", "aap", "RANK"; "syntax error");
    must_fail!(c, "LPOS", "l", "aap", "RANK", "0"; "can't be zero");
    must_fail!(c, "LPOS", "l", "aap", "COUNT", "-1"; "can't be negative");
    must_fail!(c, "LPOS", "l", "aap", "MAXLEN", "-1"; "can't be negative");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "LPOS", "str", "val"; "WRONGTYPE");
}

#[tokio::test]
async fn test_blpop() {
    let (_m, mut c) = helpers::start().await;

    // Non-blocking: data available
    let _: i64 = redis::cmd("RPUSH")
        .arg("ll")
        .arg("aap")
        .arg("noot")
        .arg("mies")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<String> = redis::cmd("BLPOP")
        .arg("ll")
        .arg(1)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["ll", "aap"]);

    // Timeout (short)
    let result: Option<Vec<String>> = redis::cmd("BLPOP")
        .arg("empty")
        .arg("0.1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_none());

    // Errors
    must_fail!(c, "BLPOP"; "wrong number of arguments");
    must_fail!(c, "BLPOP", "key"; "wrong number of arguments");
    must_fail!(c, "BLPOP", "key", "-1"; "out of range");
}

#[tokio::test]
async fn test_brpop() {
    let (_m, mut c) = helpers::start().await;

    // Non-blocking: data available
    let _: i64 = redis::cmd("RPUSH")
        .arg("ll")
        .arg("aap")
        .arg("noot")
        .arg("mies")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<String> = redis::cmd("BRPOP")
        .arg("ll")
        .arg(1)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["ll", "mies"]);

    // Timeout (short)
    let result: Option<Vec<String>> = redis::cmd("BRPOP")
        .arg("empty")
        .arg("0.1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_none());

    // Errors
    must_fail!(c, "BRPOP"; "wrong number of arguments");
    must_fail!(c, "BRPOP", "key"; "wrong number of arguments");
    must_fail!(c, "BRPOP", "key", "-1"; "out of range");
}

#[tokio::test]
async fn test_brpoplpush() {
    let (_m, mut c) = helpers::start().await;

    // Non-blocking: data available
    let _: i64 = redis::cmd("RPUSH")
        .arg("l1")
        .arg("aap")
        .arg("noot")
        .arg("mies")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: String = redis::cmd("BRPOPLPUSH")
        .arg("l1")
        .arg("l2")
        .arg(1)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, "mies");
    must_strs!(c, "LRANGE", "l2", "0", "-1"; ["mies"]);

    // Timeout (short)
    let result: Option<String> = redis::cmd("BRPOPLPUSH")
        .arg("empty")
        .arg("dst")
        .arg("0.1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_none());

    // Errors
    must_fail!(c, "BRPOPLPUSH"; "wrong number of arguments");
    must_fail!(c, "BRPOPLPUSH", "key"; "wrong number of arguments");
    must_fail!(c, "BRPOPLPUSH", "key", "bar"; "wrong number of arguments");
    must_fail!(c, "BRPOPLPUSH", "key", "foo", "-1"; "out of range");
}

#[tokio::test]
async fn test_blmove() {
    let (_m, mut c) = helpers::start().await;

    // Setup
    let _: i64 = redis::cmd("RPUSH")
        .arg("src")
        .arg("RL")
        .arg("RR")
        .arg("LL")
        .arg("LR")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: i64 = redis::cmd("RPUSH")
        .arg("dst")
        .arg("m1")
        .arg("m2")
        .arg("m3")
        .query_async(&mut c)
        .await
        .unwrap();

    // RIGHT LEFT
    let v: String = redis::cmd("BLMOVE")
        .arg("src")
        .arg("dst")
        .arg("RIGHT")
        .arg("LEFT")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "LR");

    // LEFT RIGHT
    let v: String = redis::cmd("BLMOVE")
        .arg("src")
        .arg("dst")
        .arg("LEFT")
        .arg("RIGHT")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "RL");

    // Timeout (short)
    let result: Option<String> = redis::cmd("BLMOVE")
        .arg("nosuch")
        .arg("dst")
        .arg("RIGHT")
        .arg("LEFT")
        .arg("0.1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_none());

    // Errors
    must_fail!(c, "BLMOVE"; "wrong number of arguments");
    must_fail!(c, "BLMOVE", "l"; "wrong number of arguments");
    must_fail!(c, "BLMOVE", "l", "l"; "wrong number of arguments");
    must_fail!(c, "BLMOVE", "l", "l", "l"; "wrong number of arguments");
    must_fail!(c, "BLMOVE", "l", "l", "l", "l"; "wrong number of arguments");
}

#[tokio::test]
async fn test_brpop_tx() {
    // BRPOP in a transaction behaves as if the timeout triggers right away
    let (m, mut c) = helpers::start().await;

    // BRPOP on empty list inside MULTI → null (no blocking)
    must_ok!(c, "MULTI");
    let v: String = redis::cmd("BRPOP")
        .arg("l1")
        .arg(3)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");
    let v: String = redis::cmd("SET")
        .arg("foo")
        .arg("bar")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");
    let v: redis::Value = redis::cmd("EXEC").query_async(&mut c).await.unwrap();
    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 2);
            assert_eq!(items[0], redis::Value::Nil);
        }
        _ => panic!("expected array from EXEC, got {:?}", v),
    }

    // Now push something and BRPOP in MULTI → should pop it
    m.push("l1", &["e1"]);
    must_ok!(c, "MULTI");
    let v: String = redis::cmd("BRPOP")
        .arg("l1")
        .arg(3)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");
    let v: String = redis::cmd("SET")
        .arg("foo")
        .arg("bar")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "QUEUED");
    let v: redis::Value = redis::cmd("EXEC").query_async(&mut c).await.unwrap();
    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 2);
            // First result: [key, value]
            match &items[0] {
                redis::Value::Array(kv) => {
                    assert_eq!(kv.len(), 2);
                }
                _ => panic!("expected array from BRPOP result, got {:?}", items[0]),
            }
        }
        _ => panic!("expected array from EXEC, got {:?}", v),
    }
}

#[tokio::test]
async fn test_blpop_resource_cleanup() {
    // Test that a blocking BLPOP is cleaned up when the connection is closed.
    // Ensures the server doesn't leak resources or hang.
    let m = miniredis_rs::Miniredis::run().await.unwrap();

    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut conn = client.get_multiplexed_async_connection().await.unwrap();

    // Issue a short-timeout BLPOP on a non-existent key
    let result: Option<(String, String)> = redis::cmd("BLPOP")
        .arg("nonexistent")
        .arg("0.1")
        .query_async(&mut conn)
        .await
        .unwrap();
    assert!(result.is_none(), "BLPOP should timeout with nil");

    // Drop the connection and close the server — should not hang
    drop(conn);
    m.close().await;
}

#[tokio::test]
async fn test_rpush_pop() {
    let (m, mut c) = helpers::start().await;

    // RPUSH
    must_int!(c, "RPUSH", "l", "aap", "noot", "mies"; 3);

    must_strs!(c, "LRANGE", "l", "0", "0"; ["aap"]);
    must_strs!(c, "LRANGE", "l", "-1", "-1"; ["mies"]);

    // Push more
    must_int!(c, "RPUSH", "l", "aap2", "noot2", "mies2"; 6);

    must_strs!(c, "LRANGE", "l", "0", "0"; ["aap"]);
    must_strs!(c, "LRANGE", "l", "-1", "-1"; ["mies2"]);

    // Direct API: Push and Pop
    let len = m.push("l2", &["a"]);
    assert_eq!(len, 1);
    let len = m.push("l2", &["b"]);
    assert_eq!(len, 2);

    let list = m.list("l2");
    assert_eq!(list, Some(vec!["a".to_string(), "b".to_string()]));
}
