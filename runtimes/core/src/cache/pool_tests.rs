use std::sync::Arc;

use crate::cache::memcluster::MemoryStore;
use crate::cache::pool::{ListDirection, Pool};
use crate::trace::Tracer;

fn new_test_pool() -> Pool {
    let store = Arc::new(MemoryStore::new());
    Pool::in_memory(store, Tracer::noop())
}

#[tokio::test]
async fn test_set_get_delete() {
    let p = new_test_pool();

    // Set a value.
    p.set("k", b"hello", None, None).await.unwrap();

    // Get it back.
    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"hello".to_vec()));

    // Delete it.
    let deleted = p.delete(&["k"], None).await.unwrap();
    assert_eq!(deleted, 1);

    // Should be gone.
    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, None);
}

#[tokio::test]
async fn test_set_if_not_exists() {
    let p = new_test_pool();

    // First call succeeds.
    let ok = p.set_if_not_exists("k", b"v1", None, None).await.unwrap();
    assert!(ok);

    // Second call returns false (key already exists).
    let ok = p.set_if_not_exists("k", b"v2", None, None).await.unwrap();
    assert!(!ok);

    // Original value retained.
    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"v1".to_vec()));
}

#[tokio::test]
async fn test_replace() {
    let p = new_test_pool();

    // Replace on missing key returns false.
    let ok = p.replace("k", b"v1", None, None).await.unwrap();
    assert!(!ok);

    // Set a value, then replace succeeds.
    p.set("k", b"v1", None, None).await.unwrap();
    let ok = p.replace("k", b"v2", None, None).await.unwrap();
    assert!(ok);

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"v2".to_vec()));
}

#[tokio::test]
async fn test_get_and_set() {
    let p = new_test_pool();

    // get_and_set on missing key returns None.
    let old = p.get_and_set("k", b"v1", None, None).await.unwrap();
    assert_eq!(old, None);

    // Now returns old value.
    let old = p.get_and_set("k", b"v2", None, None).await.unwrap();
    assert_eq!(old, Some(b"v1".to_vec()));

    // New value stored.
    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"v2".to_vec()));
}

#[tokio::test]
async fn test_get_and_delete() {
    let p = new_test_pool();

    p.set("k", b"val", None, None).await.unwrap();

    let old = p.get_and_delete("k", None).await.unwrap();
    assert_eq!(old, Some(b"val".to_vec()));

    // Key is gone.
    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, None);
}

#[tokio::test]
async fn test_append() {
    let p = new_test_pool();

    let len = p.append("k", b"hello", None, None).await.unwrap();
    assert_eq!(len, 5);

    let len = p.append("k", b" world", None, None).await.unwrap();
    assert_eq!(len, 11);

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"hello world".to_vec()));
}

#[tokio::test]
async fn test_get_range() {
    let p = new_test_pool();

    p.set("k", b"hello world", None, None).await.unwrap();

    let sub = p.get_range("k", 0, 4, None).await.unwrap();
    assert_eq!(sub, b"hello".to_vec());

    let sub = p.get_range("k", 6, 10, None).await.unwrap();
    assert_eq!(sub, b"world".to_vec());
}

#[tokio::test]
async fn test_set_range() {
    let p = new_test_pool();

    p.set("k", b"hello world", None, None).await.unwrap();

    let new_len = p.set_range("k", 6, b"rust!", None, None).await.unwrap();
    assert_eq!(new_len, 11);

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"hello rust!".to_vec()));
}

#[tokio::test]
async fn test_strlen() {
    let p = new_test_pool();

    // Missing key has length 0.
    let len = p.strlen("k", None).await.unwrap();
    assert_eq!(len, 0);

    p.set("k", b"hello", None, None).await.unwrap();
    let len = p.strlen("k", None).await.unwrap();
    assert_eq!(len, 5);
}

#[tokio::test]
async fn test_incr_by() {
    let p = new_test_pool();

    p.set("k", b"10", None, None).await.unwrap();

    let v = p.incr_by("k", 5, None, None).await.unwrap();
    assert_eq!(v, 15);

    let v = p.decr_by("k", 3, None, None).await.unwrap();
    assert_eq!(v, 12);
}

