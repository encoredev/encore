use crate::cache::memcluster::MemoryStore;
use crate::cache::pool::{ListDirection, TtlOp};

fn new_store() -> MemoryStore {
    MemoryStore::new()
}

#[test]
fn test_get_missing_key() {
    let s = new_store();
    assert_eq!(s.get("missing").unwrap(), None);
}

#[test]
fn test_set_and_get() {
    let s = new_store();
    s.set("k", b"hello", None).unwrap();
    assert_eq!(s.get("k").unwrap(), Some(b"hello".to_vec()));
}

#[test]
fn test_set_overwrites() {
    let s = new_store();
    s.set("k", b"v1", None).unwrap();
    s.set("k", b"v2", None).unwrap();
    assert_eq!(s.get("k").unwrap(), Some(b"v2".to_vec()));
}

#[test]
fn test_set_with_persist_ttl() {
    let s = new_store();
    // Set with a TTL, then overwrite with Persist to remove it
    s.set("k", b"v1", Some(TtlOp::SetMs(100_000))).unwrap();
    s.set("k", b"v2", Some(TtlOp::Persist)).unwrap();
    assert_eq!(s.get("k").unwrap(), Some(b"v2".to_vec()));
}

#[test]
fn test_delete() {
    let s = new_store();
    s.set("a", b"1", None).unwrap();
    s.set("b", b"2", None).unwrap();
    s.set("c", b"3", None).unwrap();

    let deleted = s.delete(&["a", "c", "missing"]).unwrap();
    assert_eq!(deleted, 2);
    assert_eq!(s.get("a").unwrap(), None);
    assert_eq!(s.get("b").unwrap(), Some(b"2".to_vec()));
    assert_eq!(s.get("c").unwrap(), None);
}

#[test]
fn test_set_if_not_exists() {
    let s = new_store();

    // First call succeeds
    assert!(s.set_if_not_exists("k", b"v1", None).unwrap());
    assert_eq!(s.get("k").unwrap(), Some(b"v1".to_vec()));

    // Second call fails — key exists
    assert!(!s.set_if_not_exists("k", b"v2", None).unwrap());
    assert_eq!(s.get("k").unwrap(), Some(b"v1".to_vec()));
}

#[test]
fn test_replace_missing_key() {
    let s = new_store();
    assert!(!s.replace("k", b"v", None).unwrap());
}

#[test]
fn test_replace_existing_key() {
    let s = new_store();
    s.set("k", b"old", None).unwrap();
    assert!(s.replace("k", b"new", None).unwrap());
    assert_eq!(s.get("k").unwrap(), Some(b"new".to_vec()));
}

#[test]
fn test_get_and_set() {
    let s = new_store();

    // First call: key doesn't exist
    let old = s.get_and_set("k", b"v1", None).unwrap();
    assert_eq!(old, None);
    assert_eq!(s.get("k").unwrap(), Some(b"v1".to_vec()));

    // Second call: returns old value
    let old = s.get_and_set("k", b"v2", None).unwrap();
    assert_eq!(old, Some(b"v1".to_vec()));
    assert_eq!(s.get("k").unwrap(), Some(b"v2".to_vec()));
}

#[test]
fn test_get_and_delete() {
    let s = new_store();
    s.set("k", b"val", None).unwrap();

    let old = s.get_and_delete("k").unwrap();
    assert_eq!(old, Some(b"val".to_vec()));

    // Key should be gone
    assert_eq!(s.get("k").unwrap(), None);
    assert_eq!(s.get_and_delete("k").unwrap(), None);
}

#[test]
fn test_mget() {
    let s = new_store();
    s.set("a", b"1", None).unwrap();
    s.set("c", b"3", None).unwrap();

    let results = s.mget(&["a", "b", "c"]).unwrap();
    assert_eq!(results.len(), 3);
    assert_eq!(results[0], Some(b"1".to_vec()));
    assert_eq!(results[1], None);
    assert_eq!(results[2], Some(b"3".to_vec()));
}

