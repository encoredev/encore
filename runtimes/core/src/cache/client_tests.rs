use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::OnceLock;

use crate::cache::client::{Client, ListDirection, TtlOp};
use crate::cache::error::Error;
use crate::cache::miniredis::MiniredisServer;
use crate::trace::Tracer;

static TEST_MINIREDIS: OnceLock<MiniredisServer> = OnceLock::new();
static TEST_COUNTER: AtomicU64 = AtomicU64::new(0);

fn new_test_pool() -> Client {
    let server = TEST_MINIREDIS.get_or_init(|| {
        // Spawn a dedicated thread with its own runtime for miniredis,
        // since we can't create a Runtime from within a #[tokio::test].
        let (tx, rx) = std::sync::mpsc::channel();
        std::thread::spawn(move || {
            let rt = tokio::runtime::Runtime::new().expect("failed to create runtime");
            let server = rt
                .block_on(MiniredisServer::start())
                .expect("failed to start miniredis for tests");
            tx.send(server).expect("failed to send server");
            // Park forever to keep the runtime (and its server task) alive.
            loop {
                std::thread::park();
            }
        });
        rx.recv().expect("failed to receive miniredis server")
    });
    let url = format!("redis://{}", server.addr());
    let client = bb8_redis::redis::Client::open(url).expect("failed to create redis client");
    // Use a unique key prefix per test to avoid interference between parallel tests.
    let id = TEST_COUNTER.fetch_add(1, Ordering::Relaxed);
    let prefix = format!("test{}:", id);
    Client::new(client, Some(prefix), Tracer::noop(), 0, 10).expect("failed to create cache client")
}

fn is_miss(err: &crate::cache::OpError) -> bool {
    matches!(err.source, Error::Miss)
}

fn is_key_exist(err: &crate::cache::OpError) -> bool {
    matches!(err.source, Error::KeyExist)
}

#[tokio::test]
async fn test_set_get_delete() {
    let p = new_test_pool();

    p.set("k", b"hello", None, None).await.unwrap();

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"hello".to_vec());

    let deleted = p.delete(&["k"], None).await.unwrap();
    assert_eq!(deleted, 1);

    let err = p.get("k", None).await.unwrap_err();
    assert!(is_miss(&err));
}

#[tokio::test]
async fn test_set_overwrites() {
    let p = new_test_pool();
    p.set("k", b"v1", None, None).await.unwrap();
    p.set("k", b"v2", None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_get_missing_key() {
    let p = new_test_pool();
    let err = p.get("missing", None).await.unwrap_err();
    assert!(is_miss(&err));
}

#[tokio::test]
async fn test_set_empty_value() {
    let p = new_test_pool();
    p.set("k", b"", None, None).await.unwrap();
    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"".to_vec());
}

#[tokio::test]
async fn test_set_binary_value() {
    let p = new_test_pool();
    let binary = vec![0u8, 1, 2, 255, 254, 0, 128];
    p.set("k", &binary, None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), binary);
}

#[tokio::test]
async fn test_set_large_value() {
    let p = new_test_pool();
    let large = vec![b'x'; 100_000];
    p.set("k", &large, None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), large);
}

#[tokio::test]
async fn test_delete_multiple() {
    let p = new_test_pool();
    p.set("a", b"1", None, None).await.unwrap();
    p.set("b", b"2", None, None).await.unwrap();
    p.set("c", b"3", None, None).await.unwrap();

    let deleted = p.delete(&["a", "c", "missing"], None).await.unwrap();
    assert_eq!(deleted, 2);
    assert!(is_miss(&p.get("a", None).await.unwrap_err()));
    assert_eq!(p.get("b", None).await.unwrap(), b"2".to_vec());
    assert!(is_miss(&p.get("c", None).await.unwrap_err()));
}

#[tokio::test]
async fn test_delete_all_missing() {
    let p = new_test_pool();
    let deleted = p.delete(&["x", "y", "z"], None).await.unwrap();
    assert_eq!(deleted, 0);
}

#[tokio::test]
async fn test_delete_single() {
    let p = new_test_pool();
    p.set("k", b"v", None, None).await.unwrap();
    assert_eq!(p.delete(&["k"], None).await.unwrap(), 1);
    assert!(is_miss(&p.get("k", None).await.unwrap_err()));
}

