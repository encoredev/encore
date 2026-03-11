use redis::AsyncCommands;

#[tokio::test]
async fn smoke_ping() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    let pong: String = redis::cmd("PING").query_async(&mut con).await.unwrap();
    assert_eq!(pong, "PONG");

    m.close().await;
}

#[tokio::test]
async fn smoke_set_get() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    let _: () = con.set("foo", "bar").await.unwrap();
    let val: String = con.get("foo").await.unwrap();
    assert_eq!(val, "bar");

    m.close().await;
}

#[tokio::test]
async fn smoke_set_get_direct() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    // Set via client
    let _: () = con.set("key1", "value1").await.unwrap();

    // Read via direct API
    m.check_get("key1", "value1");

    // Set via direct API
    m.set("key2", "value2");

    // Read via client
    let val: String = con.get("key2").await.unwrap();
    assert_eq!(val, "value2");

    m.close().await;
}

#[tokio::test]
async fn smoke_del_exists() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    let _: () = con.set("k", "v").await.unwrap();
    assert!(m.exists("k"));

    let deleted: i64 = con.del("k").await.unwrap();
    assert_eq!(deleted, 1);
    assert!(!m.exists("k"));

    m.close().await;
}

#[tokio::test]
async fn smoke_echo() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    let result: String = redis::cmd("ECHO")
        .arg("hello world")
        .query_async(&mut con)
        .await
        .unwrap();
    assert_eq!(result, "hello world");

    m.close().await;
}

#[tokio::test]
async fn smoke_dbsize_flushdb() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    let _: () = con.set("a", "1").await.unwrap();
    let _: () = con.set("b", "2").await.unwrap();

    let size: i64 = redis::cmd("DBSIZE").query_async(&mut con).await.unwrap();
    assert_eq!(size, 2);

    let _: () = redis::cmd("FLUSHDB").query_async(&mut con).await.unwrap();
    let size: i64 = redis::cmd("DBSIZE").query_async(&mut con).await.unwrap();
    assert_eq!(size, 0);

    m.close().await;
}

#[tokio::test]
async fn smoke_set_nx_xx() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    // NX: set only if not exists
    let result: bool = con.set_nx("nxkey", "first").await.unwrap();
    assert!(result);

    let result: bool = con.set_nx("nxkey", "second").await.unwrap();
    assert!(!result);

    let val: String = con.get("nxkey").await.unwrap();
    assert_eq!(val, "first");

    m.close().await;
}

#[tokio::test]
async fn smoke_select_db() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    let _: () = con.set("key", "db0").await.unwrap();

    // SELECT 1 and set a different value
    let _: () = redis::cmd("SELECT")
        .arg(1)
        .query_async(&mut con)
        .await
        .unwrap();
    let _: () = con.set("key", "db1").await.unwrap();

    // Back to db 0
    let _: () = redis::cmd("SELECT")
        .arg(0)
        .query_async(&mut con)
        .await
        .unwrap();
    let val: String = con.get("key").await.unwrap();
    assert_eq!(val, "db0");

    m.close().await;
}

#[tokio::test]
async fn smoke_fast_forward() {
    let m = miniredis_rs::Miniredis::run().await.unwrap();
    let client = redis::Client::open(m.redis_url()).unwrap();
    let mut con = client.get_multiplexed_async_connection().await.unwrap();

    // SET with EX 10
    let _: () = redis::cmd("SET")
        .arg("temp")
        .arg("val")
        .arg("EX")
        .arg(10)
        .query_async(&mut con)
        .await
        .unwrap();

    // Key should exist
    let val: Option<String> = con.get("temp").await.unwrap();
    assert_eq!(val, Some("val".to_owned()));

    // Fast forward 11 seconds
    m.fast_forward(std::time::Duration::from_secs(11));

    // Key should be gone (lazy expiration on next access)
    let val: Option<String> = con.get("temp").await.unwrap();
    assert_eq!(val, None);

    m.close().await;
}
