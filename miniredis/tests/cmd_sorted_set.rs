// Ported from ../miniredis/cmd_sorted_set_test.go
mod helpers;

#[tokio::test]
async fn test_zadd() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c"; 3);
    must_int!(c, "ZCARD", "z"; 3);

    // Update existing
    must_int!(c, "ZADD", "z", "4", "a"; 0);
    must_int!(c, "ZCARD", "z"; 3);

    // Errors
    must_fail!(c, "ZADD"; "wrong number of arguments");
    must_fail!(c, "ZADD", "z"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "ZADD", "str", "1", "a"; "WRONGTYPE");
}

#[tokio::test]
async fn test_zadd_nx_xx() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b"; 2);

    // NX: only add new
    must_int!(c, "ZADD", "z", "NX", "10", "a", "3", "c"; 1);
    // a should still be 1
    let score: f64 = redis::cmd("ZSCORE")
        .arg("z")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 1.0);

    // XX: only update existing
    must_int!(c, "ZADD", "z", "XX", "10", "a", "4", "d"; 0);
    // a should be updated
    let score: f64 = redis::cmd("ZSCORE")
        .arg("z")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 10.0);
    // d should not exist
    let d: Option<f64> = redis::cmd("ZSCORE")
        .arg("z")
        .arg("d")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(d.is_none());

    // XX and NX together
    must_fail!(c, "ZADD", "z", "XX", "NX", "1", "a"; "XX and NX");
}

#[tokio::test]
async fn test_zadd_ch() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b"; 2);

    // CH: count changed (new + updated)
    must_int!(c, "ZADD", "z", "CH", "10", "a", "2", "b", "3", "c"; 2);
    // a was updated (1->10), c was new = 2 changes
}

#[tokio::test]
async fn test_zadd_incr() {
    let (_m, mut c) = helpers::start().await;

    // INCR mode
    let score: String = redis::cmd("ZADD")
        .arg("z")
        .arg("INCR")
        .arg("5")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, "5");

    let score: String = redis::cmd("ZADD")
        .arg("z")
        .arg("INCR")
        .arg("3")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, "8");
}

#[tokio::test]
async fn test_zscore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1.5", "a", "2.5", "b"; 2);

    let score: f64 = redis::cmd("ZSCORE")
        .arg("z")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 1.5);

    // Non-existing member
    let score: Option<f64> = redis::cmd("ZSCORE")
        .arg("z")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(score.is_none());

    // Non-existing key
    must_nil!(c, "ZSCORE", "nosuch", "a");

    // Errors
    must_fail!(c, "ZSCORE"; "wrong number of arguments");
}

#[tokio::test]
async fn test_zmscore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b"; 2);

    let scores: Vec<Option<f64>> = redis::cmd("ZMSCORE")
        .arg("z")
        .arg("a")
        .arg("nosuch")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(scores, vec![Some(1.0), None, Some(2.0)]);
}

#[tokio::test]
async fn test_zincrby() {
    let (_m, mut c) = helpers::start().await;

    let score: String = redis::cmd("ZINCRBY")
        .arg("z")
        .arg("1.5")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, "1.5");

    let score: String = redis::cmd("ZINCRBY")
        .arg("z")
        .arg("2.5")
        .arg("a")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, "4");

    // Errors
    must_fail!(c, "ZINCRBY"; "wrong number of arguments");

    // Wrong type
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "ZINCRBY", "str", "1", "a"; "WRONGTYPE");
}

#[tokio::test]
async fn test_zrank() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c"; 3);

    must_int!(c, "ZRANK", "z", "a"; 0);
    must_int!(c, "ZRANK", "z", "b"; 1);
    must_int!(c, "ZRANK", "z", "c"; 2);

    // Non-existing member
    must_nil!(c, "ZRANK", "z", "nosuch");

    // Non-existing key
    must_nil!(c, "ZRANK", "nosuch", "a");

    // ZREVRANK
    must_int!(c, "ZREVRANK", "z", "a"; 2);
    must_int!(c, "ZREVRANK", "z", "c"; 0);
}

