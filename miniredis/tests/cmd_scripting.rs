mod helpers;
use helpers::*;

// ── EVAL basic ───────────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_return_string() {
    let (_m, mut c) = start().await;

    must_str!(c, "EVAL", "return 'hello'", "0"; "hello");
}

#[tokio::test]
async fn test_eval_return_number() {
    let (_m, mut c) = start().await;

    must_int!(c, "EVAL", "return 42", "0"; 42);
}

#[tokio::test]
async fn test_eval_return_true() {
    let (_m, mut c) = start().await;

    must_int!(c, "EVAL", "return true", "0"; 1);
}

#[tokio::test]
async fn test_eval_return_false() {
    let (_m, mut c) = start().await;

    must_nil!(c, "EVAL", "return false", "0");
}

#[tokio::test]
async fn test_eval_return_nil() {
    let (_m, mut c) = start().await;

    must_nil!(c, "EVAL", "return nil", "0");
}

#[tokio::test]
async fn test_eval_return_table() {
    let (_m, mut c) = start().await;

    let result: Vec<String> = redis::cmd("EVAL")
        .arg("return {'a', 'b', 'c'}")
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["a", "b", "c"]);
}

// ── KEYS and ARGV ────────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_keys_argv() {
    let (_m, mut c) = start().await;

    // KEYS[1] = "mykey", ARGV[1] = "myval"
    must_str!(c, "EVAL", "return KEYS[1]", "1", "mykey", "myval"; "mykey");
    must_str!(c, "EVAL", "return ARGV[1]", "1", "mykey", "myval"; "myval");
}

#[tokio::test]
async fn test_eval_multiple_keys() {
    let (_m, mut c) = start().await;

    must_str!(c, "EVAL", "return KEYS[2]", "2", "k1", "k2"; "k2");
}

// ── redis.call() ─────────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_redis_call_set_get() {
    let (_m, mut c) = start().await;

    // Use redis.call to SET and GET
    let script = r#"
        redis.call('SET', KEYS[1], ARGV[1])
        return redis.call('GET', KEYS[1])
    "#;
    must_str!(c, "EVAL", script, "1", "foo", "bar"; "bar");

    // Verify the value persists
    must_str!(c, "GET", "foo"; "bar");
}

#[tokio::test]
async fn test_eval_redis_call_incr() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SET", "counter", "10");

    let script = "return redis.call('INCR', KEYS[1])";
    must_int!(c, "EVAL", script, "1", "counter"; 11);
}

#[tokio::test]
async fn test_eval_redis_call_multiple() {
    let (_m, mut c) = start().await;

    let script = r#"
        redis.call('SET', 'k1', 'v1')
        redis.call('SET', 'k2', 'v2')
        return redis.call('MGET', 'k1', 'k2')
    "#;
    let result: Vec<String> = redis::cmd("EVAL")
        .arg(script)
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["v1", "v2"]);
}

// ── redis.pcall() ────────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_pcall_error() {
    let (_m, mut c) = start().await;

    // pcall catches errors
    let script = r#"
        local ok, err = pcall(function()
            return redis.call('NOSUCHCOMMAND')
        end)
        if ok then
            return 'no error'
        else
            return 'got error'
        end
    "#;
    must_str!(c, "EVAL", script, "0"; "got error");
}

#[tokio::test]
async fn test_eval_redis_pcall() {
    let (_m, mut c) = start().await;

    // redis.pcall returns error as table
    let script = r#"
        local res = redis.pcall('SET', 'key')
        if res.err then
            return 'error: ' .. res.err
        end
        return 'no error'
    "#;
    let result: String = redis::cmd("EVAL")
        .arg(script)
        .arg("0")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(
        result.starts_with("error: "),
        "expected error prefix, got: {}",
        result
    );
}

// ── redis.error_reply() / redis.status_reply() ──────────────────────

#[tokio::test]
async fn test_eval_error_reply() {
    let (_m, mut c) = start().await;

    must_fail!(c, "EVAL", "return redis.error_reply('MY_ERR custom error')", "0"; "MY_ERR");
}

