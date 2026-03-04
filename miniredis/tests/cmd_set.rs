// Ported from ../miniredis/cmd_set_test.go
mod helpers;

#[tokio::test]
async fn test_sadd() {
    let (m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s", "a", "b", "c"; 3);
    must_int!(c, "SCARD", "s"; 3);
    must_int!(c, "SADD", "s", "a", "b", "d"; 1); // only d is new

    // SMEMBERS
    must_strs_sorted!(c, "SMEMBERS", "s"; ["a", "b", "c", "d"]);

    // Non-existing
    let members: Vec<String> = redis::cmd("SMEMBERS")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(members.is_empty());

    // Direct API
    assert_eq!(m.key_type("s"), "set");

    // Errors
    must_fail!(c, "SADD"; "wrong number of arguments");
    must_fail!(c, "SADD", "s"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SADD", "str", "x"; "WRONGTYPE");
    must_fail!(c, "SMEMBERS", "str"; "WRONGTYPE");
    must_fail!(c, "SCARD", "str"; "WRONGTYPE");
}

#[tokio::test]
async fn test_sismember() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s", "a", "b"; 2);

    must_int!(c, "SISMEMBER", "s", "a"; 1);
    must_int!(c, "SISMEMBER", "s", "b"; 1);
    must_int!(c, "SISMEMBER", "s", "nosuch"; 0);

    // Non-existing key
    must_int!(c, "SISMEMBER", "nosuch", "a"; 0);

    // Errors
    must_fail!(c, "SISMEMBER"; "wrong number of arguments");
    must_fail!(c, "SISMEMBER", "s"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SISMEMBER", "str", "x"; "WRONGTYPE");
}

#[tokio::test]
async fn test_smismember() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s", "a", "b", "c"; 3);

    let result: Vec<i64> = redis::cmd("SMISMEMBER")
        .arg("s")
        .arg("a")
        .arg("x")
        .arg("c")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1, 0, 1]);

    // Non-existing key
    let result: Vec<i64> = redis::cmd("SMISMEMBER")
        .arg("nosuch")
        .arg("a")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![0, 0]);

    // Errors
    must_fail!(c, "SMISMEMBER"; "wrong number of arguments");
    must_fail!(c, "SMISMEMBER", "s"; "wrong number of arguments");
}

#[tokio::test]
async fn test_srem() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s", "a", "b", "c"; 3);

    must_int!(c, "SREM", "s", "a", "d"; 1);
    must_strs_sorted!(c, "SMEMBERS", "s"; ["b", "c"]);

    // Remove from non-existing key
    must_int!(c, "SREM", "nosuch", "a"; 0);

    // Errors
    must_fail!(c, "SREM"; "wrong number of arguments");
    must_fail!(c, "SREM", "s"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SREM", "str", "x"; "WRONGTYPE");
}

#[tokio::test]
async fn test_smove() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "src", "a", "b", "c"; 3);
    must_int!(c, "SADD", "dst", "x"; 1);

    must_int!(c, "SMOVE", "src", "dst", "a"; 1);
    must_strs_sorted!(c, "SMEMBERS", "src"; ["b", "c"]);
    must_strs_sorted!(c, "SMEMBERS", "dst"; ["a", "x"]);

    // Move non-existing member
    must_int!(c, "SMOVE", "src", "dst", "nosuch"; 0);

    // Move from non-existing key
    must_int!(c, "SMOVE", "nosuch", "dst", "a"; 0);

    // Move last member
    must_int!(c, "SADD", "single", "x"; 1);
    must_int!(c, "SMOVE", "single", "dst", "x"; 1);

    // Errors
    must_fail!(c, "SMOVE"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SMOVE", "str", "dst", "x"; "WRONGTYPE");
    must_fail!(c, "SMOVE", "src", "str", "x"; "WRONGTYPE");
}

#[tokio::test]
async fn test_sdiff() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b", "c"; 3);
    must_int!(c, "SADD", "s2", "b", "c", "d"; 3);

    must_strs_sorted!(c, "SDIFF", "s1", "s2"; ["a"]);

    // Single set
    must_strs_sorted!(c, "SDIFF", "s1"; ["a", "b", "c"]);

    // Three sets
    must_int!(c, "SADD", "s3", "a"; 1);
    let result: Vec<String> = redis::cmd("SDIFF")
        .arg("s1")
        .arg("s2")
        .arg("s3")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());

    // Non-existing key
    must_strs_sorted!(c, "SDIFF", "s1", "nosuch"; ["a", "b", "c"]);

    // Errors
    must_fail!(c, "SDIFF"; "wrong number of arguments");
}