#[tokio::test]
async fn test_incr_creates_key() {
    let p = new_test_pool();

    // Incrementing a missing key initializes to delta.
    let v = p.incr_by("k", 7, None, None).await.unwrap();
    assert_eq!(v, 7);

    let v = p.get("k", None).await.unwrap();
    assert_eq!(v, Some(b"7".to_vec()));
}

#[tokio::test]
async fn test_incr_by_float() {
    let p = new_test_pool();

    let v = p.incr_by_float("k", 1.5, None, None).await.unwrap();
    assert!((v - 1.5).abs() < f64::EPSILON);

    let v = p.decr_by_float("k", 0.5, None, None).await.unwrap();
    assert!((v - 1.0).abs() < f64::EPSILON);
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
async fn test_list_push_pop() {
    let p = new_test_pool();

    // lpush pushes each element to the head in reverse-arg order,
    // so lpush([a, b]) => list [a, b] with a at head.
    let len = p.lpush("l", &[b"a", b"b"], None, None).await.unwrap();
    assert_eq!(len, 2);

    // rpush appends to tail => [a, b, c].
    let len = p.rpush("l", &[b"c"], None, None).await.unwrap();
    assert_eq!(len, 3);

    // lpop from head returns "a".
    let vals = p.lpop("l", None, None, None).await.unwrap();
    assert_eq!(vals, vec![b"a".to_vec()]);

    // rpop from tail returns "c".
    let vals = p.rpop("l", None, None, None).await.unwrap();
    assert_eq!(vals, vec![b"c".to_vec()]);
}

#[tokio::test]
async fn test_list_set_trim() {
    let p = new_test_pool();

    p.rpush("l", &[b"a", b"b", b"c", b"d"], None, None)
        .await
        .unwrap();

    // lset at index 1.
    p.lset("l", 1, b"B", None, None).await.unwrap();

    let v = p.lindex("l", 1, None).await.unwrap();
    assert_eq!(v, Some(b"B".to_vec()));

    // ltrim to keep only indices 1..2.
    p.ltrim("l", 1, 2, None, None).await.unwrap();

    let items = p.litems("l", None).await.unwrap();
    assert_eq!(items, vec![b"B".to_vec(), b"c".to_vec()]);
}

#[tokio::test]
async fn test_list_insert() {
    let p = new_test_pool();

    p.rpush("l", &[b"a", b"c"], None, None).await.unwrap();

    // Insert before "c".
    let len = p.linsert_before("l", b"c", b"b", None, None).await.unwrap();
    assert_eq!(len, 3);

    // Insert after "c".
    let len = p.linsert_after("l", b"c", b"d", None, None).await.unwrap();
    assert_eq!(len, 4);

    let items = p.litems("l", None).await.unwrap();
    assert_eq!(
        items,
        vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec(), b"d".to_vec(),]
    );
}