#[tokio::test]
async fn test_eval_status_reply() {
    let (_m, mut c) = start().await;

    must_str!(c, "EVAL", "return redis.status_reply('PONG')", "0"; "PONG");
}

// ── redis.sha1hex() ──────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_sha1hex() {
    let (_m, mut c) = start().await;

    // SHA1 of empty string = da39a3ee5e6b4b0d3255bfef95601890afd80709
    must_str!(c, "EVAL", "return redis.sha1hex('')", "0"; "da39a3ee5e6b4b0d3255bfef95601890afd80709");
}

// ── EVALSHA ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_evalsha_basic() {
    let (_m, mut c) = start().await;

    // Load a script via EVAL first (caches it)
    must_int!(c, "EVAL", "return 42", "0"; 42);

    // Now get the SHA and use EVALSHA
    let sha: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg("return 42")
        .query_async(&mut c)
        .await
        .unwrap();

    must_int!(c, "EVALSHA", &sha, "0"; 42);
}

#[tokio::test]
async fn test_evalsha_not_found() {
    let (_m, mut c) = start().await;

    must_fail!(c, "EVALSHA", "deadbeef", "0"; "No matching script");
}

// ── SCRIPT LOAD / EXISTS / FLUSH ─────────────────────────────────────

#[tokio::test]
async fn test_script_load() {
    let (_m, mut c) = start().await;

    let sha: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg("return 'loaded'")
        .query_async(&mut c)
        .await
        .unwrap();

    // SHA should be a 40-char hex string
    assert_eq!(sha.len(), 40);

    // EVALSHA should work now
    must_str!(c, "EVALSHA", &sha, "0"; "loaded");
}

#[tokio::test]
async fn test_script_exists() {
    let (_m, mut c) = start().await;

    let sha: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg("return 1")
        .query_async(&mut c)
        .await
        .unwrap();

    // EXISTS should find it
    let result: Vec<i64> = redis::cmd("SCRIPT")
        .arg("EXISTS")
        .arg(&sha)
        .arg("deadbeef")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1, 0]);
}

#[tokio::test]
async fn test_script_flush() {
    let (_m, mut c) = start().await;

    let sha: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg("return 1")
        .query_async(&mut c)
        .await
        .unwrap();

    // Flush all scripts
    must_ok!(c, "SCRIPT", "FLUSH");

    // Should no longer exist
    let result: Vec<i64> = redis::cmd("SCRIPT")
        .arg("EXISTS")
        .arg(&sha)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![0]);
}

#[tokio::test]
async fn test_script_flush_sync() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SCRIPT", "FLUSH", "SYNC");
}

// ── Error handling ───────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_errors() {
    let (_m, mut c) = start().await;

    // Wrong number of args
    must_fail!(c, "EVAL"; "wrong number of arguments");
    must_fail!(c, "EVAL", "return 1"; "wrong number of arguments");

    // Invalid numkeys
    must_fail!(c, "EVAL", "return 1", "abc"; "not an integer");

    // Negative numkeys
    must_fail!(c, "EVAL", "return 1", "-1"; "negative");

    // numkeys > remaining args
    must_fail!(c, "EVAL", "return 1", "2", "key1"; "greater than number of args");
}

#[tokio::test]
async fn test_eval_syntax_error() {
    let (_m, mut c) = start().await;

    must_fail!(c, "EVAL", "this is not valid lua!!", "0"; "Error compiling script");
}

#[tokio::test]
async fn test_eval_runtime_error() {
    let (_m, mut c) = start().await;

    // redis.call with wrong type should propagate error
    must_ok!(c, "SET", "str", "value");
    must_fail!(c, "EVAL", "return redis.call('LPUSH', 'str', 'v')", "0"; "WRONGTYPE");
}

// ── Script using various data types ──────────────────────────────────

#[tokio::test]
async fn test_eval_with_hash() {
    let (_m, mut c) = start().await;

    let script = r#"
        redis.call('HSET', KEYS[1], 'field1', 'val1')
        redis.call('HSET', KEYS[1], 'field2', 'val2')
        return redis.call('HGET', KEYS[1], 'field1')
    "#;
    must_str!(c, "EVAL", script, "1", "myhash"; "val1");
}

