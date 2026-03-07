use miniredis_rs::Miniredis;

// ── String operations ────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_set_get() {
    let m = Miniredis::run().await.unwrap();
    m.set("key", "value");
    assert_eq!(m.get("key"), Some("value".to_string()));
    assert_eq!(m.get("nosuch"), None);
}

#[tokio::test]
async fn test_direct_incr() {
    let m = Miniredis::run().await.unwrap();
    assert_eq!(m.incr("counter", 1), 1);
    assert_eq!(m.incr("counter", 5), 6);
    assert_eq!(m.incr("counter", -2), 4);
}

#[tokio::test]
async fn test_direct_check_get() {
    let m = Miniredis::run().await.unwrap();
    m.set("k", "v");
    m.check_get("k", "v");
}

// ── Key management ───────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_del_exists() {
    let m = Miniredis::run().await.unwrap();
    m.set("k", "v");
    assert!(m.exists("k"));
    assert!(m.del("k"));
    assert!(!m.exists("k"));
    assert!(!m.del("k"));
}

#[tokio::test]
async fn test_direct_keys() {
    let m = Miniredis::run().await.unwrap();
    m.set("b", "1");
    m.set("a", "2");
    m.set("c", "3");
    assert_eq!(m.keys(), vec!["a", "b", "c"]);
}

#[tokio::test]
async fn test_direct_key_type() {
    let m = Miniredis::run().await.unwrap();
    m.set("str", "v");
    assert_eq!(m.key_type("str"), "string");
    assert_eq!(m.key_type("nosuch"), "none");
}

#[tokio::test]
async fn test_direct_db_size() {
    let m = Miniredis::run().await.unwrap();
    assert_eq!(m.db_size(), 0);
    m.set("k1", "v");
    m.set("k2", "v");
    assert_eq!(m.db_size(), 2);
}

#[tokio::test]
async fn test_direct_flush() {
    let m = Miniredis::run().await.unwrap();
    m.set("k1", "v");
    m.set("k2", "v");
    m.flush_db();
    assert_eq!(m.db_size(), 0);
}

// ── List operations ──────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_list_push_pop() {
    let m = Miniredis::run().await.unwrap();
    assert_eq!(m.push("l", &["a", "b", "c"]), 3);
    assert_eq!(
        m.list("l"),
        Some(vec!["a".to_string(), "b".to_string(), "c".to_string()])
    );

    assert_eq!(m.pop("l"), Some("c".to_string()));
    assert_eq!(m.lpop("l"), Some("a".to_string()));
    assert_eq!(m.list("l"), Some(vec!["b".to_string()]));
}

#[tokio::test]
async fn test_direct_list_lpush() {
    let m = Miniredis::run().await.unwrap();
    m.lpush("l", "a");
    m.lpush("l", "b");
    assert_eq!(m.list("l"), Some(vec!["b".to_string(), "a".to_string()]));
}

#[tokio::test]
async fn test_direct_check_list() {
    let m = Miniredis::run().await.unwrap();
    m.push("l", &["x", "y", "z"]);
    m.check_list("l", &["x", "y", "z"]);
}

// ── Set operations ───────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_set_add_members() {
    let m = Miniredis::run().await.unwrap();
    assert_eq!(m.set_add("s", &["a", "b", "c"]), 3);
    assert_eq!(m.set_add("s", &["b", "d"]), 1); // only d is new

    let members = m.members("s").unwrap();
    assert_eq!(members, vec!["a", "b", "c", "d"]);
}

#[tokio::test]
async fn test_direct_is_member() {
    let m = Miniredis::run().await.unwrap();
    m.set_add("s", &["a", "b"]);
    assert!(m.is_member("s", "a"));
    assert!(!m.is_member("s", "c"));
    assert!(!m.is_member("nosuch", "a"));
}

#[tokio::test]
async fn test_direct_check_set() {
    let m = Miniredis::run().await.unwrap();
    m.set_add("s", &["c", "a", "b"]);
    m.check_set("s", &["a", "b", "c"]);
}

// ── Hash operations ──────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_hash() {
    let m = Miniredis::run().await.unwrap();
    m.hset("h", "f1", "v1");
    m.hset("h", "f2", "v2");

    assert_eq!(m.hget("h", "f1"), Some("v1".to_string()));
    assert_eq!(m.hget("h", "nosuch"), None);

    let keys = m.hkeys("h").unwrap();
    assert_eq!(keys, vec!["f1", "f2"]);
}

#[tokio::test]
async fn test_direct_hdel() {
    let m = Miniredis::run().await.unwrap();
    m.hset("h", "f1", "v1");
    assert!(m.hdel("h", "f1"));
    assert!(!m.hdel("h", "f1"));
    assert_eq!(m.hget("h", "f1"), None);
}

// ── Sorted set operations ────────────────────────────────────────────

#[tokio::test]
async fn test_direct_sorted_set() {
    let m = Miniredis::run().await.unwrap();
    assert!(m.zadd("ss", 1.0, "a"));
    assert!(m.zadd("ss", 3.0, "c"));
    assert!(m.zadd("ss", 2.0, "b"));

    // Update existing
    assert!(!m.zadd("ss", 1.5, "a"));

    assert_eq!(m.zscore("ss", "a"), Some(1.5));
    assert_eq!(m.zscore("ss", "nosuch"), None);

    let members = m.zmembers("ss").unwrap();
    assert_eq!(members, vec!["a", "b", "c"]); // sorted by score
}