#[tokio::test]
async fn test_list_remove() {
    let p = new_test_pool();

    p.rpush("l", &[b"a", b"b", b"a", b"b", b"a"], None, None)
        .await
        .unwrap();

    // Remove first 1 occurrence of "a".
    let removed = p.lrem("l", 1, b"a", None, None).await.unwrap();
    assert_eq!(removed, 1);

    // Remove last 1 occurrence of "a" (count < 0).
    let removed = p.lrem("l", -1, b"a", None, None).await.unwrap();
    assert_eq!(removed, 1);

    // Remove all remaining "b"s (count = 0).
    let removed = p.lrem("l", 0, b"b", None, None).await.unwrap();
    assert_eq!(removed, 2);

    // Only one "a" should remain.
    let items = p.litems("l", None).await.unwrap();
    assert_eq!(items, vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_list_move() {
    let p = new_test_pool();

    p.rpush("src", &[b"a", b"b", b"c"], None, None)
        .await
        .unwrap();

    // Move left-of-src to right-of-dst.
    let moved = p
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
    assert_eq!(moved, Some(b"a".to_vec()));

    let src_items = p.litems("src", None).await.unwrap();
    assert_eq!(src_items, vec![b"b".to_vec(), b"c".to_vec()]);

    let dst_items = p.litems("dst", None).await.unwrap();
    assert_eq!(dst_items, vec![b"a".to_vec()]);
}

#[tokio::test]
async fn test_list_range_items_len() {
    let p = new_test_pool();

    p.rpush("l", &[b"a", b"b", b"c", b"d"], None, None)
        .await
        .unwrap();

    // lrange subset.
    let sub = p.lrange("l", 1, 2, None).await.unwrap();
    assert_eq!(sub, vec![b"b".to_vec(), b"c".to_vec()]);

    // litems = full list.
    let all = p.litems("l", None).await.unwrap();
    assert_eq!(all.len(), 4);

    // llen.
    let len = p.llen("l", None).await.unwrap();
    assert_eq!(len, 4);
}

#[tokio::test]
async fn test_set_add_remove() {
    let p = new_test_pool();

    let added = p.sadd("s", &[b"a", b"b", b"c"], None, None).await.unwrap();
    assert_eq!(added, 3);

    // Adding existing member returns 0 new.
    let added = p.sadd("s", &[b"a"], None, None).await.unwrap();
    assert_eq!(added, 0);

    let removed = p.srem("s", &[b"b"], None, None).await.unwrap();
    assert_eq!(removed, 1);

    let len = p.scard("s", None).await.unwrap();
    assert_eq!(len, 2);
}

#[tokio::test]
async fn test_set_contains() {
    let p = new_test_pool();

    p.sadd("s", &[b"x"], None, None).await.unwrap();

    assert!(p.sismember("s", b"x", None).await.unwrap());
    assert!(!p.sismember("s", b"y", None).await.unwrap());
}

#[tokio::test]
async fn test_set_members_len() {
    let p = new_test_pool();

    p.sadd("s", &[b"a", b"b"], None, None).await.unwrap();

    let mut members = p.smembers("s", None).await.unwrap();
    members.sort();
    assert_eq!(members, vec![b"a".to_vec(), b"b".to_vec()]);

    let len = p.scard("s", None).await.unwrap();
    assert_eq!(len, 2);
}

#[tokio::test]
async fn test_set_pop_sample() {
    let p = new_test_pool();

    p.sadd("s", &[b"only"], None, None).await.unwrap();

    // spop_one removes and returns the member.
    let popped = p.spop_one("s", None, None).await.unwrap();
    assert_eq!(popped, Some(b"only".to_vec()));

    // Empty set returns None.
    let popped = p.spop_one("s", None, None).await.unwrap();
    assert_eq!(popped, None);

    // srandmember_one on empty set returns None.
    let sampled = p.srandmember_one("s", None).await.unwrap();
    assert_eq!(sampled, None);

    // srandmember_one on non-empty set returns a member.
    p.sadd("s", &[b"m"], None, None).await.unwrap();
    let sampled = p.srandmember_one("s", None).await.unwrap();
    assert_eq!(sampled, Some(b"m".to_vec()));
}

#[tokio::test]
async fn test_set_diff() {
    let p = new_test_pool();

    p.sadd("s1", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.sadd("s2", &[b"b", b"c", b"d"], None, None).await.unwrap();

    let mut diff = p.sdiff(&["s1", "s2"], None).await.unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec()]);

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
async fn test_set_intersect() {
    let p = new_test_pool();

    p.sadd("s1", &[b"a", b"b", b"c"], None, None).await.unwrap();
    p.sadd("s2", &[b"b", b"c", b"d"], None, None).await.unwrap();

    let mut inter = p.sinter(&["s1", "s2"], None).await.unwrap();
    inter.sort();
    assert_eq!(inter, vec![b"b".to_vec(), b"c".to_vec()]);

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
async fn test_set_move() {
    let p = new_test_pool();

    p.sadd("src", &[b"a", b"b"], None, None).await.unwrap();
    p.sadd("dst", &[b"c"], None, None).await.unwrap();

    let moved = p.smove("src", "dst", b"a", None, None).await.unwrap();
    assert!(moved);

    // "a" removed from src.
    assert!(!p.sismember("src", b"a", None).await.unwrap());

    // "a" present in dst.
    assert!(p.sismember("dst", b"a", None).await.unwrap());

    // Moving non-existent member returns false.
    let moved = p.smove("src", "dst", b"z", None, None).await.unwrap();
    assert!(!moved);
}
