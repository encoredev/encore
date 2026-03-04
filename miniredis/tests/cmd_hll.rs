mod helpers;
use helpers::*;

// ── PFADD ────────────────────────────────────────────────────────────

#[tokio::test]
async fn test_pfadd_basic() {
    let (_m, mut c) = start().await;

    // Add 3 new elements → 1 (changed)
    must_int!(c, "PFADD", "h", "aap", "noot", "mies"; 1);

    // Add duplicate → 0 (not changed)
    must_int!(c, "PFADD", "h", "aap"; 0);

    // TYPE should be "hll"
    must_str!(c, "TYPE", "h"; "hll");
}

#[tokio::test]
async fn test_pfadd_errors() {
    let (_m, mut c) = start().await;

    // Wrong type
    must_ok!(c, "SET", "str", "value");
    must_fail!(c, "PFADD", "str", "hi"; "not a valid HyperLogLog string value");

    // Wrong number of arguments (no args at all)
    must_fail!(c, "PFADD"; "wrong number of arguments");
}

// ── PFCOUNT ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_pfcount_basic() {
    let (_m, mut c) = start().await;

    // Add elements one at a time
    for i in 0..100 {
        must_int!(c, "PFADD", "h1", &format!("unique-{}", i); 1);
    }

    // Add one more
    must_int!(c, "PFADD", "h1", "specific-value"; 1);

    // Duplicate additions should return 0
    for _ in 0..10 {
        must_int!(c, "PFADD", "h1", "specific-value"; 0);
    }

    // Count should be 101
    must_int!(c, "PFCOUNT", "h1"; 101);
}

#[tokio::test]
async fn test_pfcount_multiple_keys() {
    let (_m, mut c) = start().await;

    // Create two non-overlapping HLLs
    must_int!(c, "PFADD", "h1", "a", "b", "c"; 1);
    must_int!(c, "PFADD", "h2", "d", "e"; 1);

    // Single key counts
    must_int!(c, "PFCOUNT", "h1"; 3);
    must_int!(c, "PFCOUNT", "h2"; 2);

    // Multi-key count (union)
    must_int!(c, "PFCOUNT", "h1", "h2"; 5);

    // With a non-existent key
    must_int!(c, "PFCOUNT", "h1", "h2", "h3"; 5);

    // Non-existent key alone
    must_int!(c, "PFCOUNT", "h9"; 0);
}

#[tokio::test]
async fn test_pfcount_errors() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SET", "str", "value");

    // Wrong number of arguments
    must_fail!(c, "PFCOUNT"; "wrong number of arguments");

    // Wrong type
    must_fail!(c, "PFCOUNT", "str"; "not a valid HyperLogLog string value");

    // Wrong type mixed with valid key
    must_int!(c, "PFADD", "h1", "a"; 1);
    must_fail!(c, "PFCOUNT", "h1", "str"; "not a valid HyperLogLog string value");
}

// ── PFMERGE ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_pfmerge_basic() {
    let (_m, mut c) = start().await;

    // Create two non-overlapping HLLs
    for i in 0..100 {
        must_int!(c, "PFADD", "h1", &format!("item-{}", i); 1);
    }
    for i in 100..200 {
        must_int!(c, "PFADD", "h3", &format!("item-{}", i); 1);
    }

    // Merge non-intersecting
    must_ok!(c, "PFMERGE", "res1", "h1", "h3");
    let count: i64 = redis::cmd("PFCOUNT")
        .arg("res1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!((195..=205).contains(&count), "expected ~200, got {}", count);
}

#[tokio::test]
async fn test_pfmerge_overlapping() {
    let (_m, mut c) = start().await;

    // Create overlapping HLLs
    for i in 0..100 {
        must_int!(c, "PFADD", "h1", &format!("item-{}", i); 1);
        if i % 2 == 0 {
            must_int!(c, "PFADD", "h2", &format!("item-{}", i); 1);
        }
    }

    // h1 has 100, h2 has 50 (all in h1)
    must_int!(c, "PFCOUNT", "h1"; 100);
    must_int!(c, "PFCOUNT", "h2"; 50);

    // Merge overlapping → should be 100 (union)
    must_ok!(c, "PFMERGE", "res2", "h1", "h2");
    must_int!(c, "PFCOUNT", "res2"; 100);
}

#[tokio::test]
async fn test_pfmerge_with_empty() {
    let (_m, mut c) = start().await;

    must_int!(c, "PFADD", "h1", "a", "b", "c"; 1);

    // Merge with empty/non-existent key
    must_ok!(c, "PFMERGE", "res", "h1", "h_empty");
    must_int!(c, "PFCOUNT", "res"; 3);
}

#[tokio::test]
async fn test_pfmerge_dest_only() {
    let (_m, mut c) = start().await;

    // PFMERGE with just dest (no sources) — creates empty HLL
    must_ok!(c, "PFMERGE", "dest");
    must_int!(c, "PFCOUNT", "dest"; 0);
    must_str!(c, "TYPE", "dest"; "hll");
}

#[tokio::test]
async fn test_pfmerge_errors() {
    let (_m, mut c) = start().await;

    must_ok!(c, "SET", "str", "value");

    // Wrong number of arguments
    must_fail!(c, "PFMERGE"; "wrong number of arguments");

    // Wrong type source
    must_fail!(c, "PFMERGE", "h10", "str"; "not a valid HyperLogLog string value");
}

// ── DEL interaction ──────────────────────────────────────────────────

#[tokio::test]
async fn test_hll_del() {
    let (_m, mut c) = start().await;

    must_int!(c, "PFADD", "h", "a", "b", "c"; 1);
    must_int!(c, "PFCOUNT", "h"; 3);

    // DEL the key
    must_int!(c, "DEL", "h"; 1);

    // Count should be 0 now
    must_int!(c, "PFCOUNT", "h"; 0);
}