#[tokio::test]
async fn test_zrem() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c"; 3);

    must_int!(c, "ZREM", "z", "a", "nosuch"; 1);
    must_int!(c, "ZCARD", "z"; 2);

    // Non-existing key
    must_int!(c, "ZREM", "nosuch", "a"; 0);

    // Errors
    must_fail!(c, "ZREM"; "wrong number of arguments");
    must_fail!(c, "ZREM", "z"; "wrong number of arguments");
}

#[tokio::test]
async fn test_zrange() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d"; 4);

    must_strs!(c, "ZRANGE", "z", "0", "-1"; ["a", "b", "c", "d"]);
    must_strs!(c, "ZRANGE", "z", "1", "2"; ["b", "c"]);
    must_strs!(c, "ZRANGE", "z", "0", "0"; ["a"]);

    // Empty range
    let result: Vec<String> = redis::cmd("ZRANGE")
        .arg("z")
        .arg("10")
        .arg("20")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());

    // Non-existing key
    let result: Vec<String> = redis::cmd("ZRANGE")
        .arg("nosuch")
        .arg("0")
        .arg("-1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());
}

#[tokio::test]
async fn test_zrevrange() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c"; 3);

    must_strs!(c, "ZREVRANGE", "z", "0", "-1"; ["c", "b", "a"]);
    must_strs!(c, "ZREVRANGE", "z", "0", "1"; ["c", "b"]);
}

#[tokio::test]
async fn test_zrangebyscore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d"; 4);

    must_strs!(c, "ZRANGEBYSCORE", "z", "-inf", "+inf"; ["a", "b", "c", "d"]);
    must_strs!(c, "ZRANGEBYSCORE", "z", "2", "3"; ["b", "c"]);
    must_strs!(c, "ZRANGEBYSCORE", "z", "(1", "3"; ["b", "c"]);
    must_strs!(c, "ZRANGEBYSCORE", "z", "1", "(3"; ["a", "b"]);

    // LIMIT
    must_strs!(c, "ZRANGEBYSCORE", "z", "-inf", "+inf", "LIMIT", "1", "2"; ["b", "c"]);

    // Empty
    let result: Vec<String> = redis::cmd("ZRANGEBYSCORE")
        .arg("z")
        .arg("10")
        .arg("20")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());
}

#[tokio::test]
async fn test_zrevrangebyscore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d"; 4);

    must_strs!(c, "ZREVRANGEBYSCORE", "z", "+inf", "-inf"; ["d", "c", "b", "a"]);
    must_strs!(c, "ZREVRANGEBYSCORE", "z", "3", "2"; ["c", "b"]);
}

#[tokio::test]
async fn test_zrangebylex() {
    let (_m, mut c) = helpers::start().await;

    // All same score for lex ordering
    must_int!(c, "ZADD", "z", "0", "a", "0", "b", "0", "c", "0", "d"; 4);

    must_strs!(c, "ZRANGEBYLEX", "z", "-", "+"; ["a", "b", "c", "d"]);
    must_strs!(c, "ZRANGEBYLEX", "z", "[b", "[c"; ["b", "c"]);
    must_strs!(c, "ZRANGEBYLEX", "z", "(a", "[c"; ["b", "c"]);
    must_strs!(c, "ZRANGEBYLEX", "z", "[b", "(d"; ["b", "c"]);

    // LIMIT
    must_strs!(c, "ZRANGEBYLEX", "z", "-", "+", "LIMIT", "1", "2"; ["b", "c"]);
}

#[tokio::test]
async fn test_zrevrangebylex() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "0", "a", "0", "b", "0", "c", "0", "d"; 4);

    must_strs!(c, "ZREVRANGEBYLEX", "z", "+", "-"; ["d", "c", "b", "a"]);
    must_strs!(c, "ZREVRANGEBYLEX", "z", "[c", "[b"; ["c", "b"]);
}

#[tokio::test]
async fn test_zlexcount() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "0", "a", "0", "b", "0", "c", "0", "d"; 4);

    must_int!(c, "ZLEXCOUNT", "z", "-", "+"; 4);
    must_int!(c, "ZLEXCOUNT", "z", "[b", "[c"; 2);
    must_int!(c, "ZLEXCOUNT", "z", "(a", "[c"; 2);

    // Non-existing key
    must_int!(c, "ZLEXCOUNT", "nosuch", "-", "+"; 0);
}