#[tokio::test]
async fn test_delete_idempotent() {
    let p = new_test_pool();
    p.set("k", b"v", None, None).await.unwrap();
    assert_eq!(p.delete(&["k"], None).await.unwrap(), 1);
    assert_eq!(p.delete(&["k"], None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_set_if_not_exists() {
    let p = new_test_pool();

    p.set_if_not_exists("k", b"v1", None, None).await.unwrap();

    let err = p
        .set_if_not_exists("k", b"v2", None, None)
        .await
        .unwrap_err();
    assert!(is_key_exist(&err));

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"v1".to_vec());
}

#[tokio::test]
async fn test_set_if_not_exists_with_ttl() {
    let p = new_test_pool();
    p.set_if_not_exists("k", b"v", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v".to_vec());
}

#[tokio::test]
async fn test_set_if_not_exists_after_delete() {
    let p = new_test_pool();
    p.set_if_not_exists("k", b"v1", None, None).await.unwrap();
    p.delete(&["k"], None).await.unwrap();
    p.set_if_not_exists("k", b"v2", None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_replace() {
    let p = new_test_pool();

    let err = p.replace("k", b"v1", None, None).await.unwrap_err();
    assert!(is_miss(&err));

    p.set("k", b"v1", None, None).await.unwrap();
    p.replace("k", b"v2", None, None).await.unwrap();

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"v2".to_vec());
}

#[tokio::test]
async fn test_replace_with_ttl() {
    let p = new_test_pool();
    p.set("k", b"v1", None, None).await.unwrap();
    p.replace("k", b"v2", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_replace_multiple_times() {
    let p = new_test_pool();
    p.set("k", b"v1", None, None).await.unwrap();
    p.replace("k", b"v2", None, None).await.unwrap();
    p.replace("k", b"v3", None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v3".to_vec());
}

#[tokio::test]
async fn test_get_and_set() {
    let p = new_test_pool();

    // get_and_set on missing key returns Miss but still sets the value.
    let err = p.get_and_set("k", b"v1", None, None).await.unwrap_err();
    assert!(is_miss(&err));

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"v1".to_vec());

    let old = p.get_and_set("k", b"v2", None, None).await.unwrap();
    assert_eq!(old, b"v1".to_vec());

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"v2".to_vec());
}

#[tokio::test]
async fn test_get_and_set_with_ttl() {
    let p = new_test_pool();
    p.set("k", b"v1", None, None).await.unwrap();
    let old = p
        .get_and_set("k", b"v2", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(old, b"v1".to_vec());
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_get_and_set_chain() {
    let p = new_test_pool();
    p.set("k", b"v1", None, None).await.unwrap();
    let old1 = p.get_and_set("k", b"v2", None, None).await.unwrap();
    let old2 = p.get_and_set("k", b"v3", None, None).await.unwrap();
    assert_eq!(old1, b"v1".to_vec());
    assert_eq!(old2, b"v2".to_vec());
    assert_eq!(p.get("k", None).await.unwrap(), b"v3".to_vec());
}

#[tokio::test]
async fn test_get_and_delete() {
    let p = new_test_pool();

    p.set("k", b"val", None, None).await.unwrap();

    let old = p.get_and_delete("k", None).await.unwrap();
    assert_eq!(old, b"val".to_vec());

    let err = p.get("k", None).await.unwrap_err();
    assert!(is_miss(&err));

    // Double delete returns Miss.
    let err = p.get_and_delete("k", None).await.unwrap_err();
    assert!(is_miss(&err));
}

#[tokio::test]
async fn test_get_and_delete_missing_key() {
    let p = new_test_pool();
    let err = p.get_and_delete("missing", None).await.unwrap_err();
    assert!(is_miss(&err));
}

#[tokio::test]
async fn test_mget() {
    let p = new_test_pool();

    p.set("a", b"1", None, None).await.unwrap();
    p.set("b", b"2", None, None).await.unwrap();

    let vals = p.mget(&["a", "b", "c"], None).await.unwrap();
    assert_eq!(vals.len(), 3);
    assert_eq!(vals[0], Some(b"1".to_vec()));
    assert_eq!(vals[1], Some(b"2".to_vec()));
    assert_eq!(vals[2], None);
}

#[tokio::test]
async fn test_mget_all_missing() {
    let p = new_test_pool();
    let vals = p.mget(&["x", "y"], None).await.unwrap();
    assert_eq!(vals, vec![None, None]);
}

#[tokio::test]
async fn test_mget_single_key() {
    let p = new_test_pool();
    p.set("k", b"v", None, None).await.unwrap();
    let vals = p.mget(&["k"], None).await.unwrap();
    assert_eq!(vals, vec![Some(b"v".to_vec())]);
}

#[tokio::test]
async fn test_mget_duplicate_keys() {
    let p = new_test_pool();
    p.set("k", b"v", None, None).await.unwrap();
    let vals = p.mget(&["k", "k"], None).await.unwrap();
    assert_eq!(vals, vec![Some(b"v".to_vec()), Some(b"v".to_vec())]);
}

#[tokio::test]
async fn test_append() {
    let p = new_test_pool();

    // Append to non-existent key creates it.
    let len = p.append("k", b"hello", None, None).await.unwrap();
    assert_eq!(len, 5);

    let len = p.append("k", b" world", None, None).await.unwrap();
    assert_eq!(len, 11);

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"hello world".to_vec());
}

#[tokio::test]
async fn test_append_empty() {
    let p = new_test_pool();
    p.set("k", b"hi", None, None).await.unwrap();
    let len = p.append("k", b"", None, None).await.unwrap();
    assert_eq!(len, 2);
    assert_eq!(p.get("k", None).await.unwrap(), b"hi".to_vec());
}

#[tokio::test]
async fn test_append_with_ttl() {
    let p = new_test_pool();
    let len = p
        .append("k", b"hello", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(len, 5);
    assert_eq!(p.get("k", None).await.unwrap(), b"hello".to_vec());
}

#[tokio::test]
async fn test_append_binary() {
    let p = new_test_pool();
    p.append("k", &[0u8, 1, 2], None, None).await.unwrap();
    p.append("k", &[3u8, 4], None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), vec![0, 1, 2, 3, 4]);
}

#[tokio::test]
async fn test_get_range() {
    let p = new_test_pool();

    p.set("k", b"hello world", None, None).await.unwrap();

    assert_eq!(
        p.get_range("k", 0, 4, None).await.unwrap(),
        b"hello".to_vec()
    );
    assert_eq!(
        p.get_range("k", 6, 10, None).await.unwrap(),
        b"world".to_vec()
    );

    // Negative indices.
    assert_eq!(
        p.get_range("k", -5, -1, None).await.unwrap(),
        b"world".to_vec()
    );

    // Missing key returns empty.
    assert_eq!(
        p.get_range("missing", 0, 10, None).await.unwrap(),
        b"".to_vec()
    );
}

#[tokio::test]
async fn test_get_range_full_string() {
    let p = new_test_pool();
    p.set("k", b"hello", None, None).await.unwrap();
    assert_eq!(
        p.get_range("k", 0, -1, None).await.unwrap(),
        b"hello".to_vec()
    );
}

#[tokio::test]
async fn test_get_range_single_char() {
    let p = new_test_pool();
    p.set("k", b"abc", None, None).await.unwrap();
    assert_eq!(p.get_range("k", 1, 1, None).await.unwrap(), b"b".to_vec());
}

#[tokio::test]
async fn test_get_range_beyond_end() {
    let p = new_test_pool();
    p.set("k", b"hi", None, None).await.unwrap();
    // Range past end returns what's available.
    assert_eq!(
        p.get_range("k", 0, 100, None).await.unwrap(),
        b"hi".to_vec()
    );
}

#[tokio::test]
async fn test_set_range() {
    let p = new_test_pool();

    p.set("k", b"hello world", None, None).await.unwrap();

    let new_len = p.set_range("k", 6, b"rust!", None, None).await.unwrap();
    assert_eq!(new_len, 11);
    assert_eq!(p.get("k", None).await.unwrap(), b"hello rust!".to_vec());

    // Set range past end extends the string.
    let new_len = p.set_range("k", 11, b"!!", None, None).await.unwrap();
    assert_eq!(new_len, 13);
    assert_eq!(p.get("k", None).await.unwrap(), b"hello rust!!!".to_vec());
}

#[tokio::test]
async fn test_set_range_with_gap() {
    let p = new_test_pool();

    // Setting at offset past current length pads with zeros.
    let len = p.set_range("k", 5, b"hi", None, None).await.unwrap();
    assert_eq!(len, 7);
    let val = p.get("k", None).await.unwrap();
    assert_eq!(&val[..5], &[0, 0, 0, 0, 0]);
    assert_eq!(&val[5..], b"hi");
}

#[tokio::test]
async fn test_set_range_on_missing_key() {
    let p = new_test_pool();
    // SETRANGE on missing key at offset 0 creates the key.
    let len = p.set_range("k", 0, b"abc", None, None).await.unwrap();
    assert_eq!(len, 3);
    assert_eq!(p.get("k", None).await.unwrap(), b"abc".to_vec());
}

#[tokio::test]
async fn test_set_range_empty_value() {
    let p = new_test_pool();
    p.set("k", b"hello", None, None).await.unwrap();
    let len = p.set_range("k", 2, b"", None, None).await.unwrap();
    assert_eq!(len, 5);
    assert_eq!(p.get("k", None).await.unwrap(), b"hello".to_vec());
}

#[tokio::test]
async fn test_set_range_with_ttl() {
    let p = new_test_pool();
    p.set("k", b"hello", None, None).await.unwrap();
    let len = p
        .set_range("k", 0, b"HE", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(len, 5);
    assert_eq!(p.get("k", None).await.unwrap(), b"HEllo".to_vec());
}

#[tokio::test]
async fn test_strlen() {
    let p = new_test_pool();

    assert_eq!(p.strlen("missing", None).await.unwrap(), 0);

    p.set("k", b"hello", None, None).await.unwrap();
    assert_eq!(p.strlen("k", None).await.unwrap(), 5);
}

#[tokio::test]
async fn test_strlen_empty_value() {
    let p = new_test_pool();
    p.set("k", b"", None, None).await.unwrap();
    assert_eq!(p.strlen("k", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_strlen_after_append() {
    let p = new_test_pool();
    p.set("k", b"hi", None, None).await.unwrap();
    p.append("k", b"!!!", None, None).await.unwrap();
    assert_eq!(p.strlen("k", None).await.unwrap(), 5);
}

#[tokio::test]
async fn test_incr_by() {
    let p = new_test_pool();

    p.set("k", b"10", None, None).await.unwrap();

    let v = p.incr_by("k", 5, None, None).await.unwrap();
    assert_eq!(v, 15);

    let v = p.decr_by("k", 3, None, None).await.unwrap();
    assert_eq!(v, 12);

    // Negative delta acts as decrement.
    let v = p.incr_by("k", -2, None, None).await.unwrap();
    assert_eq!(v, 10);
}

#[tokio::test]
async fn test_incr_creates_key() {
    let p = new_test_pool();

    let v = p.incr_by("k", 7, None, None).await.unwrap();
    assert_eq!(v, 7);

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, b"7".to_vec());
}

#[tokio::test]
async fn test_incr_by_invalid_value() {
    let p = new_test_pool();
    p.set("k", b"not-a-number", None, None).await.unwrap();
    assert!(p.incr_by("k", 1, None, None).await.is_err());
}

#[tokio::test]
async fn test_decr_creates_key() {
    let p = new_test_pool();
    let v = p.decr_by("k", 3, None, None).await.unwrap();
    assert_eq!(v, -3);
    assert_eq!(p.get("k", None).await.unwrap(), b"-3".to_vec());
}

#[tokio::test]
async fn test_incr_by_zero() {
    let p = new_test_pool();
    p.set("k", b"5", None, None).await.unwrap();
    let v = p.incr_by("k", 0, None, None).await.unwrap();
    assert_eq!(v, 5);
}

#[tokio::test]
async fn test_incr_negative_value() {
    let p = new_test_pool();
    p.set("k", b"-10", None, None).await.unwrap();
    let v = p.incr_by("k", 3, None, None).await.unwrap();
    assert_eq!(v, -7);
}

#[tokio::test]
async fn test_incr_with_ttl() {
    let p = new_test_pool();
    let v = p
        .incr_by("k", 1, Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(v, 1);
    assert_eq!(p.get("k", None).await.unwrap(), b"1".to_vec());
}

#[tokio::test]
async fn test_decr_with_ttl() {
    let p = new_test_pool();
    p.set("k", b"100", None, None).await.unwrap();
    let v = p
        .decr_by("k", 25, Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(v, 75);
}

#[tokio::test]
async fn test_incr_by_float() {
    let p = new_test_pool();

    let v = p.incr_by_float("k", 1.5, None, None).await.unwrap();
    assert!((v - 1.5).abs() < f64::EPSILON);

    let v = p.incr_by_float("k", 2.5, None, None).await.unwrap();
    assert!((v - 4.0).abs() < f64::EPSILON);

    let v = p.incr_by_float("k", -1.0, None, None).await.unwrap();
    assert!((v - 3.0).abs() < f64::EPSILON);
}

#[tokio::test]
async fn test_incr_by_float_on_invalid_value() {
    let p = new_test_pool();
    p.set("k", b"not-a-number", None, None).await.unwrap();
    assert!(p.incr_by_float("k", 1.0, None, None).await.is_err());
}

#[tokio::test]
async fn test_incr_by_float_on_integer_string() {
    let p = new_test_pool();
    p.set("k", b"10", None, None).await.unwrap();
    let v = p.incr_by_float("k", 0.5, None, None).await.unwrap();
    assert!((v - 10.5).abs() < f64::EPSILON);
}

#[tokio::test]
async fn test_incr_by_float_with_ttl() {
    let p = new_test_pool();
    let v = p
        .incr_by_float("k", 3.125, Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert!((v - 3.125).abs() < 0.001);
}

#[tokio::test]
async fn test_list_push_pop() {
    let p = new_test_pool();

    let len = p.lpush("l", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(len, 2);

    let len = p.rpush("l", &[b"c", b"d"], None, None).await.unwrap();
    assert_eq!(len, 4);

    // LPUSH pushes left-to-right, so [a, b] becomes [b, a] at the head.
    let items = p.lrange_all("l", None).await.unwrap();
    assert_eq!(items, vec![b"b", b"a", b"c", b"d"]);

    let val = p.lpop("l", None, None).await.unwrap();
    assert_eq!(val, b"b".to_vec());

    let val = p.rpop("l", None, None).await.unwrap();
    assert_eq!(val, b"d".to_vec());
}

#[tokio::test]
async fn test_lpush_single() {
    let p = new_test_pool();
    let len = p.lpush("l", &[b"a"], None, None).await.unwrap();
    assert_eq!(len, 1);
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_rpush_single() {
    let p = new_test_pool();
    let len = p.rpush("l", &[b"a"], None, None).await.unwrap();
    assert_eq!(len, 1);
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_lpush_extends_existing() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    let len = p.lpush("l", &[b"z"], None, None).await.unwrap();
    assert_eq!(len, 3);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"z".to_vec(), b"a".to_vec(), b"b".to_vec()]
    );
}

#[tokio::test]
async fn test_rpush_extends_existing() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    let len = p.rpush("l", &[b"c"], None, None).await.unwrap();
    assert_eq!(len, 3);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );
}

#[tokio::test]
async fn test_lpush_with_ttl() {
    let p = new_test_pool();
    let len = p
        .lpush("l", &[b"a"], Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(len, 1);
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_rpush_with_ttl() {
    let p = new_test_pool();
    let len = p
        .rpush("l", &[b"a"], Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(len, 1);
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_lpush_multiple_ordering() {
    let p = new_test_pool();
    // LPUSH [a, b, c] results in [c, b, a] because each is pushed to head.
    p.lpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"c".to_vec(), b"b".to_vec(), b"a".to_vec()]
    );
}

#[tokio::test]
async fn test_lpop_rpop_empty() {
    let p = new_test_pool();
    assert!(is_miss(&p.lpop("missing", None, None).await.unwrap_err()));
    assert!(is_miss(&p.rpop("missing", None, None).await.unwrap_err()));
}

#[tokio::test]
async fn test_lpop_until_empty() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(p.lpop("l", None, None).await.unwrap(), b"a".to_vec());
    assert_eq!(p.lpop("l", None, None).await.unwrap(), b"b".to_vec());
    assert!(is_miss(&p.lpop("l", None, None).await.unwrap_err()));
}

#[tokio::test]
async fn test_rpop_until_empty() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(p.rpop("l", None, None).await.unwrap(), b"b".to_vec());
    assert_eq!(p.rpop("l", None, None).await.unwrap(), b"a".to_vec());
    assert!(is_miss(&p.rpop("l", None, None).await.unwrap_err()));
}

#[tokio::test]
async fn test_lpop_with_ttl() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    let v = p
        .lpop("l", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(v, b"a".to_vec());
}

#[tokio::test]
async fn test_rpop_with_ttl() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    let v = p
        .rpop("l", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(v, b"b".to_vec());
}

#[tokio::test]
async fn test_lindex() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();

    assert_eq!(p.lindex("l", 0, None).await.unwrap(), b"a".to_vec());
    assert_eq!(p.lindex("l", 2, None).await.unwrap(), b"c".to_vec());
    assert_eq!(p.lindex("l", -1, None).await.unwrap(), b"c".to_vec());
    assert_eq!(p.lindex("l", -3, None).await.unwrap(), b"a".to_vec());

    // Out of range.
    assert!(is_miss(&p.lindex("l", 10, None).await.unwrap_err()));
    assert!(is_miss(&p.lindex("l", -10, None).await.unwrap_err()));

    // Missing key.
    assert!(is_miss(&p.lindex("missing", 0, None).await.unwrap_err()));
}

#[tokio::test]
async fn test_lindex_single_element() {
    let p = new_test_pool();
    p.rpush("l", &[b"only"], None, None).await.unwrap();
    assert_eq!(p.lindex("l", 0, None).await.unwrap(), b"only".to_vec());
    assert_eq!(p.lindex("l", -1, None).await.unwrap(), b"only".to_vec());
    assert!(is_miss(&p.lindex("l", 1, None).await.unwrap_err()));
}

#[tokio::test]
async fn test_lset() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();

    p.lset("l", 1, b"B", None, None).await.unwrap();
    assert_eq!(p.lindex("l", 1, None).await.unwrap(), b"B".to_vec());

    // Negative index.
    p.lset("l", -1, b"C", None, None).await.unwrap();
    assert_eq!(p.lindex("l", 2, None).await.unwrap(), b"C".to_vec());
}

#[tokio::test]
async fn test_lset_out_of_range() {
    let p = new_test_pool();
    p.rpush("l", &[b"a"], None, None).await.unwrap();
    assert!(p.lset("l", 5, b"x", None, None).await.is_err());
}

#[tokio::test]
async fn test_lset_missing_key() {
    let p = new_test_pool();
    assert!(p.lset("missing", 0, b"x", None, None).await.is_err());
}

#[tokio::test]
async fn test_lset_negative_out_of_range() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    assert!(p.lset("l", -5, b"x", None, None).await.is_err());
}

#[tokio::test]
async fn test_lset_first_and_last() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.lset("l", 0, b"A", None, None).await.unwrap();
    p.lset("l", -1, b"C", None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"A".to_vec(), b"b".to_vec(), b"C".to_vec()]
    );
}

#[tokio::test]
async fn test_lrange() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c", b"d", b"e"], None, None)
        .await
        .unwrap();

    assert_eq!(
        p.lrange("l", 0, 2, None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );
    assert_eq!(
        p.lrange("l", -2, -1, None).await.unwrap(),
        vec![b"d".to_vec(), b"e".to_vec()]
    );

    // Empty range.
    assert_eq!(
        p.lrange("l", 5, 10, None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );

    // Missing key.
    assert_eq!(
        p.lrange("missing", 0, -1, None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[tokio::test]
async fn test_lrange_inverted() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    // start > stop returns empty list.
    assert_eq!(
        p.lrange("l", 2, 0, None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[tokio::test]
async fn test_list_range_items() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c", b"d"], None, None)
        .await
        .unwrap();

    let sub = p.lrange("l", 1, 2, None).await.unwrap();
    assert_eq!(sub, vec![b"b".to_vec(), b"c".to_vec()]);

    let all = p.lrange_all("l", None).await.unwrap();
    assert_eq!(all.len(), 4);
}

#[tokio::test]
async fn test_lrange_all_empty() {
    let p = new_test_pool();
    assert_eq!(
        p.lrange_all("missing", None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[tokio::test]
async fn test_ltrim() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c", b"d", b"e"], None, None)
        .await
        .unwrap();

    p.ltrim("l", 1, 3, None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"b".to_vec(), b"c".to_vec(), b"d".to_vec()]
    );
}

#[tokio::test]
async fn test_ltrim_clears_when_out_of_range() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    p.ltrim("l", 5, 10, None, None).await.unwrap();
    assert_eq!(p.llen("l", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_ltrim_negative_indices() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c", b"d"], None, None)
        .await
        .unwrap();
    p.ltrim("l", -3, -2, None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"b".to_vec(), b"c".to_vec()]
    );
}

#[tokio::test]
async fn test_ltrim_keep_all() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.ltrim("l", 0, -1, None, None).await.unwrap();
    assert_eq!(p.llen("l", None).await.unwrap(), 3);
}

#[tokio::test]
async fn test_ltrim_to_single() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.ltrim("l", 1, 1, None, None).await.unwrap();
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"b".to_vec()]);
}

#[tokio::test]
async fn test_ltrim_missing_key() {
    let p = new_test_pool();
    // LTRIM on missing key is a no-op (no error).
    p.ltrim("missing", 0, 1, None, None).await.unwrap();
}

#[tokio::test]
async fn test_linsert_before() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"c"], None, None).await.unwrap();

    let len = p.linsert_before("l", b"c", b"b", None, None).await.unwrap();
    assert_eq!(len, 3);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );

    // Pivot not found returns Miss.
    assert!(is_miss(
        &p.linsert_before("l", b"z", b"x", None, None)
            .await
            .unwrap_err()
    ));
}

#[tokio::test]
async fn test_linsert_after() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"c"], None, None).await.unwrap();

    let len = p.linsert_after("l", b"a", b"b", None, None).await.unwrap();
    assert_eq!(len, 3);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );

    // Pivot not found returns Miss.
    assert!(is_miss(
        &p.linsert_after("l", b"z", b"x", None, None)
            .await
            .unwrap_err()
    ));
}