#[test]
fn test_append() {
    let s = new_store();

    // Append to non-existent key creates it
    let len = s.append("k", b"hello", None).unwrap();
    assert_eq!(len, 5);
    assert_eq!(s.get("k").unwrap(), Some(b"hello".to_vec()));

    // Append to existing key
    let len = s.append("k", b" world", None).unwrap();
    assert_eq!(len, 11);
    assert_eq!(s.get("k").unwrap(), Some(b"hello world".to_vec()));
}

#[test]
fn test_get_range() {
    let s = new_store();
    s.set("k", b"hello world", None).unwrap();

    assert_eq!(s.get_range("k", 0, 4).unwrap(), b"hello");
    assert_eq!(s.get_range("k", 6, 10).unwrap(), b"world");
    // Negative indices
    assert_eq!(s.get_range("k", -5, -1).unwrap(), b"world");
    // Missing key returns empty
    assert_eq!(s.get_range("missing", 0, 10).unwrap(), b"".to_vec());
}

#[test]
fn test_set_range() {
    let s = new_store();
    s.set("k", b"hello world", None).unwrap();

    let len = s.set_range("k", 6, b"redis", None).unwrap();
    assert_eq!(len, 11);
    assert_eq!(s.get("k").unwrap(), Some(b"hello redis".to_vec()));

    // Set range past end extends the string
    let len = s.set_range("k", 11, b"!!", None).unwrap();
    assert_eq!(len, 13);
    assert_eq!(s.get("k").unwrap(), Some(b"hello redis!!".to_vec()));
}

#[test]
fn test_set_range_with_gap() {
    let s = new_store();
    // Setting at offset past current length pads with zeros
    let len = s.set_range("k", 5, b"hi", None).unwrap();
    assert_eq!(len, 7);
    let val = s.get("k").unwrap().unwrap();
    assert_eq!(&val[..5], &[0, 0, 0, 0, 0]);
    assert_eq!(&val[5..], b"hi");
}

#[test]
fn test_strlen() {
    let s = new_store();
    assert_eq!(s.strlen("missing").unwrap(), 0);

    s.set("k", b"hello", None).unwrap();
    assert_eq!(s.strlen("k").unwrap(), 5);
}

#[test]
fn test_incr_by() {
    let s = new_store();

    // Increment on non-existent key initializes to delta
    let val = s.incr_by("k", 5, None).unwrap();
    assert_eq!(val, 5);

    let val = s.incr_by("k", 3, None).unwrap();
    assert_eq!(val, 8);

    // Negative delta acts as decrement
    let val = s.incr_by("k", -2, None).unwrap();
    assert_eq!(val, 6);
}

#[test]
fn test_incr_by_invalid_value() {
    let s = new_store();
    s.set("k", b"not-a-number", None).unwrap();
    assert!(s.incr_by("k", 1, None).is_err());
}

#[test]
fn test_incr_by_float() {
    let s = new_store();

    let val = s.incr_by_float("k", 1.5, None).unwrap();
    assert!((val - 1.5).abs() < f64::EPSILON);

    let val = s.incr_by_float("k", 2.5, None).unwrap();
    assert!((val - 4.0).abs() < f64::EPSILON);

    // Negative delta
    let val = s.incr_by_float("k", -1.0, None).unwrap();
    assert!((val - 3.0).abs() < f64::EPSILON);
}

#[test]
fn test_lpush_rpush() {
    let s = new_store();

    // lpush reverses: lpush [b, a] → push a first, then b → [b, a]
    let len = s.lpush("k", &[b"a", b"b"], None).unwrap();
    assert_eq!(len, 2);

    let len = s.rpush("k", &[b"c", b"d"], None).unwrap();
    assert_eq!(len, 4);

    let items = s.lrange("k", 0, -1).unwrap();
    assert_eq!(items, vec![b"a", b"b", b"c", b"d"]);
}

#[test]
fn test_lpop_rpop() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"c", b"d"], None).unwrap();

    let popped = s.lpop("k", Some(2), None).unwrap();
    assert_eq!(popped, vec![b"a".to_vec(), b"b".to_vec()]);

    let popped = s.rpop("k", Some(1), None).unwrap();
    assert_eq!(popped, vec![b"d".to_vec()]);

    // Remaining: just "c"
    let items = s.lrange("k", 0, -1).unwrap();
    assert_eq!(items, vec![b"c"]);
}