#[tokio::test]
async fn test_zcount() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c"; 3);

    must_int!(c, "ZCOUNT", "z", "-inf", "+inf"; 3);
    must_int!(c, "ZCOUNT", "z", "1", "2"; 2);
    must_int!(c, "ZCOUNT", "z", "(1", "3"; 2);

    // Non-existing key
    must_int!(c, "ZCOUNT", "nosuch", "-inf", "+inf"; 0);
}

#[tokio::test]
async fn test_zremrangebyrank() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d"; 4);

    must_int!(c, "ZREMRANGEBYRANK", "z", "1", "2"; 2);
    must_strs!(c, "ZRANGE", "z", "0", "-1"; ["a", "d"]);
}

#[tokio::test]
async fn test_zremrangebyscore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d"; 4);

    must_int!(c, "ZREMRANGEBYSCORE", "z", "2", "3"; 2);
    must_strs!(c, "ZRANGE", "z", "0", "-1"; ["a", "d"]);
}

#[tokio::test]
async fn test_zremrangebylex() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "0", "a", "0", "b", "0", "c", "0", "d"; 4);

    must_int!(c, "ZREMRANGEBYLEX", "z", "[b", "[c"; 2);
    must_strs!(c, "ZRANGE", "z", "0", "-1"; ["a", "d"]);
}

#[tokio::test]
async fn test_zunionstore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z1", "1", "a", "2", "b"; 2);
    must_int!(c, "ZADD", "z2", "3", "b", "4", "c"; 2);

    must_int!(c, "ZUNIONSTORE", "dst", "2", "z1", "z2"; 3);
    // a=1, b=2+3=5, c=4
    let score: f64 = redis::cmd("ZSCORE")
        .arg("dst")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 5.0);

    // Non-existing key
    must_int!(c, "ZUNIONSTORE", "dst2", "2", "z1", "nosuch"; 2);

    // Errors
    must_fail!(c, "ZUNIONSTORE"; "wrong number of arguments");
}

#[tokio::test]
async fn test_zinterstore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z1", "1", "a", "2", "b", "3", "c"; 3);
    must_int!(c, "ZADD", "z2", "10", "b", "20", "c", "30", "d"; 3);

    must_int!(c, "ZINTERSTORE", "dst", "2", "z1", "z2"; 2);
    // b=2+10=12, c=3+20=23
    let score: f64 = redis::cmd("ZSCORE")
        .arg("dst")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 12.0);

    // WEIGHTS
    must_int!(c, "ZINTERSTORE", "dst2", "2", "z1", "z2", "WEIGHTS", "2", "1"; 2);
    let score: f64 = redis::cmd("ZSCORE")
        .arg("dst2")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 14.0); // 2*2 + 10*1

    // AGGREGATE MIN
    must_int!(c, "ZINTERSTORE", "dst3", "2", "z1", "z2", "AGGREGATE", "MIN"; 2);
    let score: f64 = redis::cmd("ZSCORE")
        .arg("dst3")
        .arg("b")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(score, 2.0);
}

#[tokio::test]
async fn test_zpopmin_zpopmax() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "a", "2", "b", "3", "c"; 3);

    // ZPOPMIN
    let result: Vec<String> = redis::cmd("ZPOPMIN")
        .arg("z")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["a", "1"]);

    // ZPOPMAX
    let result: Vec<String> = redis::cmd("ZPOPMAX")
        .arg("z")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["c", "3"]);

    must_int!(c, "ZCARD", "z"; 1);

    // Empty set
    let result: Vec<String> = redis::cmd("ZPOPMIN")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());
}

#[tokio::test]
async fn test_zrange_withscores() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1.5", "a", "2.5", "b"; 2);

    let result: Vec<String> = redis::cmd("ZRANGE")
        .arg("z")
        .arg("0")
        .arg("-1")
        .arg("WITHSCORES")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec!["a", "1.5", "b", "2.5"]);
}