#[tokio::test]
async fn test_linsert_missing_key() {
    let p = new_test_pool();
    // Redis LINSERT on a non-existent key returns 0 (not an error).
    let result = p
        .linsert_before("missing", b"a", b"b", None, None)
        .await
        .unwrap();
    assert_eq!(result, 0);
    let result = p
        .linsert_after("missing", b"a", b"b", None, None)
        .await
        .unwrap();
    assert_eq!(result, 0);
}

#[tokio::test]
async fn test_linsert_before_first() {
    let p = new_test_pool();
    p.rpush("l", &[b"b", b"c"], None, None).await.unwrap();
    p.linsert_before("l", b"b", b"a", None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );
}

#[tokio::test]
async fn test_linsert_after_last() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    p.linsert_after("l", b"b", b"c", None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );
}

#[tokio::test]
async fn test_linsert_with_duplicate_pivots() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"a", b"b"], None, None).await.unwrap();
    // LINSERT finds the first occurrence of the pivot.
    p.linsert_before("l", b"a", b"x", None, None).await.unwrap();
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"x".to_vec(), b"a".to_vec(), b"a".to_vec(), b"b".to_vec()]
    );
}

#[tokio::test]
async fn test_linsert_with_ttl() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"c"], None, None).await.unwrap();
    p.linsert_before("l", b"c", b"b", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(p.llen("l", None).await.unwrap(), 3);
}