#[test]
fn test_lpop_rpop_empty() {
    let s = new_store();
    assert_eq!(
        s.lpop("missing", Some(1), None).unwrap(),
        Vec::<Vec<u8>>::new()
    );
    assert_eq!(
        s.rpop("missing", Some(1), None).unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[test]
fn test_lindex() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"c"], None).unwrap();

    assert_eq!(s.lindex("k", 0).unwrap(), Some(b"a".to_vec()));
    assert_eq!(s.lindex("k", 2).unwrap(), Some(b"c".to_vec()));
    assert_eq!(s.lindex("k", -1).unwrap(), Some(b"c".to_vec()));
    assert_eq!(s.lindex("k", -3).unwrap(), Some(b"a".to_vec()));
    // Out of range
    assert_eq!(s.lindex("k", 10).unwrap(), None);
    assert_eq!(s.lindex("k", -10).unwrap(), None);
    // Missing key
    assert_eq!(s.lindex("missing", 0).unwrap(), None);
}

#[test]
fn test_lset() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"c"], None).unwrap();

    s.lset("k", 1, b"B", None).unwrap();
    assert_eq!(s.lindex("k", 1).unwrap(), Some(b"B".to_vec()));

    // Negative index
    s.lset("k", -1, b"C", None).unwrap();
    assert_eq!(s.lindex("k", 2).unwrap(), Some(b"C".to_vec()));
}

#[test]
fn test_lset_out_of_range() {
    let s = new_store();
    s.rpush("k", &[b"a"], None).unwrap();
    assert!(s.lset("k", 5, b"x", None).is_err());
}

#[test]
fn test_lset_missing_key() {
    let s = new_store();
    assert!(s.lset("missing", 0, b"x", None).is_err());
}

#[test]
fn test_lrange() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"c", b"d", b"e"], None).unwrap();

    assert_eq!(s.lrange("k", 0, 2).unwrap(), vec![b"a", b"b", b"c"]);
    assert_eq!(s.lrange("k", -2, -1).unwrap(), vec![b"d", b"e"]);
    // Empty range
    assert_eq!(s.lrange("k", 5, 10).unwrap(), Vec::<Vec<u8>>::new());
    // Missing key
    assert_eq!(s.lrange("missing", 0, -1).unwrap(), Vec::<Vec<u8>>::new());
}

#[test]
fn test_ltrim() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"c", b"d", b"e"], None).unwrap();

    s.ltrim("k", 1, 3, None).unwrap();
    assert_eq!(s.lrange("k", 0, -1).unwrap(), vec![b"b", b"c", b"d"]);
}

#[test]
fn test_ltrim_clears_when_out_of_range() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b"], None).unwrap();
    s.ltrim("k", 5, 10, None).unwrap();
    assert_eq!(s.llen("k").unwrap(), 0);
}

#[test]
fn test_linsert_before() {
    let s = new_store();
    s.rpush("k", &[b"a", b"c"], None).unwrap();

    let len = s.linsert_before("k", b"c", b"b", None).unwrap();
    assert_eq!(len, 3);
    assert_eq!(s.lrange("k", 0, -1).unwrap(), vec![b"a", b"b", b"c"]);

    // Pivot not found
    assert_eq!(s.linsert_before("k", b"z", b"x", None).unwrap(), -1);
}

#[test]
fn test_linsert_after() {
    let s = new_store();
    s.rpush("k", &[b"a", b"c"], None).unwrap();

    let len = s.linsert_after("k", b"a", b"b", None).unwrap();
    assert_eq!(len, 3);
    assert_eq!(s.lrange("k", 0, -1).unwrap(), vec![b"a", b"b", b"c"]);

    // Pivot not found
    assert_eq!(s.linsert_after("k", b"z", b"x", None).unwrap(), -1);
}

#[test]
fn test_linsert_missing_key() {
    let s = new_store();
    assert_eq!(s.linsert_before("missing", b"a", b"b", None).unwrap(), -1);
    assert_eq!(s.linsert_after("missing", b"a", b"b", None).unwrap(), -1);
}

#[test]
fn test_lrem_positive_count() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"a", b"c", b"a"], None).unwrap();

    // Remove first 2 occurrences of "a"
    let removed = s.lrem("k", 2, b"a", None).unwrap();
    assert_eq!(removed, 2);
    assert_eq!(s.lrange("k", 0, -1).unwrap(), vec![b"b", b"c", b"a"]);
}

