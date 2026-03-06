// Ported from ../miniredis/cmd_hash_test.go
mod helpers;

#[tokio::test]
async fn test_hset_hget() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "HSET", "h", "field1", "val1"; 1);
    must_str!(c, "HGET", "h", "field1"; "val1");

    // Overwrite existing field
    must_int!(c, "HSET", "h", "field1", "val2"; 0);
    must_str!(c, "HGET", "h", "field1"; "val2");

    // Multiple fields at once
    must_int!(c, "HSET", "h", "a", "1", "b", "2"; 2);
    must_str!(c, "HGET", "h", "a"; "1");
    must_str!(c, "HGET", "h", "b"; "2");

    // Non-existent field
    must_nil!(c, "HGET", "h", "nosuch");
    // Non-existent key
    must_nil!(c, "HGET", "nosuch", "field");

    // Errors
    must_fail!(c, "HSET"; "wrong number of arguments");
    must_fail!(c, "HSET", "h"; "wrong number of arguments");
    must_fail!(c, "HSET", "h", "f"; "wrong number of arguments");
    must_fail!(c, "HGET"; "wrong number of arguments");
}

#[tokio::test]
async fn test_hsetnx() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "HSETNX", "h", "field", "val"; 1);
    must_str!(c, "HGET", "h", "field"; "val");

    // Already exists
    must_int!(c, "HSETNX", "h", "field", "other"; 0);
    must_str!(c, "HGET", "h", "field"; "val");
}

#[tokio::test]
async fn test_hmset_hmget() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "HMSET", "h", "a", "1", "b", "2", "c", "3");
    must_strs!(c, "HMGET", "h", "a", "b", "c"; ["1", "2", "3"]);

    // Missing fields return nil
    let result: Vec<Option<String>> = redis::cmd("HMGET")
        .arg("h")
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
async fn test_hdel() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "HMSET", "h", "a", "1", "b", "2", "c", "3");
    must_int!(c, "HDEL", "h", "a", "b"; 2);
    must_nil!(c, "HGET", "h", "a");
    must_str!(c, "HGET", "h", "c"; "3");

    // Non-existent field
    must_int!(c, "HDEL", "h", "nosuch"; 0);

    // Non-existent key
    must_int!(c, "HDEL", "nosuch", "field"; 0);
}

#[tokio::test]
async fn test_hexists() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "HSET", "h", "field", "val"; 1);
    must_int!(c, "HEXISTS", "h", "field"; 1);
    must_int!(c, "HEXISTS", "h", "nosuch"; 0);
    must_int!(c, "HEXISTS", "nosuch", "field"; 0);
}

#[tokio::test]
async fn test_hgetall() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "HMSET", "h", "a", "1", "b", "2");

    // HGETALL returns field-value pairs
    let result: Vec<String> = redis::cmd("HGETALL")
        .arg("h")
        .query_async(&mut c)
        .await
        .unwrap();
    // Should be sorted by field name: a, 1, b, 2
    assert_eq!(result, vec!["a", "1", "b", "2"]);

    // Empty key
    let result: Vec<String> = redis::cmd("HGETALL")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(result.is_empty());
}

#[tokio::test]
async fn test_hkeys_hvals() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "HMSET", "h", "b", "2", "a", "1", "c", "3");

    must_strs!(c, "HKEYS", "h"; ["a", "b", "c"]);
    must_strs!(c, "HVALS", "h"; ["1", "2", "3"]);
}

#[tokio::test]
async fn test_hlen() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "HLEN", "h"; 0);
    must_ok!(c, "HMSET", "h", "a", "1", "b", "2");
    must_int!(c, "HLEN", "h"; 2);
}

#[tokio::test]
async fn test_hincrby() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "HINCRBY", "h", "field", "5"; 5);
    must_int!(c, "HINCRBY", "h", "field", "3"; 8);
    must_int!(c, "HINCRBY", "h", "field", "-2"; 6);

    // Non-integer value
    must_ok!(c, "HMSET", "h", "str", "notanumber");
    must_fail!(c, "HINCRBY", "h", "str", "1"; "not an integer");
}