#[tokio::test]
async fn test_sorted_set_wrongtype() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "ZADD", "str", "1", "a"; "WRONGTYPE");
    must_fail!(c, "ZCARD", "str"; "WRONGTYPE");
    must_fail!(c, "ZSCORE", "str", "a"; "WRONGTYPE");
    must_fail!(c, "ZRANK", "str", "a"; "WRONGTYPE");
    must_fail!(c, "ZREM", "str", "a"; "WRONGTYPE");
    must_fail!(c, "ZRANGE", "str", "0", "-1"; "WRONGTYPE");
}

#[tokio::test]
async fn test_zinter() {
    let (_m, mut c) = helpers::start().await;

    let _: i64 = redis::cmd("ZADD")
        .arg("h1")
        .arg(1)
        .arg("field1")
        .arg(2)
        .arg("field2")
        .arg(3)
        .arg("field3")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: i64 = redis::cmd("ZADD")
        .arg("h2")
        .arg(1)
        .arg("field1")
        .arg(2)
        .arg("field2")
        .arg(4)
        .arg("field4")
        .query_async(&mut c)
        .await
        .unwrap();

    // Basic intersection
    must_strs!(c, "ZINTER", "2", "h1", "h2"; ["field1", "field2"]);

    // With WITHSCORES
    must_strs!(c, "ZINTER", "2", "h1", "h2", "WITHSCORES"; ["field1", "2", "field2", "4"]);

    // Errors
    must_fail!(c, "ZINTER"; "wrong number of arguments");
    must_fail!(c, "ZINTER", "noint", "k"; "not an integer");
}

#[tokio::test]
async fn test_zunion() {
    let (_m, mut c) = helpers::start().await;

    let _: i64 = redis::cmd("ZADD")
        .arg("h1")
        .arg(1)
        .arg("field1")
        .arg(2)
        .arg("field2")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: i64 = redis::cmd("ZADD")
        .arg("h2")
        .arg(1)
        .arg("field1")
        .arg(2)
        .arg("field2")
        .query_async(&mut c)
        .await
        .unwrap();

    // Basic union
    must_strs!(c, "ZUNION", "2", "h1", "h2"; ["field1", "field2"]);

    // With WITHSCORES (sum by default)
    must_strs!(c, "ZUNION", "2", "h1", "h2", "WITHSCORES"; ["field1", "2", "field2", "4"]);

    // AGGREGATE MIN
    must_strs!(c, "ZUNION", "2", "h1", "h2", "AGGREGATE", "MIN", "WITHSCORES"; ["field1", "1", "field2", "2"]);

    // Errors
    must_fail!(c, "ZUNION"; "wrong number of arguments");
    must_fail!(c, "ZUNION", "2"; "wrong number of arguments");
    must_fail!(c, "ZUNION", "noint", "k"; "not an integer");
}

#[tokio::test]
async fn test_zrandmember() {
    let (_m, mut c) = helpers::start().await;

    let _: i64 = redis::cmd("ZADD")
        .arg("z")
        .arg(1)
        .arg("one")
        .arg(2)
        .arg("two")
        .arg(3)
        .arg("three")
        .query_async(&mut c)
        .await
        .unwrap();

    // Without count
    let v: String = redis::cmd("ZRANDMEMBER")
        .arg("z")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v == "one" || v == "two" || v == "three");

    // Nonexistent key
    must_nil!(c, "ZRANDMEMBER", "nosuch");

    // Positive count
    let vals: Vec<String> = redis::cmd("ZRANDMEMBER")
        .arg("z")
        .arg(2)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 2);

    // Positive count larger than set
    let vals: Vec<String> = redis::cmd("ZRANDMEMBER")
        .arg("z")
        .arg(10)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 3);

    // Count 0
    let vals: Vec<String> = redis::cmd("ZRANDMEMBER")
        .arg("z")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(vals.is_empty());

    // Negative count
    let vals: Vec<String> = redis::cmd("ZRANDMEMBER")
        .arg("z")
        .arg(-5)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 5);

    // Nonexistent key with count
    let vals: Vec<String> = redis::cmd("ZRANDMEMBER")
        .arg("nosuch")
        .arg(40)
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(vals.is_empty());

    // Errors
    must_fail!(c, "ZRANDMEMBER"; "wrong number of arguments");
    must_fail!(c, "ZRANDMEMBER", "z", "noint"; "not an integer");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "ZRANDMEMBER", "str", "1"; "WRONGTYPE");
}

