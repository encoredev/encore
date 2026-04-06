use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;

use anyhow::{Context, Result};
use azservicebus::prelude::{
    ServiceBusMessage, ServiceBusReceivedMessage, ServiceBusReceiver, ServiceBusReceiverOptions,
    ServiceBusSender, ServiceBusSenderOptions,
};
use azservicebus::receiver::DeadLetterOptions;
use fe2o3_amqp_types::messaging::ApplicationProperties;
use fe2o3_amqp_types::primitives::{OrderedMap, SimpleValue};
use time::OffsetDateTime;

use crate::api::APIResult;
use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::names::CloudName;
use crate::pubsub::azure::LazyAzureClient;
use crate::pubsub::manager::SubHandler;
use crate::pubsub::{self, MessageId};

/// Application property key used to track the encore retry count across scheduled retries.
/// Matches the Go runtime convention so that cross-runtime interoperability is preserved.
const ENCORE_RETRY_COUNT_ATTR: &str = "encore-retry-count";

/// Base delay for the first retry.
const RETRY_BASE_SECS: u64 = 1;

/// Maximum retry delay (matches Go runtime cap).
const RETRY_MAX_SECS: u64 = 600;

/// Maximum number of messages to fetch in one batch.
const MAX_BATCH_SIZE: u32 = 100;

/// How long to wait for messages in each receive call.
///
/// Using a bounded wait time ensures that processing tasks waiting to settle
/// messages (complete / abandon / dead-letter) get a chance to acquire the
/// shared receiver mutex after each receive window completes.  Choose a value
/// comfortably shorter than the subscription's lock duration (typically ≥ 30 s).
const RECEIVE_TIMEOUT: Duration = Duration::from_secs(20);

/// Base sleep duration after a receive error, doubles on each consecutive error.
const ERR_SLEEP_BASE: Duration = Duration::from_millis(500);
const ERR_SLEEP_MAX: Duration = Duration::from_secs(30);

#[derive(Debug)]
pub struct Subscription {
    client: Arc<LazyAzureClient>,
    topic_cloud_name: CloudName,
    subscription_cloud_name: CloudName,
    max_concurrency: usize,
    /// Maximum number of delivery attempts before the message is dead-lettered.
    /// When `None`, dead-lettering is delegated to Azure's built-in
    /// `max_delivery_count` setting on the subscription.
    max_retries: Option<u32>,
}

impl Subscription {
    pub(super) fn new(
        client: Arc<LazyAzureClient>,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Self {
        // Only honour max_retries when explicitly set to a positive number.
        let max_retries = meta
            .retry_policy
            .as_ref()
            .map(|r| r.max_retries as u32)
            .filter(|&n| n > 0);

        Self {
            client,
            topic_cloud_name: cfg.topic_cloud_name.clone().into(),
            subscription_cloud_name: cfg.subscription_cloud_name.clone().into(),
            max_concurrency: meta.max_concurrency.unwrap_or(100) as usize,
            max_retries,
        }
    }
}

impl pubsub::Subscription for Subscription {
    fn subscribe(
        &self,
        handler: Arc<SubHandler>,
    ) -> Pin<Box<dyn Future<Output = APIResult<()>> + Send + 'static>> {
        let client = self.client.clone();
        let topic = self.topic_cloud_name.to_string();
        let sub = self.subscription_cloud_name.to_string();
        let max_concurrency = self.max_concurrency;
        let max_retries = self.max_retries;

        // Resolve the sender eagerly so we can move it into the async block.
        // If sender creation fails here we fall back to abandon on retry (logged per-message).
        let sender_fut = {
            // We pin to self's lifetime via a cloned client so the future is 'static.
            let client_for_sender = client.clone();
            let topic_for_sender = topic.clone();
            async move {
                let arc_client = client_for_sender.get().await.as_ref().ok()?.clone();
                let mut c = arc_client.lock().await;
                c.create_sender(topic_for_sender, ServiceBusSenderOptions::default())
                    .await
                    .ok()
                    .map(|s| Arc::new(tokio::sync::Mutex::new(s)))
            }
        };