#[tokio::test]
async fn test_hincrbyfloat() {
    let (_m, mut c) = helpers::start().await;

    must_str!(c, "HINCRBYFLOAT", "h", "field", "1.5"; "1.5");
    must_str!(c, "HINCRBYFLOAT", "h", "field", "2.5"; "4");
    must_str!(c, "HINCRBYFLOAT", "h", "field", "-1"; "3");
}

#[tokio::test]
async fn test_hstrlen() {
    let (_m, mut c) = helpers::start().await;

    must_int!(c, "HSET", "h", "field", "hello"; 1);
    must_int!(c, "HSTRLEN", "h", "field"; 5);
    must_int!(c, "HSTRLEN", "h", "nosuch"; 0);
    must_int!(c, "HSTRLEN", "nosuch", "field"; 0);
}

#[tokio::test]
async fn test_hash_wrongtype() {
    let (_m, mut c) = helpers::start().await;

    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "HGET", "str", "field"; "WRONGTYPE");
    must_fail!(c, "HSET", "str", "field", "val"; "WRONGTYPE");
}

#[tokio::test]
async fn test_hscan() {
    let (_m, mut c) = helpers::start().await;

    let _: i64 = redis::cmd("HSET")
        .arg("h")
        .arg("field1")
        .arg("value1")
        .arg("field2")
        .arg("value2")
        .query_async(&mut c)
        .await
        .unwrap();

    // Basic scan
    let (cursor, vals): (String, Vec<String>) = redis::cmd("HSCAN")
        .arg("h")
        .arg(0)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    // Returns [field1, value1, field2, value2]
    assert_eq!(vals.len(), 4);
    assert!(vals.contains(&"field1".to_string()));
    assert!(vals.contains(&"value1".to_string()));

    // MATCH
    let _: i64 = redis::cmd("HSET")
        .arg("h2")
        .arg("aap")
        .arg("a")
        .arg("noot")
        .arg("b")
        .arg("mies")
        .arg("m")
        .query_async(&mut c)
        .await
        .unwrap();
    let (cursor, vals): (String, Vec<String>) = redis::cmd("HSCAN")
        .arg("h2")
        .arg(0)
        .arg("MATCH")
        .arg("mi*")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(cursor, "0");
    assert_eq!(vals, vec!["mies", "m"]);

    // Errors
    must_fail!(c, "HSCAN"; "wrong number of arguments");
    must_fail!(c, "HSCAN", "h"; "wrong number of arguments");
    must_fail!(c, "HSCAN", "h", "noint"; "invalid cursor");
    must_fail!(c, "HSCAN", "h", "0", "MATCH"; "syntax error");
    must_fail!(c, "HSCAN", "h", "0", "COUNT"; "syntax error");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "HSCAN", "str", "0"; "WRONGTYPE");
}

#[tokio::test]
async fn test_hrandfield() {
    let (_m, mut c) = helpers::start().await;

    let _: i64 = redis::cmd("HSET")
        .arg("h")
        .arg("f1")
        .arg("v1")
        .arg("f2")
        .arg("v2")
        .arg("f3")
        .arg("v3")
        .arg("f4")
        .arg("v4")
        .query_async(&mut c)
        .await
        .unwrap();

    // Single field (no count)
    let v: String = redis::cmd("HRANDFIELD")
        .arg("h")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v.starts_with("f"));

    // Positive count
    let vals: Vec<String> = redis::cmd("HRANDFIELD")
        .arg("h")
        .arg(2)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 2);

    // Count larger than hash
    let vals: Vec<String> = redis::cmd("HRANDFIELD")
        .arg("h")
        .arg(10)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 4);

    // Negative count (allows duplicates)
    let vals: Vec<String> = redis::cmd("HRANDFIELD")
        .arg("h")
        .arg(-6)
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 6);

    // WITHVALUES
    let vals: Vec<String> = redis::cmd("HRANDFIELD")
        .arg("h")
        .arg(1)
        .arg("WITHVALUES")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(vals.len(), 2); // [field, value]

    // Nonexistent key
    must_nil!(c, "HRANDFIELD", "nosuch");

    // Errors
    must_fail!(c, "HRANDFIELD"; "wrong number of arguments");
    must_fail!(c, "HRANDFIELD", "h", "noint"; "not an integer");
}