#[test]
fn test_lrem_negative_count() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"a", b"c", b"a"], None).unwrap();

    // Remove last 2 occurrences of "a"
    let removed = s.lrem("k", -2, b"a", None).unwrap();
    assert_eq!(removed, 2);
    assert_eq!(s.lrange("k", 0, -1).unwrap(), vec![b"a", b"b", b"c"]);
}

#[test]
fn test_lrem_zero_removes_all() {
    let s = new_store();
    s.rpush("k", &[b"a", b"b", b"a", b"c", b"a"], None).unwrap();

    let removed = s.lrem("k", 0, b"a", None).unwrap();
    assert_eq!(removed, 3);
    assert_eq!(s.lrange("k", 0, -1).unwrap(), vec![b"b", b"c"]);
}

#[test]
fn test_lrem_missing_key() {
    let s = new_store();
    assert_eq!(s.lrem("missing", 0, b"a", None).unwrap(), 0);
}

#[test]
fn test_lmove() {
    let s = new_store();
    s.rpush("src", &[b"a", b"b", b"c"], None).unwrap();

    // Move left of src to right of dst
    let val = s
        .lmove(
            "src",
            "dst",
            ListDirection::Left,
            ListDirection::Right,
            None,
        )
        .unwrap();
    assert_eq!(val, Some(b"a".to_vec()));

    assert_eq!(s.lrange("src", 0, -1).unwrap(), vec![b"b", b"c"]);
    assert_eq!(s.lrange("dst", 0, -1).unwrap(), vec![b"a"]);

    // Move right of src to left of dst
    let val = s
        .lmove(
            "src",
            "dst",
            ListDirection::Right,
            ListDirection::Left,
            None,
        )
        .unwrap();
    assert_eq!(val, Some(b"c".to_vec()));

    assert_eq!(s.lrange("src", 0, -1).unwrap(), vec![b"b"]);
    assert_eq!(s.lrange("dst", 0, -1).unwrap(), vec![b"c", b"a"]);
}

#[test]
fn test_lmove_empty_source() {
    let s = new_store();
    assert_eq!(
        s.lmove(
            "missing",
            "dst",
            ListDirection::Left,
            ListDirection::Right,
            None
        )
        .unwrap(),
        None
    );
}

#[test]
fn test_llen() {
    let s = new_store();
    assert_eq!(s.llen("missing").unwrap(), 0);

    s.rpush("k", &[b"a", b"b", b"c"], None).unwrap();
    assert_eq!(s.llen("k").unwrap(), 3);
}

#[test]
fn test_sadd_srem() {
    let s = new_store();

    let added = s.sadd("k", &[b"a", b"b", b"c"], None).unwrap();
    assert_eq!(added, 3);

    // Adding duplicates
    let added = s.sadd("k", &[b"b", b"c", b"d"], None).unwrap();
    assert_eq!(added, 1); // only "d" is new

    let removed = s.srem("k", &[b"a", b"missing"], None).unwrap();
    assert_eq!(removed, 1);
}

#[test]
fn test_sismember() {
    let s = new_store();
    s.sadd("k", &[b"a", b"b"], None).unwrap();

    assert!(s.sismember("k", b"a").unwrap());
    assert!(!s.sismember("k", b"c").unwrap());
    assert!(!s.sismember("missing", b"a").unwrap());
}

#[test]
fn test_smembers_scard() {
    let s = new_store();
    s.sadd("k", &[b"a", b"b", b"c"], None).unwrap();

    let mut members = s.smembers("k").unwrap();
    members.sort();
    assert_eq!(members, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);

    assert_eq!(s.scard("k").unwrap(), 3);
    assert_eq!(s.scard("missing").unwrap(), 0);
}

#[test]
fn test_smembers_empty() {
    let s = new_store();
    assert_eq!(s.smembers("missing").unwrap(), Vec::<Vec<u8>>::new());
}