        Box::pin(async move {
            // Resolve the lazily-initialised Azure Service Bus client.
            let arc_client = match client.get().await {
                Ok(c) => c.clone(),
                Err(e) => {
                    return Err(crate::api::Error::internal(anyhow::anyhow!(
                        "failed to get Azure Service Bus client: {}",
                        e
                    )));
                }
            };

            // Create the AMQP receiver link for this subscription.
            let receiver = {
                let mut client_guard = arc_client.lock().await;
                client_guard
                    .create_receiver_for_subscription(
                        topic,
                        sub,
                        ServiceBusReceiverOptions::default(),
                    )
                    .await
                    .context("failed to create Azure Service Bus receiver")
                    .map_err(crate::api::Error::internal)?
            };

            // Resolve the sender used for scheduling delayed retries.
            let sender = sender_fut.await;

            let receiver = Arc::new(tokio::sync::Mutex::new(receiver));
            let sem = Arc::new(tokio::sync::Semaphore::new(max_concurrency));

            subscribe_loop(receiver, sender, handler, sem, max_retries).await;
            Ok(())
        })
    }
}

/// Core receive-process loop.
///
/// Receives messages in bounded windows so that spawned settlement tasks
/// (complete / abandon / dead-letter) can periodically acquire the shared
/// receiver mutex between receive calls.
async fn subscribe_loop(
    receiver: Arc<tokio::sync::Mutex<ServiceBusReceiver>>,
    sender: Option<Arc<tokio::sync::Mutex<ServiceBusSender>>>,
    handler: Arc<SubHandler>,
    sem: Arc<tokio::sync::Semaphore>,
    max_retries: Option<u32>,
) {
    let mut err_sleep = ERR_SLEEP_BASE;

    loop {
        // Determine how many messages to request based on available capacity.
        let available = sem.available_permits().max(1).min(MAX_BATCH_SIZE as usize);

        // Receive messages.  The bounded wait time releases the mutex so that
        // concurrent settlement tasks can proceed.
        let msgs = {
            let mut recv = receiver.lock().await;
            match recv
                .receive_messages_with_max_wait_time(available as u32, Some(RECEIVE_TIMEOUT))
                .await
            {
                Ok(msgs) => {
                    err_sleep = ERR_SLEEP_BASE;
                    msgs
                }
                Err(e) => {
                    log::error!(
                        "encore: Azure Service Bus receive error, retrying in {:?}: {}",
                        err_sleep,
                        e
                    );
                    drop(recv);
                    tokio::time::sleep(err_sleep).await;
                    err_sleep = err_sleep.mul_f32(2.0).min(ERR_SLEEP_MAX);
                    continue;
                }
            }
        }; // receiver mutex released here

        if msgs.is_empty() {
            // No messages in this window; loop immediately to try again.
            continue;
        }

        // Spawn a processing task for each received message.
        for msg in msgs {
            let permit = sem.clone().acquire_owned().await.expect("semaphore closed");

            let handler = handler.clone();
            let receiver = receiver.clone();
            let sender = sender.clone();

            tokio::spawn(async move {
                let _permit = permit; // held until this task completes
                process_message(receiver, sender, handler, msg, max_retries).await;
            });
        }
    }
}