#[tokio::test]
async fn test_eval_with_list() {
    let (_m, mut c) = start().await;

    let script = r#"
        redis.call('RPUSH', KEYS[1], 'a', 'b', 'c')
        return redis.call('LLEN', KEYS[1])
    "#;
    must_int!(c, "EVAL", script, "1", "mylist"; 3);
}

// ── SCRIPT subcommand errors ─────────────────────────────────────────

#[tokio::test]
async fn test_script_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "SCRIPT"; "wrong number of arguments");
    must_fail!(c, "SCRIPT", "NOSUCHSUB"; "unknown subcommand");
    must_fail!(c, "SCRIPT", "LOAD"; "wrong number of arguments");
    must_fail!(c, "SCRIPT", "EXISTS"; "wrong number of arguments");
}

// ── EVAL_RO / EVALSHA_RO ─────────────────────────────────────────────

#[tokio::test]
async fn test_eval_ro_read() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SET", "k", "v");

    // Read-only should allow reads
    must_str!(c, "EVAL_RO", "return redis.call('GET', KEYS[1])", "1", "k"; "v");
}

#[tokio::test]
async fn test_eval_ro_write_blocked() {
    let (_m, mut c) = start().await;

    // Read-only should block writes
    must_fail!(c, "EVAL_RO", "return redis.call('SET', 'k', 'v')", "0"; "Write commands are not allowed");
}

// ── EVALSHA_RO ───────────────────────────────────────────────────────

#[tokio::test]
async fn test_evalsha_ro_read() {
    let (_m, mut c) = start().await;

    let script = "return redis.call('GET', KEYS[1])";
    let sha: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg(script)
        .query_async(&mut c)
        .await
        .unwrap();

    must_ok!(c, "SET", "readonly", "foo");

    // Read-only should allow reads
    must_str!(c, "EVALSHA_RO", &sha, "1", "readonly"; "foo");
}

#[tokio::test]
async fn test_evalsha_ro_write_blocked() {
    let (_m, mut c) = start().await;

    let write_script = "return redis.call('SET', KEYS[1], ARGV[1])";
    let sha: String = redis::cmd("SCRIPT")
        .arg("LOAD")
        .arg(write_script)
        .query_async(&mut c)
        .await
        .unwrap();

    // Read-only should block writes
    must_fail!(c, "EVALSHA_RO", &sha, "1", "key1", "value1"; "Write commands are not allowed");
}

// ── redis.call() error cases ─────────────────────────────────────────

#[tokio::test]
async fn test_eval_call_errors() {
    let (_m, mut c) = start().await;

    // redis.call() with no args
    must_fail!(c, "EVAL", "redis.call()", "0"; "Please specify at least one argument");

    // redis.call with table arg
    must_fail!(c, "EVAL", "redis.call({})", "0"; "must be strings or integers");

    // redis.call with number arg (number as command name is treated as string "1")
    must_fail!(c, "EVAL", "redis.call(1)", "0"; "Unknown Redis command");
}

// ── redis.log() ──────────────────────────────────────────────────────

#[tokio::test]
async fn test_eval_log() {
    let (_m, mut c) = start().await;

    // redis.log should succeed and return nil
    must_nil!(c, "EVAL", "redis.log(redis.LOG_NOTICE, 'hello')", "0");
}

// ── redis.replicate_commands() / redis.set_repl() ────────────────────

#[tokio::test]
async fn test_eval_replicate_commands() {
    let (_m, mut c) = start().await;

    // replicate_commands returns true (always enabled)
    must_int!(c, "EVAL", "return redis.replicate_commands()", "0"; 1);
}

#[tokio::test]
async fn test_eval_set_repl() {
    let (_m, mut c) = start().await;

    // set_repl is a no-op, takes an integer argument
    must_nil!(c, "EVAL", "redis.set_repl(0)", "0");
}