#[tokio::test]
async fn test_issue10_float_scores() {
    let (_m, mut c) = helpers::start().await;

    // Regression: ZRANGEBYSCORE with exact float boundaries
    let _: i64 = redis::cmd("ZADD")
        .arg("key")
        .arg(3.3)
        .arg("element")
        .query_async(&mut c)
        .await
        .unwrap();

    must_strs!(c, "ZRANGEBYSCORE", "key", "3.3", "3.3"; ["element"]);

    // No match
    let result: Vec<String> = redis::cmd("ZRANGEBYSCORE")
        .arg("key")
        .arg("4.3")
        .arg("4.3")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());
}

#[tokio::test]
async fn test_zscan() {
    let (_m, mut c) = helpers::start().await;

    let _: i64 = redis::cmd("ZADD")
        .arg("z")
        .arg(1.0)
        .arg("field1")
        .arg(2.0)
        .arg("field2")
        .query_async(&mut c)
        .await
        .unwrap();

    // Basic scan
    let (cursor, vals): (String, Vec<String>) = redis::cmd("ZSCAN")
        .arg("z")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    assert_eq!(vals.len(), 4); // field1, 1, field2, 2
    assert!(vals.contains(&"field1".to_string()));
    assert!(vals.contains(&"field2".to_string()));

    // COUNT (accepted but ignored)
    let (cursor, vals): (String, Vec<String>) = redis::cmd("ZSCAN")
        .arg("z")
        .arg(0)
        .arg("COUNT")
        .arg(200)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    assert_eq!(vals.len(), 4);

    // MATCH
    let _: i64 = redis::cmd("ZADD")
        .arg("z")
        .arg(3.0)
        .arg("aap")
        .arg(4.0)
        .arg("noot")
        .arg(5.0)
        .arg("mies")
        .query_async(&mut c)
        .await
        .unwrap();
    let (cursor, vals): (String, Vec<String>) = redis::cmd("ZSCAN")
        .arg("z")
        .arg(0)
        .arg("MATCH")
        .arg("mi*")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    assert_eq!(vals, vec!["mies", "5"]);

    // Errors
    must_fail!(c, "ZSCAN"; "wrong number of arguments");
    must_fail!(c, "ZSCAN", "z"; "wrong number of arguments");
    must_fail!(c, "ZSCAN", "z", "noint"; "invalid cursor");
    must_fail!(c, "ZSCAN", "z", "0", "MATCH"; "syntax error");
    must_fail!(c, "ZSCAN", "z", "0", "COUNT"; "syntax error");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "ZSCAN", "str", "0"; "WRONGTYPE");
}

#[tokio::test]
async fn test_sorted_set_infinity() {
    let (_m, mut c) = helpers::start().await;

    // Add with infinity scores
    must_int!(c, "ZADD", "zinf", "inf", "plus_inf", "-inf", "minus_inf", "10", "ten"; 3);
    must_int!(c, "ZCARD", "zinf"; 3);

    // Check ordering: -inf, 10, +inf
    must_strs!(c, "ZRANGE", "zinf", "0", "-1"; ["minus_inf", "ten", "plus_inf"]);
}

#[tokio::test]
async fn test_sorted_set_zrank_withscore() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "ZADD", "z", "1", "one", "2", "two", "3", "three"; 3);

    // ZRANK with WITHSCORE
    let v: redis::Value = redis::cmd("ZRANK")
        .arg("z")
        .arg("three")
        .arg("WITHSCORE")
        .query_async(&mut c)
        .await
        .unwrap();
    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 2);
            assert_eq!(items[0], redis::Value::Int(2));
        }
        _ => panic!("expected array from ZRANK WITHSCORE, got {:?}", v),
    }

    // ZREVRANK with WITHSCORE
    let v: redis::Value = redis::cmd("ZREVRANK")
        .arg("z")
        .arg("one")
        .arg("WITHSCORE")
        .query_async(&mut c)
        .await
        .unwrap();
    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 2);
            assert_eq!(items[0], redis::Value::Int(2));
        }
        _ => panic!("expected array from ZREVRANK WITHSCORE, got {:?}", v),
    }
}