#[tokio::test]
async fn test_lrem_first() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"a", b"c", b"a"], None, None)
        .await
        .unwrap();

    let removed = p.lrem_first("l", 2, b"a", None, None).await.unwrap();
    assert_eq!(removed, 2);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"b".to_vec(), b"c".to_vec(), b"a".to_vec()]
    );
}

#[tokio::test]
async fn test_lrem_last() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"a", b"c", b"a"], None, None)
        .await
        .unwrap();

    let removed = p.lrem_last("l", 2, b"a", None, None).await.unwrap();
    assert_eq!(removed, 2);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]
    );
}

#[tokio::test]
async fn test_lrem_all() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"a", b"c", b"a"], None, None)
        .await
        .unwrap();

    let removed = p.lrem_all("l", b"a", None, None).await.unwrap();
    assert_eq!(removed, 3);
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"b".to_vec(), b"c".to_vec()]
    );
}

#[tokio::test]
async fn test_lrem_missing_key() {
    let p = new_test_pool();
    assert_eq!(p.lrem_all("missing", b"a", None, None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_lrem_value_not_in_list() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    let removed = p.lrem_all("l", b"z", None, None).await.unwrap();
    assert_eq!(removed, 0);
    assert_eq!(p.llen("l", None).await.unwrap(), 3);
}

#[tokio::test]
async fn test_lrem_first_more_than_exist() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"a"], None, None).await.unwrap();
    // count=10 but only 2 occurrences of "a".
    let removed = p.lrem_first("l", 10, b"a", None, None).await.unwrap();
    assert_eq!(removed, 2);
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"b".to_vec()]);
}