/// Process a single message: invoke the handler then settle with the service.
async fn process_message(
    receiver: Arc<tokio::sync::Mutex<ServiceBusReceiver>>,
    sender: Option<Arc<tokio::sync::Mutex<ServiceBusSender>>>,
    handler: Arc<SubHandler>,
    msg: ServiceBusReceivedMessage,
    max_retries: Option<u32>,
) {
    // Derive the logical attempt number from the encore-retry-count attribute so
    // that it stays accurate across scheduled retries (which reset the Azure
    // native delivery_count by creating a new message).  Fall back to
    // delivery_count for messages that pre-date this retry scheme.
    let encore_retry_count: u32 = msg
        .application_properties()
        .and_then(|props| props.0.get(ENCORE_RETRY_COUNT_ATTR))
        .and_then(|v| match v {
            SimpleValue::String(s) => s.parse().ok(),
            SimpleValue::Uint(n) => Some(*n),
            SimpleValue::Long(n) => Some(*n as u32),
            _ => None,
        })
        .unwrap_or(0);
    let attempt = encore_retry_count + 1;

    let handler_result = match parse_message(&msg, attempt) {
        Ok(pubsub_msg) => handler
            .handle_message(pubsub_msg)
            .await
            .map_err(|e| anyhow::anyhow!("{}", e)),
        Err(e) => {
            log::error!(
                "encore: failed to parse Azure Service Bus message: {:#?}",
                e
            );
            Err(e)
        }
    };

    let mut recv = receiver.lock().await;

    match handler_result {
        Ok(()) => {
            if let Err(e) = recv.complete_message(&msg).await {
                log::error!(
                    "encore: failed to complete Azure Service Bus message: {}",
                    e
                );
            }
        }
        Err(_) => {
            let should_dead_letter = max_retries.map_or(false, |max| attempt > max);

            if should_dead_letter {
                let opts = DeadLetterOptions {
                    dead_letter_reason: Some("ExhaustedRetries".to_string()),
                    dead_letter_error_description: Some(format!(
                        "Message processing failed after {} delivery attempt(s)",
                        attempt
                    )),
                    properties_to_modify: None,
                };
                if let Err(e) = recv.dead_letter_message(&msg, opts).await {
                    log::error!(
                        "encore: failed to dead-letter Azure Service Bus message: {}",
                        e
                    );
                    // Fall back to abandon so the message is not silently lost.
                    if let Err(ae) = recv.abandon_message(&msg, None).await {
                        log::error!(
                            "encore: failed to abandon Azure Service Bus message after \
                             dead-letter failure: {}",
                            ae
                        );
                    }
                }
            } else {
                // Compute exponential backoff: base 1s × 2^(attempt−1), capped at 600s.
                // This mirrors the Go runtime's retry delay calculation.
                let backoff_secs = retry_backoff_secs(attempt);
                let backoff = Duration::from_secs(backoff_secs);

                match sender {
                    Some(ref arc_sender) => {
                        // Build a new message carrying the same body and application
                        // properties as the original, with encore-retry-count incremented.
                        let scheduled = build_retry_message(&msg, encore_retry_count + 1);

                        let enqueue_at = OffsetDateTime::now_utc()
                            + time::Duration::seconds(backoff_secs as i64);

                        let schedule_result = {
                            let mut sender_guard = arc_sender.lock().await;
                            sender_guard.schedule_message(scheduled, enqueue_at).await
                        };

                        match schedule_result {
                            Ok(_seq) => {
                                // Successfully scheduled — complete (remove) the original
                                // message so it does not count against the Azure delivery limit.
                                if let Err(e) = recv.complete_message(&msg).await {
                                    log::error!(
                                        "encore: failed to complete Azure Service Bus message \
                                         after scheduling retry: {}",
                                        e
                                    );
                                }
                                log::debug!(
                                    "encore: scheduled Azure Service Bus retry in {:?} \
                                     (attempt {})",
                                    backoff,
                                    attempt
                                );
                            }
                            Err(e) => {
                                log::error!(
                                    "encore: failed to schedule Azure Service Bus retry, \
                                     falling back to abandon: {}",
                                    e
                                );
                                if let Err(ae) = recv.abandon_message(&msg, None).await {
                                    log::error!(
                                        "encore: failed to abandon Azure Service Bus message: {}",
                                        ae
                                    );
                                }
                            }
                        }
                    }
                    None => {
                        // No sender available — fall back to plain abandon.  Azure will
                        // re-deliver the message immediately without backoff.  Consider
                        // ensuring a sender can be created to enable backoff scheduling.
                        if let Err(e) = recv.abandon_message(&msg, None).await {
                            log::error!(
                                "encore: failed to abandon Azure Service Bus message: {}",
                                e
                            );
                        }
                    }
                }
            }
        }
    }
}

/// Compute an exponential backoff delay for the given attempt number.
///
/// Returns `base × 2^(attempt−1)` capped at [`RETRY_MAX_SECS`], matching the
/// Go runtime's behaviour.
fn retry_backoff_secs(attempt: u32) -> u64 {
    RETRY_BASE_SECS
        .saturating_mul(1u64.checked_shl((attempt - 1).min(63)).unwrap_or(0))
        .min(RETRY_MAX_SECS)
}

/// Build a new [`ServiceBusMessage`] suitable for scheduling as a retry.
///
/// Copies the body and all application properties from the original received
/// message, then sets `encore-retry-count` to `new_retry_count`.
fn build_retry_message(
    original: &ServiceBusReceivedMessage,
    new_retry_count: u32,
) -> ServiceBusMessage {
    let body = original.body().map(|b| b.to_vec()).unwrap_or_default();

    let mut new_msg = ServiceBusMessage::new(body);

    // Copy existing application properties and update the retry counter.
    let app_props = new_msg
        .application_properties_mut()
        .get_or_insert_with(|| ApplicationProperties(OrderedMap::new()));

    if let Some(orig_props) = original.application_properties() {
        for (k, v) in &orig_props.0 {
            if k.as_str() != ENCORE_RETRY_COUNT_ATTR {
                app_props.0.insert(k.clone(), v.clone());
            }
        }
    }

    app_props.0.insert(
        ENCORE_RETRY_COUNT_ATTR.to_string(),
        SimpleValue::String(new_retry_count.to_string()),
    );

    new_msg
}