#[tokio::test]
async fn test_sdiffstore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b", "c"; 3);
    must_int!(c, "SADD", "s2", "b", "c", "d"; 3);

    must_int!(c, "SDIFFSTORE", "dst", "s1", "s2"; 1);
    must_strs_sorted!(c, "SMEMBERS", "dst"; ["a"]);

    // Errors
    must_fail!(c, "SDIFFSTORE"; "wrong number of arguments");
    must_fail!(c, "SDIFFSTORE", "dst"; "wrong number of arguments");
}

#[tokio::test]
async fn test_sinter() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b", "c"; 3);
    must_int!(c, "SADD", "s2", "b", "c", "d"; 3);

    must_strs_sorted!(c, "SINTER", "s1", "s2"; ["b", "c"]);

    // Single set
    must_strs_sorted!(c, "SINTER", "s1"; ["a", "b", "c"]);

    // Three sets
    must_int!(c, "SADD", "s3", "b"; 1);
    must_strs_sorted!(c, "SINTER", "s1", "s2", "s3"; ["b"]);

    // Non-existing key → empty
    let result: Vec<String> = redis::cmd("SINTER")
        .arg("s1")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());

    // Errors
    must_fail!(c, "SINTER"; "wrong number of arguments");
}

#[tokio::test]
async fn test_sinterstore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b", "c"; 3);
    must_int!(c, "SADD", "s2", "b", "c", "d"; 3);

    must_int!(c, "SINTERSTORE", "dst", "s1", "s2"; 2);
    must_strs_sorted!(c, "SMEMBERS", "dst"; ["b", "c"]);

    // Empty intersection with non-existing key
    must_int!(c, "SINTERSTORE", "dst2", "s1", "nosuch"; 0);

    // Errors
    must_fail!(c, "SINTERSTORE"; "wrong number of arguments");
    must_fail!(c, "SINTERSTORE", "dst"; "wrong number of arguments");
}

#[tokio::test]
async fn test_sunion() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b"; 2);
    must_int!(c, "SADD", "s2", "b", "c"; 2);

    must_strs_sorted!(c, "SUNION", "s1", "s2"; ["a", "b", "c"]);

    // Single set
    must_strs_sorted!(c, "SUNION", "s1"; ["a", "b"]);

    // Three sets
    must_int!(c, "SADD", "s3", "d"; 1);
    must_strs_sorted!(c, "SUNION", "s1", "s2", "s3"; ["a", "b", "c", "d"]);

    // Non-existing key
    must_strs_sorted!(c, "SUNION", "s1", "nosuch"; ["a", "b"]);

    // Errors
    must_fail!(c, "SUNION"; "wrong number of arguments");
}

#[tokio::test]
async fn test_sunionstore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b"; 2);
    must_int!(c, "SADD", "s2", "b", "c"; 2);

    must_int!(c, "SUNIONSTORE", "dst", "s1", "s2"; 3);
    must_strs_sorted!(c, "SMEMBERS", "dst"; ["a", "b", "c"]);

    // Errors
    must_fail!(c, "SUNIONSTORE"; "wrong number of arguments");
    must_fail!(c, "SUNIONSTORE", "dst"; "wrong number of arguments");
}

#[tokio::test]
async fn test_scard() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SCARD", "nosuch"; 0);
    must_int!(c, "SADD", "s", "a", "b", "c"; 3);
    must_int!(c, "SCARD", "s"; 3);

    // Errors
    must_fail!(c, "SCARD"; "wrong number of arguments");
}

#[tokio::test]
async fn test_set_wrongtype() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SADD", "str", "x"; "WRONGTYPE");
    must_fail!(c, "SREM", "str", "x"; "WRONGTYPE");
    must_fail!(c, "SCARD", "str"; "WRONGTYPE");
    must_fail!(c, "SMEMBERS", "str"; "WRONGTYPE");
    must_fail!(c, "SISMEMBER", "str", "x"; "WRONGTYPE");
}

#[tokio::test]
async fn test_spop() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s", "aap", "noot"; 2);

    // SPOP returns a member
    let v: String = redis::cmd("SPOP")
        .arg("s")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v == "aap" || v == "noot");

    // One left, pop it
    let v2: String = redis::cmd("SPOP")
        .arg("s")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v2 == "aap" || v2 == "noot");
    assert_ne!(v, v2);

    // Key is now gone
    must_int!(c, "SCARD", "s"; 0);

    // Non-existing key
    must_nil!(c, "SPOP", "nosuch");

    // With count
    must_int!(c, "SADD", "s2", "a", "b", "c", "d"; 4);
    let vals: Vec<String> = redis::cmd("SPOP")
        .arg("s2")
        .arg(2)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 2);
    must_int!(c, "SCARD", "s2"; 2);

    // Errors
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SPOP", "str"; "WRONGTYPE");
    must_fail!(c, "SPOP", "str", "-12"; "out of range");
}