#[tokio::test]
async fn test_lrem_last_more_than_exist() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"a"], None, None).await.unwrap();
    let removed = p.lrem_last("l", 10, b"a", None, None).await.unwrap();
    assert_eq!(removed, 2);
    assert_eq!(p.lrange_all("l", None).await.unwrap(), vec![b"b".to_vec()]);
}

#[tokio::test]
async fn test_lrem_first_zero_count() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"a"], None, None).await.unwrap();
    // count=0 in lrem_first should return 0 immediately (no-op).
    let removed = p.lrem_first("l", 0, b"a", None, None).await.unwrap();
    assert_eq!(removed, 0);
    assert_eq!(p.llen("l", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_lrem_last_zero_count() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"a"], None, None).await.unwrap();
    let removed = p.lrem_last("l", 0, b"a", None, None).await.unwrap();
    assert_eq!(removed, 0);
    assert_eq!(p.llen("l", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_lrem_all_empties_list() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"a", b"a"], None, None).await.unwrap();
    let removed = p.lrem_all("l", b"a", None, None).await.unwrap();
    assert_eq!(removed, 3);
    assert_eq!(p.llen("l", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_lmove() {
    let p = new_test_pool();
    p.rpush("src", &[b"a", b"b", b"c"], None, None)
        .await
        .unwrap();

    // Move left of src to right of dst.
    let val = p
        .lmove(
            "src",
            "dst",
            ListDirection::Left,
            ListDirection::Right,
            None,
            None,
        )
        .await
        .unwrap();
    assert_eq!(val, b"a".to_vec());

    assert_eq!(
        p.lrange_all("src", None).await.unwrap(),
        vec![b"b".to_vec(), b"c".to_vec()]
    );
    assert_eq!(
        p.lrange_all("dst", None).await.unwrap(),
        vec![b"a".to_vec()]
    );

    // Move right of src to left of dst.
    let val = p
        .lmove(
            "src",
            "dst",
            ListDirection::Right,
            ListDirection::Left,
            None,
            None,
        )
        .await
        .unwrap();
    assert_eq!(val, b"c".to_vec());

    assert_eq!(
        p.lrange_all("src", None).await.unwrap(),
        vec![b"b".to_vec()]
    );
    assert_eq!(
        p.lrange_all("dst", None).await.unwrap(),
        vec![b"c".to_vec(), b"a".to_vec()]
    );
}

#[tokio::test]
async fn test_lmove_empty_source() {
    let p = new_test_pool();
    assert!(is_miss(
        &p.lmove(
            "missing",
            "dst",
            ListDirection::Left,
            ListDirection::Right,
            None,
            None
        )
        .await
        .unwrap_err()
    ));
}

#[tokio::test]
async fn test_lmove_same_list_rotate() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();

    // Rotate: move right to left (rpoplpush on same key).
    let val = p
        .lmove(
            "l",
            "l",
            ListDirection::Right,
            ListDirection::Left,
            None,
            None,
        )
        .await
        .unwrap();
    assert_eq!(val, b"c".to_vec());
    assert_eq!(
        p.lrange_all("l", None).await.unwrap(),
        vec![b"c".to_vec(), b"a".to_vec(), b"b".to_vec()]
    );
}

#[tokio::test]
async fn test_lmove_left_to_left() {
    let p = new_test_pool();
    p.rpush("src", &[b"a", b"b"], None, None).await.unwrap();
    p.rpush("dst", &[b"x"], None, None).await.unwrap();

    let val = p
        .lmove(
            "src",
            "dst",
            ListDirection::Left,
            ListDirection::Left,
            None,
            None,
        )
        .await
        .unwrap();
    assert_eq!(val, b"a".to_vec());
    assert_eq!(
        p.lrange_all("dst", None).await.unwrap(),
        vec![b"a".to_vec(), b"x".to_vec()]
    );
}

#[tokio::test]
async fn test_lmove_right_to_right() {
    let p = new_test_pool();
    p.rpush("src", &[b"a", b"b"], None, None).await.unwrap();
    p.rpush("dst", &[b"x"], None, None).await.unwrap();

    let val = p
        .lmove(
            "src",
            "dst",
            ListDirection::Right,
            ListDirection::Right,
            None,
            None,
        )
        .await
        .unwrap();
    assert_eq!(val, b"b".to_vec());
    assert_eq!(
        p.lrange_all("dst", None).await.unwrap(),
        vec![b"x".to_vec(), b"b".to_vec()]
    );
}

#[tokio::test]
async fn test_lmove_creates_destination() {
    let p = new_test_pool();
    p.rpush("src", &[b"a"], None, None).await.unwrap();

    p.lmove(
        "src",
        "dst",
        ListDirection::Left,
        ListDirection::Right,
        None,
        None,
    )
    .await
    .unwrap();

    assert_eq!(p.llen("src", None).await.unwrap(), 0);
    assert_eq!(
        p.lrange_all("dst", None).await.unwrap(),
        vec![b"a".to_vec()]
    );
}

#[tokio::test]
async fn test_lmove_with_ttl() {
    let p = new_test_pool();
    p.rpush("src", &[b"a", b"b"], None, None).await.unwrap();
    let val = p
        .lmove(
            "src",
            "dst",
            ListDirection::Left,
            ListDirection::Right,
            Some(TtlOp::SetMs(100_000)),
            None,
        )
        .await
        .unwrap();
    assert_eq!(val, b"a".to_vec());
}

#[tokio::test]
async fn test_llen() {
    let p = new_test_pool();
    assert_eq!(p.llen("missing", None).await.unwrap(), 0);

    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    assert_eq!(p.llen("l", None).await.unwrap(), 3);
}

#[tokio::test]
async fn test_llen_after_pop() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.lpop("l", None, None).await.unwrap();
    assert_eq!(p.llen("l", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_set_add_remove() {
    let p = new_test_pool();

    let added = p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();
    assert_eq!(added, 3);

    // Adding duplicates.
    let added = p.sadd("s", &[b"b", b"c", b"d"], None, None).await.unwrap();
    assert_eq!(added, 1);

    let removed = p.srem("s", &[b"a", b"missing"], None, None).await.unwrap();
    assert_eq!(removed, 1);

    let len = p.scard("s", None).await.unwrap();
    assert_eq!(len, 3);
}

#[tokio::test]
async fn test_sadd_single() {
    let p = new_test_pool();
    let added = p.sadd("s", &[b"x"], None, None).await.unwrap();
    assert_eq!(added, 1);
    assert_eq!(p.scard("s", None).await.unwrap(), 1);
}

#[tokio::test]
async fn test_sadd_all_duplicates() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let added = p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(added, 0);
    assert_eq!(p.scard("s", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_sadd_with_ttl() {
    let p = new_test_pool();
    let added = p
        .sadd("s", &[b"a"], Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(added, 1);
}

#[tokio::test]
async fn test_srem_from_missing_key() {
    let p = new_test_pool();
    let removed = p.srem("missing", &[b"a"], None, None).await.unwrap();
    assert_eq!(removed, 0);
}

#[tokio::test]
async fn test_srem_all_members() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let removed = p.srem("s", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(removed, 2);
    assert_eq!(p.scard("s", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_srem_with_ttl() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();
    let removed = p
        .srem("s", &[b"a"], Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert_eq!(removed, 1);
    assert_eq!(p.scard("s", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_set_contains() {
    let p = new_test_pool();

    p.sadd("s", &[b"x"], None, None).await.unwrap();

    assert!(p.sismember("s", b"x", None).await.unwrap());
    assert!(!p.sismember("s", b"y", None).await.unwrap());
    assert!(!p.sismember("missing", b"x", None).await.unwrap());
}

#[tokio::test]
async fn test_sismember_after_remove() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    p.srem("s", &[b"a"], None, None).await.unwrap();
    assert!(!p.sismember("s", b"a", None).await.unwrap());
    assert!(p.sismember("s", b"b", None).await.unwrap());
}

#[tokio::test]
async fn test_set_members_len() {
    let p = new_test_pool();

    p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();

    let mut members = p.smembers("s", None).await.unwrap();
    members.sort();
    assert_eq!(members, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);

    assert_eq!(p.scard("s", None).await.unwrap(), 3);
    assert_eq!(p.scard("missing", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_smembers_empty() {
    let p = new_test_pool();
    assert_eq!(
        p.smembers("missing", None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[tokio::test]
async fn test_smembers_single() {
    let p = new_test_pool();
    p.sadd("s", &[b"only"], None, None).await.unwrap();
    assert_eq!(p.smembers("s", None).await.unwrap(), vec![b"only".to_vec()]);
}

#[tokio::test]
async fn test_spop() {
    let p = new_test_pool();

    p.sadd("s", &[b"only"], None, None).await.unwrap();

    // spop removes and returns the member.
    let popped = p.spop("s", None, None).await.unwrap();
    assert_eq!(popped, b"only".to_vec());

    // Empty set returns Miss.
    let err = p.spop("s", None, None).await.unwrap_err();
    assert!(is_miss(&err));
}

#[tokio::test]
async fn test_spop_missing_key() {
    let p = new_test_pool();
    assert!(is_miss(&p.spop("missing", None, None).await.unwrap_err()));
}

#[tokio::test]
async fn test_spop_reduces_cardinality() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();
    let popped = p.spop("s", None, None).await.unwrap();
    assert!(!popped.is_empty());
    assert_eq!(p.scard("s", None).await.unwrap(), 2);
    assert!(!p.sismember("s", &popped, None).await.unwrap());
}

#[tokio::test]
async fn test_spop_n() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();

    let popped = p.spop_n("s", 2, None, None).await.unwrap();
    assert_eq!(popped.len(), 2);
    assert_eq!(p.scard("s", None).await.unwrap(), 1);

    // Pop from missing key.
    assert_eq!(
        p.spop_n("missing", 1, None, None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[tokio::test]
async fn test_spop_n_more_than_set_size() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let popped = p.spop_n("s", 10, None, None).await.unwrap();
    assert_eq!(popped.len(), 2);
    assert_eq!(p.scard("s", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_spop_n_zero() {
    let p = new_test_pool();
    p.sadd("s", &[b"a"], None, None).await.unwrap();
    let popped = p.spop_n("s", 0, None, None).await.unwrap();
    assert!(popped.is_empty());
    assert_eq!(p.scard("s", None).await.unwrap(), 1);
}

#[tokio::test]
async fn test_srandmember() {
    let p = new_test_pool();

    // Empty set returns Miss.
    let err = p.srandmember("s", None).await.unwrap_err();
    assert!(is_miss(&err));

    p.sadd("s", &[b"m"], None, None).await.unwrap();
    let sampled = p.srandmember("s", None).await.unwrap();
    assert_eq!(sampled, b"m".to_vec());
}

#[tokio::test]
async fn test_srandmember_doesnt_remove() {
    let p = new_test_pool();
    p.sadd("s", &[b"a"], None, None).await.unwrap();
    p.srandmember("s", None).await.unwrap();
    assert_eq!(p.scard("s", None).await.unwrap(), 1);
}

#[tokio::test]
async fn test_srandmember_from_larger_set() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b", b"c", b"d", b"e"], None, None)
        .await
        .unwrap();
    let sampled = p.srandmember("s", None).await.unwrap();
    assert!(p.sismember("s", &sampled, None).await.unwrap());
}

#[tokio::test]
async fn test_srandmember_n() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();

    // Positive count — distinct members.
    let sample = p.srandmember_n("s", 2, None).await.unwrap();
    assert_eq!(sample.len(), 2);
    for m in &sample {
        assert!(p.sismember("s", m, None).await.unwrap());
    }

    // Positive count > set size returns at most set size.
    let sample = p.srandmember_n("s", 10, None).await.unwrap();
    assert_eq!(sample.len(), 3);

    // Empty/missing set.
    let result = p.srandmember_n("missing", 2, None).await;
    match result {
        Ok(v) => assert!(v.is_empty()),
        Err(e) => assert!(is_miss(&e)),
    }
}

#[tokio::test]
async fn test_srandmember_negative_count() {
    let p = new_test_pool();
    p.sadd("s", &[b"a"], None, None).await.unwrap();

    // Negative count — allows duplicates, returns exactly abs(count).
    let sample = p.srandmember_n("s", -5, None).await.unwrap();
    assert_eq!(sample.len(), 5);
    for m in &sample {
        assert_eq!(m, &b"a".to_vec());
    }
}

#[tokio::test]
async fn test_srandmember_n_distinct() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b", b"c", b"d", b"e"], None, None)
        .await
        .unwrap();
    let sample = p.srandmember_n("s", 3, None).await.unwrap();
    assert_eq!(sample.len(), 3);
    // All distinct.
    let mut sorted = sample.clone();
    sorted.sort();
    sorted.dedup();
    assert_eq!(sorted.len(), 3);
}

#[tokio::test]
async fn test_srandmember_n_negative_from_multiple() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let sample = p.srandmember_n("s", -10, None).await.unwrap();
    assert_eq!(sample.len(), 10);
    for m in &sample {
        assert!(m == b"a" || m == b"b");
    }
}

#[tokio::test]
async fn test_set_diff() {
    let p = new_test_pool();

    p.sadd("s1", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.sadd("s2", &[b"b", b"c", b"d"], None, None).await.unwrap();

    let mut diff = p.sdiff(&["s1", "s2"], None).await.unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec()]);

    // Diff with missing set — returns all of first set.
    let mut diff = p.sdiff(&["s1", "missing"], None).await.unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);

    let count = p
        .sdiffstore("dest", &["s1", "s2"], None, None)
        .await
        .unwrap();
    assert_eq!(count, 1);

    let mut stored = p.smembers("dest", None).await.unwrap();
    stored.sort();
    assert_eq!(stored, vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_sdiff_single_set() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let mut diff = p.sdiff(&["s"], None).await.unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec(), b"b".to_vec()]);
}

#[tokio::test]
async fn test_sdiff_identical_sets() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("s2", &[b"a", b"b"], None, None).await.unwrap();
    let diff = p.sdiff(&["s1", "s2"], None).await.unwrap();
    assert!(diff.is_empty());
}

#[tokio::test]
async fn test_sdiff_three_sets() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a", b"b", b"c", b"d"], None, None)
        .await
        .unwrap();
    p.sadd("s2", &[b"b"], None, None).await.unwrap();
    p.sadd("s3", &[b"c"], None, None).await.unwrap();
    let mut diff = p.sdiff(&["s1", "s2", "s3"], None).await.unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec(), b"d".to_vec()]);
}

#[tokio::test]
async fn test_sdiffstore_overwrites_destination() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("s2", &[b"b"], None, None).await.unwrap();
    p.sadd("dest", &[b"old1", b"old2", b"old3"], None, None)
        .await
        .unwrap();
    let count = p
        .sdiffstore("dest", &["s1", "s2"], None, None)
        .await
        .unwrap();
    assert_eq!(count, 1);
    assert_eq!(p.smembers("dest", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_set_intersect() {
    let p = new_test_pool();

    p.sadd("s1", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.sadd("s2", &[b"b", b"c", b"d"], None, None).await.unwrap();

    let mut inter = p.sinter(&["s1", "s2"], None).await.unwrap();
    inter.sort();
    assert_eq!(inter, vec![b"b".to_vec(), b"c".to_vec()]);

    // Intersection with missing set — empty.
    assert_eq!(
        p.sinter(&["s1", "missing"], None).await.unwrap(),
        Vec::<Vec<u8>>::new()
    );

    let count = p
        .sinterstore("dest", &["s1", "s2"], None, None)
        .await
        .unwrap();
    assert_eq!(count, 2);

    let mut stored = p.smembers("dest", None).await.unwrap();
    stored.sort();
    assert_eq!(stored, vec![b"b".to_vec(), b"c".to_vec()]);
}

#[tokio::test]
async fn test_sinter_single_set() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let mut inter = p.sinter(&["s"], None).await.unwrap();
    inter.sort();
    assert_eq!(inter, vec![b"a".to_vec(), b"b".to_vec()]);
}

#[tokio::test]
async fn test_sinter_disjoint() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("s2", &[b"c", b"d"], None, None).await.unwrap();
    let inter = p.sinter(&["s1", "s2"], None).await.unwrap();
    assert!(inter.is_empty());
}

#[tokio::test]
async fn test_sinter_three_sets() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.sadd("s2", &[b"b", b"c", b"d"], None, None).await.unwrap();
    p.sadd("s3", &[b"c", b"d", b"e"], None, None).await.unwrap();
    let inter = p.sinter(&["s1", "s2", "s3"], None).await.unwrap();
    assert_eq!(inter, vec![b"c".to_vec()]);
}

#[tokio::test]
async fn test_sinterstore_overwrites_destination() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("s2", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("dest", &[b"old"], None, None).await.unwrap();
    let count = p
        .sinterstore("dest", &["s1", "s2"], None, None)
        .await
        .unwrap();
    assert_eq!(count, 2);
    let mut stored = p.smembers("dest", None).await.unwrap();
    stored.sort();
    assert_eq!(stored, vec![b"a".to_vec(), b"b".to_vec()]);
}

#[tokio::test]
async fn test_set_union() {
    let p = new_test_pool();

    p.sadd("s1", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("s2", &[b"b", b"c"], None, None).await.unwrap();

    let mut un = p.sunion(&["s1", "s2"], None).await.unwrap();
    un.sort();
    assert_eq!(un, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);

    let count = p
        .sunionstore("dest", &["s1", "s2"], None, None)
        .await
        .unwrap();
    assert_eq!(count, 3);

    let mut stored = p.smembers("dest", None).await.unwrap();
    stored.sort();
    assert_eq!(stored, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
}

#[tokio::test]
async fn test_sunion_single_set() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    let mut un = p.sunion(&["s"], None).await.unwrap();
    un.sort();
    assert_eq!(un, vec![b"a".to_vec(), b"b".to_vec()]);
}

#[tokio::test]
async fn test_sunion_with_missing() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a"], None, None).await.unwrap();
    let un = p.sunion(&["s1", "missing"], None).await.unwrap();
    assert_eq!(un, vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_sunion_three_sets() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a"], None, None).await.unwrap();
    p.sadd("s2", &[b"b"], None, None).await.unwrap();
    p.sadd("s3", &[b"c"], None, None).await.unwrap();
    let mut un = p.sunion(&["s1", "s2", "s3"], None).await.unwrap();
    un.sort();
    assert_eq!(un, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
}

#[tokio::test]
async fn test_sunionstore_overwrites_destination() {
    let p = new_test_pool();
    p.sadd("s1", &[b"a"], None, None).await.unwrap();
    p.sadd("dest", &[b"old1", b"old2"], None, None)
        .await
        .unwrap();
    let count = p.sunionstore("dest", &["s1"], None, None).await.unwrap();
    assert_eq!(count, 1);
    assert_eq!(p.smembers("dest", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_set_move() {
    let p = new_test_pool();

    p.sadd("src", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("dst", &[b"c"], None, None).await.unwrap();

    let moved = p.smove("src", "dst", b"a", None, None).await.unwrap();
    assert!(moved);

    assert!(!p.sismember("src", b"a", None).await.unwrap());
    assert!(p.sismember("dst", b"a", None).await.unwrap());

    // Moving non-existent member returns false.
    let moved = p.smove("src", "dst", b"z", None, None).await.unwrap();
    assert!(!moved);
}

#[tokio::test]
async fn test_smove_missing_source() {
    let p = new_test_pool();
    assert!(!p.smove("missing", "dst", b"a", None, None).await.unwrap());
}

#[tokio::test]
async fn test_smove_creates_destination() {
    let p = new_test_pool();
    p.sadd("src", &[b"a"], None, None).await.unwrap();

    assert!(p.smove("src", "dst", b"a", None, None).await.unwrap());
    assert_eq!(p.scard("src", None).await.unwrap(), 0);
    assert_eq!(p.scard("dst", None).await.unwrap(), 1);
    assert!(p.sismember("dst", b"a", None).await.unwrap());
}

#[tokio::test]
async fn test_smove_member_already_in_destination() {
    let p = new_test_pool();
    p.sadd("src", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("dst", &[b"a", b"c"], None, None).await.unwrap();

    // "a" is in both src and dst. Move should still succeed.
    let moved = p.smove("src", "dst", b"a", None, None).await.unwrap();
    assert!(moved);
    assert!(!p.sismember("src", b"a", None).await.unwrap());
    assert!(p.sismember("dst", b"a", None).await.unwrap());
    // dst should still have 2 members (a was already there, not duplicated).
    assert_eq!(p.scard("dst", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_smove_with_ttl() {
    let p = new_test_pool();
    p.sadd("src", &[b"a"], None, None).await.unwrap();
    let moved = p
        .smove("src", "dst", b"a", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    assert!(moved);
}

#[tokio::test]
async fn test_type_mismatch_string_on_list() {
    let p = new_test_pool();
    p.rpush("k", &[b"a"], None, None).await.unwrap();
    assert!(p.get("k", None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_list_on_string() {
    let p = new_test_pool();
    p.set("k", b"val", None, None).await.unwrap();
    assert!(p.lpush("k", &[b"a"], None, None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_set_on_string() {
    let p = new_test_pool();
    p.set("k", b"val", None, None).await.unwrap();
    assert!(p.sadd("k", &[b"a"], None, None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_string_on_set() {
    let p = new_test_pool();
    p.sadd("k", &[b"a"], None, None).await.unwrap();
    assert!(p.get("k", None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_list_on_set() {
    let p = new_test_pool();
    p.sadd("k", &[b"a"], None, None).await.unwrap();
    assert!(p.lpush("k", &[b"x"], None, None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_set_on_list() {
    let p = new_test_pool();
    p.rpush("k", &[b"a"], None, None).await.unwrap();
    assert!(p.sadd("k", &[b"x"], None, None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_incr_on_list() {
    let p = new_test_pool();
    p.rpush("k", &[b"1"], None, None).await.unwrap();
    assert!(p.incr_by("k", 1, None, None).await.is_err());
}

#[tokio::test]
async fn test_type_mismatch_append_on_list() {
    let p = new_test_pool();
    p.rpush("k", &[b"a"], None, None).await.unwrap();
    assert!(p.append("k", b"x", None, None).await.is_err());
}

#[tokio::test]
async fn test_set_with_persist_ttl() {
    let p = new_test_pool();
    p.set("k", b"v1", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    p.set("k", b"v2", Some(TtlOp::Persist), None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_ttl_keep_preserves_expiry() {
    let p = new_test_pool();
    p.set("k", b"v1", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    p.set("k", b"v2", Some(TtlOp::Keep), None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_replace_with_keep_ttl() {
    let p = new_test_pool();
    p.set("k", b"v1", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    p.replace("k", b"v2", Some(TtlOp::Keep), None)
        .await
        .unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_expired_key_not_returned() {
    let p = new_test_pool();
    // Use a short TTL; miniredis FastForward advances 1s per real second.
    p.set("k", b"val", Some(TtlOp::SetMs(1)), None)
        .await
        .unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(1500)).await;
    assert!(is_miss(&p.get("k", None).await.unwrap_err()));
}

#[tokio::test]
async fn test_set_if_not_exists_on_expired_key() {
    let p = new_test_pool();
    p.set("k", b"old", Some(TtlOp::SetMs(1)), None)
        .await
        .unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(1500)).await;

    p.set_if_not_exists("k", b"new", None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"new".to_vec());
}

#[tokio::test]
async fn test_replace_on_expired_key_fails() {
    let p = new_test_pool();
    p.set("k", b"old", Some(TtlOp::SetMs(1)), None)
        .await
        .unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(1500)).await;
    assert!(is_miss(
        &p.replace("k", b"new", None, None).await.unwrap_err()
    ));
}

#[tokio::test]
async fn test_set_with_ttl_then_overwrite_without() {
    let p = new_test_pool();
    p.set("k", b"v1", Some(TtlOp::SetMs(100_000)), None)
        .await
        .unwrap();
    // Overwriting without TTL should remove TTL (Persist semantics).
    p.set("k", b"v2", Some(TtlOp::Persist), None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v2".to_vec());
}

#[tokio::test]
async fn test_set_if_not_exists_with_ttl_expires() {
    let p = new_test_pool();
    p.set_if_not_exists("k", b"v", Some(TtlOp::SetMs(1)), None)
        .await
        .unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(1500)).await;
    assert!(is_miss(&p.get("k", None).await.unwrap_err()));
}

#[tokio::test]
async fn test_independent_keys_different_types() {
    let p = new_test_pool();

    p.set("str", b"hello", None, None).await.unwrap();
    p.rpush("list", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("set", &[b"x", b"y"], None, None).await.unwrap();

    // All independent operations work.
    assert_eq!(p.get("str", None).await.unwrap(), b"hello".to_vec());
    assert_eq!(p.llen("list", None).await.unwrap(), 2);
    assert_eq!(p.scard("set", None).await.unwrap(), 2);

    // Deleting one doesn't affect others.
    p.delete(&["str"], None).await.unwrap();
    assert_eq!(p.llen("list", None).await.unwrap(), 2);
    assert_eq!(p.scard("set", None).await.unwrap(), 2);
}

#[tokio::test]
async fn test_delete_list_key() {
    let p = new_test_pool();
    p.rpush("l", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(p.delete(&["l"], None).await.unwrap(), 1);
    assert_eq!(p.llen("l", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_delete_set_key() {
    let p = new_test_pool();
    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(p.delete(&["s"], None).await.unwrap(), 1);
    assert_eq!(p.scard("s", None).await.unwrap(), 0);
}

#[tokio::test]
async fn test_delete_mixed_types() {
    let p = new_test_pool();
    p.set("str", b"v", None, None).await.unwrap();
    p.rpush("list", &[b"a"], None, None).await.unwrap();
    p.sadd("set", &[b"x"], None, None).await.unwrap();
    let deleted = p
        .delete(&["str", "list", "set", "missing"], None)
        .await
        .unwrap();
    assert_eq!(deleted, 3);
}

#[tokio::test]
async fn test_reuse_key_string_to_list() {
    let p = new_test_pool();
    p.set("k", b"v", None, None).await.unwrap();
    p.delete(&["k"], None).await.unwrap();
    p.rpush("k", &[b"a"], None, None).await.unwrap();
    assert_eq!(p.lrange_all("k", None).await.unwrap(), vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_reuse_key_list_to_set() {
    let p = new_test_pool();
    p.rpush("k", &[b"a"], None, None).await.unwrap();
    p.delete(&["k"], None).await.unwrap();
    p.sadd("k", &[b"x"], None, None).await.unwrap();
    assert!(p.sismember("k", b"x", None).await.unwrap());
}

#[tokio::test]
async fn test_reuse_key_set_to_string() {
    let p = new_test_pool();
    p.sadd("k", &[b"x"], None, None).await.unwrap();
    p.delete(&["k"], None).await.unwrap();
    p.set("k", b"v", None, None).await.unwrap();
    assert_eq!(p.get("k", None).await.unwrap(), b"v".to_vec());
}
