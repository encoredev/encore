/// Pub/Sub message delivery infrastructure.
use std::collections::HashSet;
use std::sync::{Arc, Mutex};
use tokio::sync::mpsc;

/// A message delivered to a pub/sub subscriber.
pub struct PubsubMessage {
    /// "message" or "pmessage"
    pub kind: &'static str,
    /// The pattern that matched (for pmessage only)
    pub pattern: Option<String>,
    /// The channel the message was published to
    pub channel: String,
    /// The message payload
    pub data: String,
}

/// A pattern matcher: (pattern_string, compiled_matcher).
pub type PatternMatcher = (String, Box<dyn Fn(&str) -> bool + Send + Sync>);

/// Per-subscriber state: channels, patterns, and message sender.
pub struct SubscriberInner {
    pub channels: HashSet<String>,
    pub patterns: Vec<PatternMatcher>,
    pub tx: mpsc::UnboundedSender<PubsubMessage>,
}

/// Handle to a subscriber, stored in the global registry.
pub type SubscriberHandle = Arc<Mutex<SubscriberInner>>;

/// Global pub/sub subscriber registry.
pub struct PubsubRegistry {
    subscribers: Vec<SubscriberHandle>,
}

impl Default for PubsubRegistry {
    fn default() -> Self {
        Self::new()
    }
}

impl PubsubRegistry {
    pub fn new() -> Self {
        PubsubRegistry {
            subscribers: Vec::new(),
        }
    }

    pub fn add(&mut self, handle: SubscriberHandle) {
        self.subscribers.push(handle);
    }

    pub fn remove(&mut self, handle: &SubscriberHandle) {
        self.subscribers.retain(|s| !Arc::ptr_eq(s, handle));
    }

    /// Publish a message to all matching subscribers. Returns total delivery count.
    pub fn publish(&self, channel: &str, message: &str) -> i64 {
        let mut count = 0i64;
        for sub in &self.subscribers {
            let inner = sub.lock().unwrap();
            // Check direct channel subscription
            if inner.channels.contains(channel) {
                let _ = inner.tx.send(PubsubMessage {
                    kind: "message",
                    pattern: None,
                    channel: channel.to_string(),
                    data: message.to_string(),
                });
                count += 1;
            }
            // Check pattern subscriptions
            for (pat_str, matcher) in &inner.patterns {
                if matcher(channel) {
                    let _ = inner.tx.send(PubsubMessage {
                        kind: "pmessage",
                        pattern: Some(pat_str.clone()),
                        channel: channel.to_string(),
                        data: message.to_string(),
                    });
                    count += 1;
                    break; // only one match per subscriber
                }
            }
        }
        count
    }

    /// Return all unique channels that have at least one subscriber, optionally filtered by pattern.
    pub fn active_channels(&self, pattern: Option<&str>) -> Vec<String> {
        let mut channels = HashSet::new();
        for sub in &self.subscribers {
            let inner = sub.lock().unwrap();
            for ch in &inner.channels {
                channels.insert(ch.clone());
            }
        }
        let mut result: Vec<String> = if let Some(pat) = pattern {
            channels
                .into_iter()
                .filter(|ch| crate::keys::glob_match(pat, ch))
                .collect()
        } else {
            channels.into_iter().collect()
        };
        result.sort();
        result
    }

    /// Count subscribers for a specific channel (direct subscriptions only).
    pub fn numsub(&self, channel: &str) -> i64 {
        let mut count = 0i64;
        for sub in &self.subscribers {
            let inner = sub.lock().unwrap();
            if inner.channels.contains(channel) {
                count += 1;
            }
        }
        count
    }

    /// Total number of pattern subscriptions across all subscribers.
    pub fn numpat(&self) -> i64 {
        let mut count = 0i64;
        for sub in &self.subscribers {
            let inner = sub.lock().unwrap();
            count += inner.patterns.len() as i64;
        }
        count
    }
}

/// Per-connection pub/sub context.
pub struct PubsubCtx {
    /// Shared handle registered in the global registry.
    pub handle: SubscriberHandle,
    /// Receiver for pub/sub messages.
    pub rx: mpsc::UnboundedReceiver<PubsubMessage>,
}

impl PubsubCtx {
    /// Create a new pub/sub context. Returns the context and registers it in the registry.
    pub fn new(registry: &mut PubsubRegistry) -> Self {
        let (tx, rx) = mpsc::unbounded_channel();
        let inner = SubscriberInner {
            channels: HashSet::new(),
            patterns: Vec::new(),
            tx,
        };
        let handle = Arc::new(Mutex::new(inner));
        registry.add(handle.clone());
        PubsubCtx { handle, rx }
    }

    /// Subscribe to a channel. Returns the total subscription count.
    pub fn subscribe(&self, channel: &str) -> usize {
        let mut inner = self.handle.lock().unwrap();
        inner.channels.insert(channel.to_string());
        inner.channels.len() + inner.patterns.len()
    }

    /// Unsubscribe from a channel. Returns the total subscription count.
    pub fn unsubscribe(&self, channel: &str) -> usize {
        let mut inner = self.handle.lock().unwrap();
        inner.channels.remove(channel);
        inner.channels.len() + inner.patterns.len()
    }

    /// Subscribe to a pattern. Returns the total subscription count.
    pub fn psubscribe(&self, pattern: &str) -> usize {
        let pat = pattern.to_string();
        let pat_clone = pat.clone();
        let matcher: Box<dyn Fn(&str) -> bool + Send + Sync> =
            Box::new(move |text: &str| crate::keys::glob_match(&pat_clone, text));
        let mut inner = self.handle.lock().unwrap();
        inner.patterns.push((pat, matcher));
        inner.channels.len() + inner.patterns.len()
    }

    /// Unsubscribe from a pattern. Returns the total subscription count.
    pub fn punsubscribe(&self, pattern: &str) -> usize {
        let mut inner = self.handle.lock().unwrap();
        inner.patterns.retain(|(p, _)| p != pattern);
        inner.channels.len() + inner.patterns.len()
    }

    /// Get all subscribed channel names.
    pub fn channels(&self) -> Vec<String> {
        let inner = self.handle.lock().unwrap();
        let mut channels: Vec<String> = inner.channels.iter().cloned().collect();
        channels.sort();
        channels
    }

    /// Get all subscribed pattern strings.
    pub fn patterns(&self) -> Vec<String> {
        let inner = self.handle.lock().unwrap();
        inner.patterns.iter().map(|(p, _)| p.clone()).collect()
    }

    /// Total subscription count (channels + patterns).
    pub fn total_count(&self) -> usize {
        let inner = self.handle.lock().unwrap();
        inner.channels.len() + inner.patterns.len()
    }
}