#[tokio::test]
async fn test_hexpire() {
    let (m, mut c) = helpers::start().await;

    // Basic expiration
    let _: i64 = redis::cmd("HSET")
        .arg("myhash")
        .arg("field1")
        .arg("value1")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("myhash")
        .arg(10)
        .arg("FIELDS")
        .arg(1)
        .arg("field1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1]);

    // Multiple fields
    let _: i64 = redis::cmd("HSET")
        .arg("myhash2")
        .arg("field1")
        .arg("value1")
        .arg("field2")
        .arg("value2")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("myhash2")
        .arg(20)
        .arg("FIELDS")
        .arg(2)
        .arg("field1")
        .arg("field2")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1, 1]);

    // Nonexistent field
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("myhash")
        .arg(10)
        .arg("FIELDS")
        .arg(1)
        .arg("nonexistent")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![-2]);

    // Nonexistent key
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("nokey")
        .arg(10)
        .arg("FIELDS")
        .arg(1)
        .arg("field1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![-2]);

    // NX option
    let _: i64 = redis::cmd("HSET")
        .arg("nxhash")
        .arg("f1")
        .arg("v1")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("nxhash")
        .arg(10)
        .arg("NX")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1]);
    // NX again → 0 (already has TTL)
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("nxhash")
        .arg(20)
        .arg("NX")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![0]);

    // XX option: no TTL → 0
    let _: i64 = redis::cmd("HSET")
        .arg("xxhash")
        .arg("f1")
        .arg("v1")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("xxhash")
        .arg(10)
        .arg("XX")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![0]);
    // Set TTL first, then XX → 1
    let _: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("xxhash")
        .arg(10)
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("xxhash")
        .arg(20)
        .arg("XX")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1]);

    // GT option
    let _: i64 = redis::cmd("HSET")
        .arg("gthash")
        .arg("f1")
        .arg("v1")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("gthash")
        .arg(10)
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    // GT with smaller → 0
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("gthash")
        .arg(5)
        .arg("GT")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![0]);
    // GT with larger → 1
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("gthash")
        .arg(20)
        .arg("GT")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1]);

    // LT option
    let _: i64 = redis::cmd("HSET")
        .arg("lthash")
        .arg("f1")
        .arg("v1")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("lthash")
        .arg(20)
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    // LT with larger → 0
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("lthash")
        .arg(30)
        .arg("LT")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![0]);
    // LT with smaller → 1
    let result: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("lthash")
        .arg(10)
        .arg("LT")
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(result, vec![1]);

    // Field actually expires via fast_forward
    let _: i64 = redis::cmd("HSET")
        .arg("hash6")
        .arg("f1")
        .arg("v1")
        .arg("f2")
        .arg("v2")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("hash6")
        .arg(1)
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    m.fast_forward(std::time::Duration::from_secs(2));
    must_nil!(c, "HGET", "hash6", "f1");
    must_str!(c, "HGET", "hash6", "f2"; "v2");

    // All fields expired → key deleted
    let _: i64 = redis::cmd("HSET")
        .arg("hash7")
        .arg("f1")
        .arg("v1")
        .query_async(&mut c)
        .await
        .unwrap();
    let _: Vec<i64> = redis::cmd("HEXPIRE")
        .arg("hash7")
        .arg(1)
        .arg("FIELDS")
        .arg(1)
        .arg("f1")
        .query_async(&mut c)
        .await
        .unwrap();
    m.fast_forward(std::time::Duration::from_secs(2));
    must_int!(c, "EXISTS", "hash7"; 0);

    // Errors
    must_fail!(c, "HEXPIRE", "myhash"; "wrong number of arguments");
    must_ok!(c, "SET", "str", "val");
    must_fail!(c, "HEXPIRE", "str", "10", "FIELDS", "1", "f"; "WRONGTYPE");
    must_fail!(c, "HEXPIRE", "myhash", "notanumber", "FIELDS", "1", "f"; "not an integer");
    must_fail!(c, "HEXPIRE", "myhash", "10", "FIELDS", "0"; "wrong number of arguments");
    must_fail!(c, "HEXPIRE", "myhash", "10", "FIELDS", "2", "f"; "numfields");
    must_fail!(c, "HEXPIRE", "myhash", "10", "GT", "LT", "FIELDS", "1", "f"; "GT and LT");
    must_fail!(c, "HEXPIRE", "myhash", "10", "NX", "XX", "FIELDS", "1", "f"; "NX and XX");
}