fn parse_message(item: &ServiceBusReceivedMessage, attempt: u32) -> Result<pubsub::Message> {
    let body = item
        .body()
        .map_err(|e| anyhow::anyhow!("failed to read Azure Service Bus message body: {:?}", e))?;
    let raw_body = body.to_vec();

    let id: Option<String> = item.message_id().map(|s| s.into_owned());

    let enqueued = item.enqueued_time();

    // Convert Azure AMQP application properties to plain string key/value pairs.
    let attrs: HashMap<String, String> = item
        .application_properties()
        .map(|props| {
            props
                .0
                .iter()
                .map(|(k, v)| {
                    let s = match v {
                        SimpleValue::String(s) => s.clone(),
                        other => format!("{:?}", other),
                    };
                    (k.clone(), s)
                })
                .collect()
        })
        .unwrap_or_default();

    Ok(build_pubsub_message(raw_body, id, enqueued, attrs, attempt))
}

/// Constructs a [`pubsub::Message`] from its raw parts.
///
/// Extracted from [`parse_message`] so that the mapping logic (ID fallback,
/// timestamp conversion, attribute passthrough) can be tested independently of
/// the Azure Service Bus SDK types.
fn build_pubsub_message(
    raw_body: Vec<u8>,
    id: Option<String>,
    enqueued: time::OffsetDateTime,
    attrs: HashMap<String, String>,
    attempt: u32,
) -> pubsub::Message {
    let id: MessageId = id.unwrap_or_else(|| xid::new().to_string());
    let publish_time =
        chrono::DateTime::from_timestamp(enqueued.unix_timestamp(), enqueued.nanosecond());

    pubsub::Message {
        id,
        publish_time,
        attempt,
        data: pubsub::MessageData { attrs, raw_body },
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn fixed_time(unix_secs: i64) -> time::OffsetDateTime {
        time::OffsetDateTime::from_unix_timestamp(unix_secs)
            .expect("valid unix timestamp")
    }

    #[test]
    fn test_build_pubsub_message_with_id_and_attrs() {
        let body = b"hello world".to_vec();
        let id = Some("msg-abc-123".to_string());
        let enqueued = fixed_time(1_700_000_000);
        let mut attrs = HashMap::new();
        attrs.insert("env".to_string(), "production".to_string());
        attrs.insert("version".to_string(), "2".to_string());

        let msg = build_pubsub_message(body.clone(), id.clone(), enqueued, attrs.clone(), 1);

        assert_eq!(msg.id, "msg-abc-123");
        assert_eq!(msg.attempt, 1);
        assert_eq!(msg.data.raw_body, body);
        assert_eq!(msg.data.attrs.get("env").map(String::as_str), Some("production"));
        assert_eq!(msg.data.attrs.get("version").map(String::as_str), Some("2"));

        // publish_time should be set from enqueued timestamp.
        let ts = msg.publish_time.expect("publish_time should be set");
        assert_eq!(ts.timestamp(), 1_700_000_000);
    }

    #[test]
    fn test_build_pubsub_message_no_id_generates_one() {
        let msg = build_pubsub_message(
            vec![],
            None, // no explicit ID
            fixed_time(1_000_000),
            HashMap::new(),
            3,
        );

        // A generated xid is always non-empty.
        assert!(!msg.id.is_empty(), "generated ID must be non-empty");
        assert_eq!(msg.attempt, 3);
        assert!(msg.data.raw_body.is_empty());
    }

    #[test]
    fn test_build_pubsub_message_empty_attrs() {
        let msg = build_pubsub_message(
            b"data".to_vec(),
            Some("id1".to_string()),
            fixed_time(0),
            HashMap::new(),
            1,
        );

        assert!(msg.data.attrs.is_empty());
    }

    #[test]
    fn test_build_pubsub_message_high_attempt() {
        let msg = build_pubsub_message(
            vec![],
            Some("retry-msg".to_string()),
            fixed_time(1_600_000_000),
            HashMap::new(),
            99,
        );

        assert_eq!(msg.attempt, 99);
    }
}
