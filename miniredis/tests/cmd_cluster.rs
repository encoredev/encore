mod helpers;

#[tokio::test]
async fn test_cluster_slots() {
    let (_m, mut c) = helpers::start().await;

    let v: redis::Value = redis::cmd("CLUSTER")
        .arg("SLOTS")
        .query_async(&mut c)
        .await
        .unwrap();

    // Should return a nested array with slot range 0-16383
    match v {
        redis::Value::Array(slots) => {
            assert_eq!(slots.len(), 1);
            match &slots[0] {
                redis::Value::Array(range) => {
                    assert!(range.len() >= 3);
                    // start slot = 0
                    assert_eq!(range[0], redis::Value::Int(0));
                    // end slot = 16383
                    assert_eq!(range[1], redis::Value::Int(16383));
                }
                _ => panic!("expected array for slot range, got {:?}", slots[0]),
            }
        }
        _ => panic!("expected array from CLUSTER SLOTS, got {:?}", v),
    }
}

#[tokio::test]
async fn test_cluster_nodes() {
    let (_m, mut c) = helpers::start().await;

    let v: String = redis::cmd("CLUSTER")
        .arg("NODES")
        .query_async(&mut c)
        .await
        .unwrap();

    assert!(v.contains("myself,master"));
    assert!(v.contains("connected 0-16383"));
}

#[tokio::test]
async fn test_cluster_keyslot() {
    let (_m, mut c) = helpers::start().await;

    let v: i64 = redis::cmd("CLUSTER")
        .arg("KEYSLOT")
        .arg("{test_key}")
        .query_async(&mut c)
        .await
        .unwrap();

    assert_eq!(v, 163);
}

#[tokio::test]
async fn test_cluster_shards() {
    let (_m, mut c) = helpers::start().await;

    let v: redis::Value = redis::cmd("CLUSTER")
        .arg("SHARDS")
        .query_async(&mut c)
        .await
        .unwrap();

    // Should return a nested array with shard info
    match v {
        redis::Value::Array(shards) => {
            assert_eq!(shards.len(), 1);
        }
        _ => panic!("expected array from CLUSTER SHARDS, got {:?}", v),
    }
}

#[tokio::test]
async fn test_cluster_errors() {
    let (_m, mut c) = helpers::start().await;

    must_fail!(c, "CLUSTER"; "wrong number of arguments");
    must_fail!(c, "CLUSTER", "NOSUCHSUB"; "unknown subcommand");
}