#[tokio::test]
async fn test_srandmember() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s", "aap", "noot", "mies"; 3);

    // Without count
    let v: String = redis::cmd("SRANDMEMBER")
        .arg("s")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v == "aap" || v == "noot" || v == "mies");

    // Set still has all members (non-destructive)
    must_int!(c, "SCARD", "s"; 3);

    // Positive count
    let vals: Vec<String> = redis::cmd("SRANDMEMBER")
        .arg("s")
        .arg(2)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 2);
    // No duplicates with positive count
    assert_ne!(vals[0], vals[1]);

    // Positive count larger than set
    let vals: Vec<String> = redis::cmd("SRANDMEMBER")
        .arg("s")
        .arg(10)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 3);

    // Negative count allows duplicates
    let vals: Vec<String> = redis::cmd("SRANDMEMBER")
        .arg("s")
        .arg(-5)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 5);

    // Non-existing key
    must_nil!(c, "SRANDMEMBER", "nosuch");
    let vals: Vec<String> = redis::cmd("SRANDMEMBER")
        .arg("nosuch")
        .arg(1)
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(vals.is_empty());

    // Errors
    must_fail!(c, "SRANDMEMBER"; "wrong number of arguments");
    must_fail!(c, "SRANDMEMBER", "s", "noint"; "not an integer");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SRANDMEMBER", "str"; "WRONGTYPE");
}

#[tokio::test]
async fn test_sscan() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "set", "value1", "value2"; 2);

    // Basic scan
    let (cursor, vals): (String, Vec<String>) = redis::cmd("SSCAN")
        .arg("set")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    let mut sorted = vals.clone();
    sorted.sort();
    assert_eq!(sorted, vec!["value1", "value2"]);

    // MATCH
    must_int!(c, "SADD", "s2", "aap", "noot", "mies"; 3);
    let (cursor, vals): (String, Vec<String>) = redis::cmd("SSCAN")
        .arg("s2")
        .arg(0)
        .arg("MATCH")
        .arg("mi*")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    assert_eq!(vals, vec!["mies"]);

    // COUNT (accepted but ignored in miniredis)
    let (cursor, _vals): (String, Vec<String>) = redis::cmd("SSCAN")
        .arg("set")
        .arg(0)
        .arg("COUNT")
        .arg(200)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");

    // Errors
    must_fail!(c, "SSCAN"; "wrong number of arguments");
    must_fail!(c, "SSCAN", "set"; "wrong number of arguments");
    must_fail!(c, "SSCAN", "set", "noint"; "invalid cursor");
    must_fail!(c, "SSCAN", "set", "0", "MATCH"; "syntax error");
    must_fail!(c, "SSCAN", "set", "0", "COUNT"; "syntax error");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SSCAN", "str", "0"; "WRONGTYPE");
}

#[tokio::test]
async fn test_sintercard() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "SADD", "s1", "a", "b", "c"; 3);
    must_int!(c, "SADD", "s2", "b", "c", "d"; 3);

    // Basic
    must_int!(c, "SINTERCARD", "2", "s1", "s2"; 2);

    // With LIMIT > result
    must_int!(c, "SINTERCARD", "2", "s1", "s2", "LIMIT", "15"; 2);

    // LIMIT 0 (unlimited)
    must_int!(c, "SINTERCARD", "2", "s1", "s2", "LIMIT", "0"; 2);

    // LIMIT 1
    must_int!(c, "SINTERCARD", "2", "s1", "s2", "LIMIT", "1"; 1);

    // Multi intersection
    must_int!(c, "SADD", "s3", "c", "d", "e"; 3);
    must_int!(c, "SINTERCARD", "3", "s1", "s2", "s3"; 1);

    // Non-existing key
    must_int!(c, "SINTERCARD", "2", "s1", "NOT_A_KEY"; 0);

    // Errors
    must_fail!(c, "SINTERCARD", "two", "k1", "k2"; "numkeys");
    must_fail!(c, "SINTERCARD", "2", "k1", "k2", "LIMIT", "five"; "not an integer");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "SINTERCARD", "1", "str"; "WRONGTYPE");
}