// ── Stream operations ────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_stream() {
    let m = Miniredis::run().await.unwrap();
    let id = m.xadd("stream", "1-0", &[("field", "value")]);
    assert_eq!(id, "1-0");
    assert_eq!(m.key_type("stream"), "stream");
}

// ── HyperLogLog operations ──────────────────────────────────────────

#[tokio::test]
async fn test_direct_hll() {
    let m = Miniredis::run().await.unwrap();
    assert!(m.pfadd("hll", &["a", "b", "c"]));
    assert!(!m.pfadd("hll", &["a", "b"])); // no new elements
    assert_eq!(m.pfcount("hll"), 3);
}

// ── TTL operations ───────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_ttl() {
    let m = Miniredis::run().await.unwrap();
    m.set("k", "v");
    assert_eq!(m.ttl("k"), None);

    m.set_ttl("k", std::time::Duration::from_secs(60));
    assert!(m.ttl("k").is_some());
}

// ── Select DB ────────────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_select_db() {
    let mut m = Miniredis::run().await.unwrap();
    m.set("k", "db0");
    m.select(1);
    m.set("k", "db1");
    assert_eq!(m.get("k"), Some("db1".to_string()));
    m.select(0);
    assert_eq!(m.get("k"), Some("db0".to_string()));
}

// ── Connection counting ──────────────────────────────────────────────

#[tokio::test]
async fn test_direct_connection_count() {
    let m = Miniredis::run().await.unwrap();

    // Before any connections
    assert_eq!(m.total_connection_count(), 0);

    // Create a client connection
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut conn = client.get_multiplexed_async_connection().await.unwrap();

    // Small delay for the connection to register
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    assert!(m.total_connection_count() > 0);
    assert!(m.current_connection_count() > 0);

    // Do a command to confirm it works
    let _: String = redis::cmd("PING").query_async(&mut conn).await.unwrap();

    drop(conn);
}

// ── Fast forward ─────────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_fast_forward() {
    let m = Miniredis::run().await.unwrap();
    m.set("k", "v");
    m.set_ttl("k", std::time::Duration::from_secs(10));

    // Key exists before expiration
    assert!(m.exists("k"));

    // Fast forward past TTL
    m.fast_forward(std::time::Duration::from_secs(11));

    // Key should be gone
    assert!(!m.exists("k"));
}

// ── Auth ─────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_require_auth() {
    let m = Miniredis::run().await.unwrap();
    m.require_auth("secret");

    // Connection without auth should fail
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut conn = client.get_multiplexed_async_connection().await.unwrap();

    let result: redis::RedisResult<String> = redis::cmd("PING").query_async(&mut conn).await;
    assert!(result.is_err());
}

// ── DB() access ─────────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_db_access() {
    let m = Miniredis::run().await.unwrap();

    // Set key in DB 0
    m.set("key0", "val0");

    // Set key in DB 5 via db()
    m.db(5).set("key5", "val5");

    // Keys are isolated
    assert_eq!(m.db(0).get("key0"), Some("val0".to_string()));
    assert_eq!(m.db(0).get("key5"), None);
    assert_eq!(m.db(5).get("key5"), Some("val5".to_string()));
    assert_eq!(m.db(5).get("key0"), None);

    // db() methods
    assert!(m.db(5).exists("key5"));
    assert_eq!(m.db(5).key_type("key5"), "string");
    assert_eq!(m.db(5).db_size(), 1);
    assert_eq!(m.db(5).keys(), vec!["key5".to_string()]);
}

// ── Restart ─────────────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_restart() {
    let mut m = Miniredis::run().await.unwrap();
    m.set("before", "restart");

    let old_addr = m.addr();
    m.close().await;
    tokio::task::yield_now().await;

    m.restart().await.unwrap();
    let new_addr = m.addr();

    // Port should change (new random port)
    assert_ne!(old_addr.port(), new_addr.port());

    // Data should be preserved
    assert_eq!(m.get("before"), Some("restart".to_string()));

    // New connections should work
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut conn = client.get_multiplexed_async_connection().await.unwrap();
    let v: String = redis::cmd("GET")
        .arg("before")
        .query_async(&mut conn)
        .await
        .unwrap();
    assert_eq!(v, "restart");
}

// ── Dump ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_direct_dump() {
    let m = Miniredis::run().await.unwrap();
    m.set("str", "hello");
    m.push("mylist", &["a", "b", "c"]);
    m.set_add("myset", &["x", "y"]);
    m.hset("myhash", "f1", "v1");
    m.zadd("myzset", 1.5, "member1");

    let dump = m.dump();

    assert!(dump.contains("- str\n"), "missing string key");
    assert!(dump.contains("\"hello\""), "missing string value");
    assert!(dump.contains("- mylist\n"), "missing list key");
    assert!(dump.contains("\"a\""), "missing list element");
    assert!(dump.contains("- myset\n"), "missing set key");
    assert!(dump.contains("- myhash\n"), "missing hash key");
    assert!(dump.contains("f1:"), "missing hash field");
    assert!(dump.contains("- myzset\n"), "missing zset key");
    assert!(dump.contains("1.5:"), "missing zset score");
}

#[tokio::test]
async fn test_direct_db_dump() {
    let m = Miniredis::run().await.unwrap();
    m.db(3).set("k", "v");

    // DB 0 dump should be empty
    let dump0 = m.db(0).dump();
    assert!(dump0.is_empty(), "DB 0 should be empty");

    // DB 3 dump should have the key
    let dump3 = m.db(3).dump();
    assert!(dump3.contains("- k\n"), "DB 3 should have key k");
}