#[test]
fn test_spop() {
    let s = new_store();
    s.sadd("k", &[b"a", b"b", b"c"], None).unwrap();

    let popped = s.spop("k", Some(2), None).unwrap();
    assert_eq!(popped.len(), 2);
    assert_eq!(s.scard("k").unwrap(), 1);

    // Pop from empty
    assert_eq!(
        s.spop("missing", Some(1), None).unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[test]
fn test_spop_one() {
    let s = new_store();
    s.sadd("k", &[b"only"], None).unwrap();

    let popped = s.spop("k", None, None).unwrap();
    assert_eq!(popped, vec![b"only".to_vec()]);
    assert_eq!(s.scard("k").unwrap(), 0);
}

#[test]
fn test_srandmember() {
    let s = new_store();
    s.sadd("k", &[b"a", b"b", b"c"], None).unwrap();

    // Positive count — distinct members
    let sample = s.srandmember("k", 2).unwrap();
    assert_eq!(sample.len(), 2);
    // All members should be from the set
    for m in &sample {
        assert!(s.sismember("k", m).unwrap());
    }

    // Positive count > set size returns at most set size
    let sample = s.srandmember("k", 10).unwrap();
    assert_eq!(sample.len(), 3);

    // Empty set
    assert_eq!(s.srandmember("missing", 2).unwrap(), Vec::<Vec<u8>>::new());
}

#[test]
fn test_srandmember_negative_count() {
    let s = new_store();
    s.sadd("k", &[b"a"], None).unwrap();

    // Negative count — allows duplicates, returns exactly abs(count)
    let sample = s.srandmember("k", -5).unwrap();
    assert_eq!(sample.len(), 5);
    for m in &sample {
        assert_eq!(m, &b"a".to_vec());
    }
}

#[test]
fn test_sdiff() {
    let s = new_store();
    s.sadd("s1", &[b"a", b"b", b"c"], None).unwrap();
    s.sadd("s2", &[b"b", b"c", b"d"], None).unwrap();

    let mut diff = s.sdiff(&["s1", "s2"]).unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec()]);

    // Diff with missing set — returns all of first set
    let mut diff = s.sdiff(&["s1", "missing"]).unwrap();
    diff.sort();
    assert_eq!(diff, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
}

#[test]
fn test_sdiffstore() {
    let s = new_store();
    s.sadd("s1", &[b"a", b"b", b"c"], None).unwrap();
    s.sadd("s2", &[b"b", b"c", b"d"], None).unwrap();

    let count = s.sdiffstore("dest", &["s1", "s2"], None).unwrap();
    assert_eq!(count, 1);

    let mut members = s.smembers("dest").unwrap();
    members.sort();
    assert_eq!(members, vec![b"a".to_vec()]);
}

#[test]
fn test_sinter() {
    let s = new_store();
    s.sadd("s1", &[b"a", b"b", b"c"], None).unwrap();
    s.sadd("s2", &[b"b", b"c", b"d"], None).unwrap();

    let mut inter = s.sinter(&["s1", "s2"]).unwrap();
    inter.sort();
    assert_eq!(inter, vec![b"b".to_vec(), b"c".to_vec()]);

    // Intersection with missing set — empty
    assert_eq!(s.sinter(&["s1", "missing"]).unwrap(), Vec::<Vec<u8>>::new());
}

#[test]
fn test_sinterstore() {
    let s = new_store();
    s.sadd("s1", &[b"a", b"b", b"c"], None).unwrap();
    s.sadd("s2", &[b"b", b"c", b"d"], None).unwrap();

    let count = s.sinterstore("dest", &["s1", "s2"], None).unwrap();
    assert_eq!(count, 2);

    let mut members = s.smembers("dest").unwrap();
    members.sort();
    assert_eq!(members, vec![b"b".to_vec(), b"c".to_vec()]);
}

#[test]
fn test_sunion() {
    let s = new_store();
    s.sadd("s1", &[b"a", b"b"], None).unwrap();
    s.sadd("s2", &[b"b", b"c"], None).unwrap();

    let mut union = s.sunion(&["s1", "s2"]).unwrap();
    union.sort();
    assert_eq!(union, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
}

#[test]
fn test_sunionstore() {
    let s = new_store();
    s.sadd("s1", &[b"a", b"b"], None).unwrap();
    s.sadd("s2", &[b"b", b"c"], None).unwrap();

    let count = s.sunionstore("dest", &["s1", "s2"], None).unwrap();
    assert_eq!(count, 3);

    let mut members = s.smembers("dest").unwrap();
    members.sort();
    assert_eq!(members, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
}

#[test]
fn test_smove() {
    let s = new_store();
    s.sadd("src", &[b"a", b"b"], None).unwrap();
    s.sadd("dst", &[b"c"], None).unwrap();

    // Move existing member
    assert!(s.smove("src", "dst", b"a", None).unwrap());

    let mut src_members = s.smembers("src").unwrap();
    src_members.sort();
    assert_eq!(src_members, vec![b"b".to_vec()]);

    let mut dst_members = s.smembers("dst").unwrap();
    dst_members.sort();
    assert_eq!(dst_members, vec![b"a".to_vec(), b"c".to_vec()]);

    // Move non-existent member
    assert!(!s.smove("src", "dst", b"missing", None).unwrap());
}

#[test]
fn test_smove_missing_source() {
    let s = new_store();
    assert!(!s.smove("missing", "dst", b"a", None).unwrap());
}

#[test]
fn test_smove_creates_destination() {
    let s = new_store();
    s.sadd("src", &[b"a"], None).unwrap();

    assert!(s.smove("src", "dst", b"a", None).unwrap());
    assert_eq!(s.scard("src").unwrap(), 0);
    assert_eq!(s.scard("dst").unwrap(), 1);
    assert!(s.sismember("dst", b"a").unwrap());
}

#[test]
fn test_type_mismatch_string_on_list() {
    let s = new_store();
    s.rpush("k", &[b"a"], None).unwrap();
    assert!(s.get("k").is_err());
}

#[test]
fn test_type_mismatch_list_on_string() {
    let s = new_store();
    s.set("k", b"val", None).unwrap();
    assert!(s.lpush("k", &[b"a"], None).is_err());
}

#[test]
fn test_type_mismatch_set_on_string() {
    let s = new_store();
    s.set("k", b"val", None).unwrap();
    assert!(s.sadd("k", &[b"a"], None).is_err());
}

#[test]
fn test_type_mismatch_string_on_set() {
    let s = new_store();
    s.sadd("k", &[b"a"], None).unwrap();
    assert!(s.get("k").is_err());
}

#[test]
fn test_ttl_keep_preserves_expiry() {
    let s = new_store();
    // Set with a long TTL
    s.set("k", b"v1", Some(TtlOp::SetMs(100_000))).unwrap();

    // Replace with KeepTTL — value changes but TTL stays
    s.set("k", b"v2", Some(TtlOp::Keep)).unwrap();
    assert_eq!(s.get("k").unwrap(), Some(b"v2".to_vec()));
}

#[test]
fn test_replace_with_keep_ttl() {
    let s = new_store();
    s.set("k", b"v1", Some(TtlOp::SetMs(100_000))).unwrap();

    assert!(s.replace("k", b"v2", Some(TtlOp::Keep)).unwrap());
    assert_eq!(s.get("k").unwrap(), Some(b"v2".to_vec()));
}

#[test]
fn test_expired_key_not_returned() {
    let s = new_store();
    // Set with 0ms TTL — immediately expired
    s.set("k", b"val", Some(TtlOp::SetMs(0))).unwrap();

    // Allow a tiny bit of time to pass
    std::thread::sleep(std::time::Duration::from_millis(1));

    assert_eq!(s.get("k").unwrap(), None);
}

#[test]
fn test_set_if_not_exists_on_expired_key() {
    let s = new_store();
    s.set("k", b"old", Some(TtlOp::SetMs(0))).unwrap();
    std::thread::sleep(std::time::Duration::from_millis(1));

    // Expired key should allow set_if_not_exists
    assert!(s.set_if_not_exists("k", b"new", None).unwrap());
    assert_eq!(s.get("k").unwrap(), Some(b"new".to_vec()));
}

#[test]
fn test_replace_on_expired_key_fails() {
    let s = new_store();
    s.set("k", b"old", Some(TtlOp::SetMs(0))).unwrap();
    std::thread::sleep(std::time::Duration::from_millis(1));

    assert!(!s.replace("k", b"new", None).unwrap());
}
