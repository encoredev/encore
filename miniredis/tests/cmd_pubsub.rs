mod helpers;
use helpers::*;

// ── PUBLISH (through regular dispatch) ──────────────────────────────

#[tokio::test]
async fn test_publish_no_subscribers() {
    let (_m, mut c) = start().await;

    // No subscribers → 0
    must_int!(c, "PUBLISH", "ch", "hello"; 0);
}

#[tokio::test]
async fn test_publish_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "PUBLISH"; "wrong number of arguments");
    must_fail!(c, "PUBLISH", "ch"; "wrong number of arguments");
    must_fail!(c, "PUBLISH", "ch", "msg", "extra"; "wrong number of arguments");
}

// ── PUBSUB CHANNELS/NUMSUB/NUMPAT (through regular dispatch) ───────

#[tokio::test]
async fn test_pubsub_channels_empty() {
    let (_m, mut c) = start().await;

    let v: Vec<String> = redis::cmd("PUBSUB")
        .arg("CHANNELS")
        .query_async(&mut c)
        .await
        .unwrap();
    assert!(v.is_empty());
}

#[tokio::test]
async fn test_pubsub_numsub_empty() {
    let (_m, mut c) = start().await;

    let v: redis::Value = redis::cmd("PUBSUB")
        .arg("NUMSUB")
        .arg("ch1")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 2);
            // channel name, count (0)
        }
        _ => panic!("expected array from PUBSUB NUMSUB, got {:?}", v),
    }
}

#[tokio::test]
async fn test_pubsub_numpat_empty() {
    let (_m, mut c) = start().await;

    must_int!(c, "PUBSUB", "NUMPAT"; 0);
}

// ── SUBSCRIBE + PUBLISH via redis-rs PubSub API ────────────────────

#[tokio::test]
async fn test_subscribe_and_publish() {
    let (m, mut c) = start().await;

    // Create a subscriber using redis-rs PubSub
    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    // Subscribe to a channel
    pubsub.subscribe("ch1").await.unwrap();

    // Give a moment for subscription to register
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // Publish a message
    must_int!(c, "PUBLISH", "ch1", "hello"; 1);

    // Receive the message
    let msg = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        pubsub.on_message().next(),
    )
    .await
    .expect("timeout waiting for message")
    .expect("no message received");

    let payload: String = msg.get_payload().unwrap();
    assert_eq!(payload, "hello");
    assert_eq!(msg.get_channel_name(), "ch1");
}

#[tokio::test]
async fn test_subscribe_multiple_channels() {
    let (m, mut c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.subscribe("ch1").await.unwrap();
    pubsub.subscribe("ch2").await.unwrap();

    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // Publish to ch1
    must_int!(c, "PUBLISH", "ch1", "msg1"; 1);
    // Publish to ch2
    must_int!(c, "PUBLISH", "ch2", "msg2"; 1);

    // Receive both messages
    let msg1 = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        pubsub.on_message().next(),
    )
    .await
    .expect("timeout")
    .expect("no message");
    let payload1: String = msg1.get_payload().unwrap();

    let msg2 = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        pubsub.on_message().next(),
    )
    .await
    .expect("timeout")
    .expect("no message");
    let payload2: String = msg2.get_payload().unwrap();

    let mut payloads = vec![payload1, payload2];
    payloads.sort();
    assert_eq!(payloads, vec!["msg1", "msg2"]);
}

#[tokio::test]
async fn test_psubscribe() {
    let (m, mut c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.psubscribe("event*").await.unwrap();

    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // Publish to a matching channel
    must_int!(c, "PUBLISH", "event123", "data"; 1);

    // Non-matching channel
    must_int!(c, "PUBLISH", "other", "data"; 0);

    // Receive the matching message
    let msg = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        pubsub.on_message().next(),
    )
    .await
    .expect("timeout")
    .expect("no message");

    let payload: String = msg.get_payload().unwrap();
    assert_eq!(payload, "data");
}

#[tokio::test]
async fn test_unsubscribe() {
    let (m, mut c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.subscribe("ch1").await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // Can receive
    must_int!(c, "PUBLISH", "ch1", "before"; 1);
    let msg = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        pubsub.on_message().next(),
    )
    .await
    .expect("timeout")
    .expect("no message");
    let payload: String = msg.get_payload().unwrap();
    assert_eq!(payload, "before");

    // Unsubscribe
    pubsub.unsubscribe("ch1").await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // Should have 0 subscribers now
    must_int!(c, "PUBLISH", "ch1", "after"; 0);
}

#[tokio::test]
async fn test_pubsub_channels_with_subscriber() {
    let (m, mut c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.subscribe("news").await.unwrap();
    pubsub.subscribe("sports").await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // PUBSUB CHANNELS should list both
    let mut channels: Vec<String> = redis::cmd("PUBSUB")
        .arg("CHANNELS")
        .query_async(&mut c)
        .await
        .unwrap();
    channels.sort();
    assert_eq!(channels, vec!["news", "sports"]);

    // PUBSUB CHANNELS with pattern
    let channels: Vec<String> = redis::cmd("PUBSUB")
        .arg("CHANNELS")
        .arg("n*")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(channels, vec!["news"]);
}

#[tokio::test]
async fn test_pubsub_numsub_with_subscriber() {
    let (m, mut c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.subscribe("ch1").await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    let v: redis::Value = redis::cmd("PUBSUB")
        .arg("NUMSUB")
        .arg("ch1")
        .arg("ch2")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(items) => {
            assert_eq!(items.len(), 4); // ch1, count, ch2, count
        }
        _ => panic!("expected array from PUBSUB NUMSUB, got {:?}", v),
    }
}

#[tokio::test]
async fn test_pubsub_numpat_with_subscriber() {
    let (m, mut c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.psubscribe("event*").await.unwrap();
    pubsub.psubscribe("news*").await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    must_int!(c, "PUBSUB", "NUMPAT"; 2);
}

// ── Direct API ──────────────────────────────────────────────────────

#[tokio::test]
async fn test_publish_direct_api() {
    let (m, _c) = start().await;

    let sub_client = redis::Client::open(m.redis_url()).unwrap();
    let mut pubsub = sub_client.get_async_pubsub().await.unwrap();

    pubsub.subscribe("ch1").await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(50)).await;

    // Use direct API to publish
    let count = m.publish("ch1", "hello-direct");
    assert_eq!(count, 1);

    // Receive the message
    let msg = tokio::time::timeout(
        std::time::Duration::from_secs(2),
        pubsub.on_message().next(),
    )
    .await
    .expect("timeout")
    .expect("no message");

    let payload: String = msg.get_payload().unwrap();
    assert_eq!(payload, "hello-direct");
}

// Need this import for Stream trait used by on_message()
use futures_lite::StreamExt;
