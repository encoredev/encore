// This code is a Rust port of miniredis by Harmen (https://github.com/alicebob/miniredis),
// originally licensed under the MIT License. See MINIREDIS_LICENSE.txt for the original license.

//! # miniredis-rs
//!
//! Pure Rust in-memory Redis test server, for use in Rust integration tests.
//!
//! Start a server with `Miniredis::run().await`, it will listen on a random port.
//! Point your Redis client to `m.redis_url()` or `m.addr()`.
//!
//! ```rust
//! # async fn example() {
//! let m = miniredis_rs::Miniredis::run().await.unwrap();
//! m.set("foo", "bar");
//! // Use m.redis_url() with any Redis client
//! m.close().await;
//! # }
//! ```

pub mod cmd;
pub mod connection;
pub mod db;
pub mod dispatch;
pub mod frame;
pub mod geo;
pub mod hll;
pub mod keys;
pub mod pubsub;
pub mod server;
pub mod types;

mod error;

pub use error::{Error, Result};

use std::net::SocketAddr;
use std::sync::Arc;
use std::time::{Duration, SystemTime};

use tokio::net::TcpListener;

use crate::db::SharedState;

/// A running miniredis instance for use in tests.
///
/// Create one with [`Miniredis::run()`], use [`addr()`](Miniredis::addr) or
/// [`redis_url()`](Miniredis::redis_url) to connect a client, and call
/// [`close()`](Miniredis::close) when done.
#[derive(Clone)]
pub struct Miniredis {
    state: Arc<SharedState>,
    addr: SocketAddr,
    selected_db: usize,
}

impl Miniredis {
    /// Start a new miniredis server on a random available port (127.0.0.1:0).
    pub async fn run() -> Result<Self> {
        Self::run_addr("127.0.0.1:0").await
    }

    /// Start a new miniredis server on the given address.
    pub async fn run_addr(addr: &str) -> Result<Self> {
        let listener = TcpListener::bind(addr).await?;
        let local_addr = listener.local_addr()?;
        let state = SharedState::new();
        let state_clone = Arc::clone(&state);
        let shutdown_rx = state.shutdown_tx.subscribe();

        tokio::spawn(async move {
            server::run(listener, state_clone, shutdown_rx, None).await;
        });

        Ok(Miniredis {
            state,
            addr: local_addr,
            selected_db: 0,
        })
    }

    /// Start a new miniredis server with TLS on a random port.
    ///
    /// Connections must use TLS (rediss:// scheme). Use `tls_url()` for the URL.
    #[cfg(feature = "tls")]
    pub async fn run_tls(tls_config: Arc<rustls::ServerConfig>) -> Result<Self> {
        Self::run_tls_addr("127.0.0.1:0", tls_config).await
    }

    /// Start a new miniredis server with TLS on the given address.
    #[cfg(feature = "tls")]
    pub async fn run_tls_addr(addr: &str, tls_config: Arc<rustls::ServerConfig>) -> Result<Self> {
        let listener = TcpListener::bind(addr).await?;
        let local_addr = listener.local_addr()?;
        let state = SharedState::new();
        let state_clone = Arc::clone(&state);
        let shutdown_rx = state.shutdown_tx.subscribe();
        let acceptor = tokio_rustls::TlsAcceptor::from(tls_config);

        tokio::spawn(async move {
            server::run(listener, state_clone, shutdown_rx, Some(acceptor)).await;
        });

        Ok(Miniredis {
            state,
            addr: local_addr,
            selected_db: 0,
        })
    }

    /// Shut down the server.
    pub async fn close(&self) {
        let _ = self.state.shutdown_tx.send(());
        // Give tasks a moment to clean up
        tokio::task::yield_now().await;
    }

    // ── Address helpers ──────────────────────────────────────────────

    /// The bound address as a `SocketAddr`.
    pub fn addr(&self) -> SocketAddr {
        self.addr
    }

    /// Just the host (e.g. "127.0.0.1").
    pub fn host(&self) -> String {
        self.addr.ip().to_string()
    }

    /// Just the port number.
    pub fn port(&self) -> u16 {
        self.addr.port()
    }

    /// A `redis://host:port` URL suitable for most Redis clients.
    pub fn redis_url(&self) -> String {
        format!("redis://{}:{}", self.addr.ip(), self.addr.port())
    }

    /// A `rediss://host:port` URL for TLS Redis clients.
    #[cfg(feature = "tls")]
    pub fn tls_url(&self) -> String {
        format!("rediss://{}:{}", self.addr.ip(), self.addr.port())
    }

    // ── Database selection ───────────────────────────────────────────

    /// Select the database used by the direct-access methods.
    pub fn select(&mut self, db: usize) {
        assert!(db < 16, "database index must be 0-15");
        self.selected_db = db;
    }

    // ── Authentication ───────────────────────────────────────────────

    /// Require AUTH with a password (default user).
    pub fn require_auth(&self, password: &str) {
        let mut inner = self.state.lock();
        inner
            .passwords
            .insert("default".to_owned(), password.to_owned());
    }

    /// Require AUTH with a username and password.
    pub fn require_user_auth(&self, username: &str, password: &str) {
        let mut inner = self.state.lock();
        inner
            .passwords
            .insert(username.to_owned(), password.to_owned());
    }

    // ── Time & determinism ───────────────────────────────────────────

    /// Set a fixed mock time. Affects EXPIREAT, stream IDs, etc.
    pub fn set_time(&self, t: SystemTime) {
        let mut inner = self.state.lock();
        inner.now = Some(t);
    }

    /// Decrease all TTLs by `duration`, expiring any that drop to zero.
    pub fn fast_forward(&self, duration: Duration) {
        let mut inner = self.state.lock();
        inner.fast_forward(duration);
    }

    /// Seed the random number generator for deterministic tests.
    pub fn seed(&self, seed: u64) {
        use rand::SeedableRng;
        let mut inner = self.state.lock();
        inner.rng = rand::rngs::StdRng::seed_from_u64(seed);
    }

    // ── Key management ───────────────────────────────────────────────

    /// Delete a key. Returns true if it existed.
    pub fn del(&self, key: &str) -> bool {
        let mut inner = self.state.lock();
        inner.db_mut(self.selected_db).del(key)
    }

    /// Check if a key exists.
    pub fn exists(&self, key: &str) -> bool {
        let mut inner = self.state.lock();
        let now = inner.effective_now();
        inner.db_mut(self.selected_db).exists(key, now)
    }

    /// Return the type of a key ("string", "list", "set", "hash", "zset",
    /// "stream", "none").
    pub fn key_type(&self, key: &str) -> &'static str {
        let inner = self.state.lock();
        match inner.db(self.selected_db).key_type(key) {
            Some(t) => t.as_str(),
            None => "none",
        }
    }

    /// Return all keys from the selected database, sorted.
    pub fn keys(&self) -> Vec<String> {
        let inner = self.state.lock();
        inner.db(self.selected_db).all_keys()
    }

    /// Get the TTL of a key. Returns None if the key has no TTL.
    pub fn ttl(&self, key: &str) -> Option<Duration> {
        let inner = self.state.lock();
        inner.db(self.selected_db).ttl.get(key).copied()
    }

    /// Set the TTL for a key.
    pub fn set_ttl(&self, key: &str, ttl: Duration) {
        let mut inner = self.state.lock();
        inner
            .db_mut(self.selected_db)
            .ttl
            .insert(key.to_owned(), ttl);
    }

    // ── String operations ────────────────────────────────────────────

    /// Get a string key value.
    pub fn get(&self, key: &str) -> Option<String> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.string_get(key)
            .map(|v| String::from_utf8_lossy(v).into_owned())
    }

    /// Set a string key. Removes any existing TTL.
    pub fn set(&self, key: &str, value: &str) {
        let mut inner = self.state.lock();
        let now = inner.effective_now();
        let db = inner.db_mut(self.selected_db);
        db.string_set(key, value.as_bytes().to_vec(), now);
        db.ttl.remove(key);
    }

    /// Increment a string key by delta. Creates the key if it doesn't exist.
    pub fn incr(&self, key: &str, delta: i64) -> i64 {
        let mut inner = self.state.lock();
        let now = inner.effective_now();
        let db = inner.db_mut(self.selected_db);
        let current = db
            .string_get(key)
            .and_then(|v| String::from_utf8_lossy(v).parse::<i64>().ok())
            .unwrap_or(0);
        let new_val = current + delta;
        db.string_set(key, new_val.to_string().into_bytes(), now);
        new_val
    }

    // ── List operations ──────────────────────────────────────────────

    /// Push values to the end (right) of a list. Creates the key if needed.
    /// Returns the new list length.
    pub fn push(&self, key: &str, values: &[&str]) -> usize {
        let mut inner = self.state.lock();

        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::List);
        let list = db.list_keys.entry(key.to_owned()).or_default();
        for v in values {
            list.push_back(v.as_bytes().to_vec());
        }
        list.len()
    }

    /// Push a value to the beginning (left) of a list.
    pub fn lpush(&self, key: &str, value: &str) -> usize {
        let mut inner = self.state.lock();

        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::List);
        let list = db.list_keys.entry(key.to_owned()).or_default();
        list.push_front(value.as_bytes().to_vec());
        list.len()
    }

    /// Pop from the end (right) of a list.
    pub fn pop(&self, key: &str) -> Option<String> {
        let mut inner = self.state.lock();
        let db = inner.db_mut(self.selected_db);
        let list = db.list_keys.get_mut(key)?;
        let val = list.pop_back()?;
        if list.is_empty() {
            db.list_keys.remove(key);
            db.del(key);
        }
        Some(String::from_utf8_lossy(&val).into_owned())
    }

    /// Pop from the beginning (left) of a list.
    pub fn lpop(&self, key: &str) -> Option<String> {
        let mut inner = self.state.lock();
        let db = inner.db_mut(self.selected_db);
        let list = db.list_keys.get_mut(key)?;
        let val = list.pop_front()?;
        if list.is_empty() {
            db.list_keys.remove(key);
            db.del(key);
        }
        Some(String::from_utf8_lossy(&val).into_owned())
    }

    /// Get all values in a list.
    pub fn list(&self, key: &str) -> Option<Vec<String>> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.list_keys.get(key).map(|list| {
            list.iter()
                .map(|v| String::from_utf8_lossy(v).into_owned())
                .collect()
        })
    }

    // ── Set operations ───────────────────────────────────────────────

    /// Add members to a set. Returns the number of new members added.
    pub fn set_add(&self, key: &str, members: &[&str]) -> usize {
        let mut inner = self.state.lock();

        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::Set);
        let set = db.set_keys.entry(key.to_owned()).or_default();
        let mut added = 0;
        for m in members {
            if set.insert(m.to_string()) {
                added += 1;
            }
        }
        added
    }

    /// Get all members of a set, sorted.
    pub fn members(&self, key: &str) -> Option<Vec<String>> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.set_keys.get(key).map(|set| {
            let mut v: Vec<String> = set.iter().cloned().collect();
            v.sort();
            v
        })
    }

    /// Check if a value is a member of a set.
    pub fn is_member(&self, key: &str, member: &str) -> bool {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.set_keys
            .get(key)
            .map(|set| set.contains(member))
            .unwrap_or(false)
    }

    // ── Hash operations ──────────────────────────────────────────────

    /// Set a hash field.
    pub fn hset(&self, key: &str, field: &str, value: &str) {
        let mut inner = self.state.lock();

        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::Hash);
        let hash = db.hash_keys.entry(key.to_owned()).or_default();
        hash.insert(field.to_owned(), value.as_bytes().to_vec());
    }

    /// Get a hash field value.
    pub fn hget(&self, key: &str, field: &str) -> Option<String> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.hash_keys
            .get(key)
            .and_then(|h| h.get(field))
            .map(|v| String::from_utf8_lossy(v).into_owned())
    }

    /// Get all field names in a hash, sorted.
    pub fn hkeys(&self, key: &str) -> Option<Vec<String>> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.hash_keys.get(key).map(|h| {
            let mut keys: Vec<String> = h.keys().cloned().collect();
            keys.sort();
            keys
        })
    }

    /// Delete a hash field. Returns true if the field existed.
    pub fn hdel(&self, key: &str, field: &str) -> bool {
        let mut inner = self.state.lock();
        let db = inner.db_mut(self.selected_db);
        if let Some(hash) = db.hash_keys.get_mut(key) {
            let removed = hash.remove(field).is_some();
            if hash.is_empty() {
                db.hash_keys.remove(key);
                db.del(key);
            }
            removed
        } else {
            false
        }
    }

    // ── Sorted set operations ────────────────────────────────────────

    /// Add a member to a sorted set with the given score.
    /// Returns true if the member was new.
    pub fn zadd(&self, key: &str, score: f64, member: &str) -> bool {
        let mut inner = self.state.lock();

        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::SortedSet);
        let ss = db.sorted_set_keys.entry(key.to_owned()).or_default();
        ss.set(score, member)
    }

    /// Get the score of a member in a sorted set.
    pub fn zscore(&self, key: &str, member: &str) -> Option<f64> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.sorted_set_keys.get(key).and_then(|ss| ss.get(member))
    }

    /// Get all members of a sorted set, sorted by score then member name.
    pub fn zmembers(&self, key: &str) -> Option<Vec<String>> {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.sorted_set_keys.get(key).map(|ss| ss.members_sorted())
    }

    // ── Stream operations ────────────────────────────────────────────

    /// Add an entry to a stream. Returns the assigned ID.
    pub fn xadd(&self, key: &str, id: &str, values: &[(&str, &str)]) -> String {
        let mut inner = self.state.lock();
        let now = inner.effective_now();
        let now_ms = now
            .duration_since(SystemTime::UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as u64;
        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::Stream);
        let stream = db.stream_keys.entry(key.to_owned()).or_default();
        let field_values: Vec<String> = values
            .iter()
            .flat_map(|(k, v)| vec![k.to_string(), v.to_string()])
            .collect();
        stream.add(id, field_values, now_ms).unwrap_or_default()
    }

    // ── HyperLogLog operations ───────────────────────────────────────

    /// Add elements to a HyperLogLog. Returns true if the cardinality estimate changed.
    pub fn pfadd(&self, key: &str, elements: &[&str]) -> bool {
        let mut inner = self.state.lock();

        let db = inner.db_mut(self.selected_db);
        db.keys.insert(key.to_owned(), types::KeyType::HyperLogLog);
        let hll = db.hll_keys.entry(key.to_owned()).or_default();
        let mut changed = false;
        for elem in elements {
            if hll.add(elem.as_bytes()) {
                changed = true;
            }
        }
        changed
    }

    /// Get the cardinality estimate of a HyperLogLog.
    pub fn pfcount(&self, key: &str) -> i64 {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        db.hll_keys
            .get(key)
            .map(|hll| hll.count() as i64)
            .unwrap_or(0)
    }

    // ── Flush ────────────────────────────────────────────────────────

    /// Remove all keys from the selected database.
    pub fn flush_db(&self) {
        let mut inner = self.state.lock();
        inner.db_mut(self.selected_db).flush();
    }

    /// Remove all keys from all databases.
    pub fn flush_all(&self) {
        let mut inner = self.state.lock();
        for db in &mut inner.dbs {
            db.flush();
        }
    }

    // ── Testing assertions ───────────────────────────────────────────

    /// Assert that a string key has the expected value. Panics on mismatch.
    pub fn check_get(&self, key: &str, expected: &str) {
        let val = self
            .get(key)
            .unwrap_or_else(|| panic!("key {:?} not found", key));
        assert_eq!(
            val, expected,
            "key {:?}: expected {:?}, got {:?}",
            key, expected, val
        );
    }

    // ── Pub/Sub ──────────────────────────────────────────────────────

    /// Publish a message on a channel. Returns the number of subscribers that received it.
    pub fn publish(&self, channel: &str, message: &str) -> i64 {
        let registry = self.state.pubsub.lock().unwrap();
        registry.publish(channel, message)
    }

    /// Return active pub/sub channels, optionally filtered by glob pattern.
    pub fn pubsub_channels(&self, pattern: Option<&str>) -> Vec<String> {
        let registry = self.state.pubsub.lock().unwrap();
        registry.active_channels(pattern)
    }

    /// Return the number of subscribers for specific channels.
    pub fn pubsub_numsub(&self, channels: &[&str]) -> Vec<(String, i64)> {
        let registry = self.state.pubsub.lock().unwrap();
        channels
            .iter()
            .map(|ch| (ch.to_string(), registry.numsub(ch)))
            .collect()
    }

    /// Return the total number of pattern subscriptions.
    pub fn pubsub_numpat(&self) -> i64 {
        let registry = self.state.pubsub.lock().unwrap();
        registry.numpat()
    }

    // ── Testing assertions ───────────────────────────────────────────

    /// Assert that a list key has the expected values. Panics on mismatch.
    pub fn check_list(&self, key: &str, expected: &[&str]) {
        let val = self
            .list(key)
            .unwrap_or_else(|| panic!("key {:?} not found or not a list", key));
        let expected_strs: Vec<String> = expected.iter().map(|s| s.to_string()).collect();
        assert_eq!(
            val, expected_strs,
            "key {:?}: expected {:?}, got {:?}",
            key, expected, val
        );
    }

    /// Assert that a set key has the expected members (order-independent). Panics on mismatch.
    pub fn check_set(&self, key: &str, expected: &[&str]) {
        let val = self
            .members(key)
            .unwrap_or_else(|| panic!("key {:?} not found or not a set", key));
        let mut expected_sorted: Vec<String> = expected.iter().map(|s| s.to_string()).collect();
        expected_sorted.sort();
        assert_eq!(
            val, expected_sorted,
            "key {:?}: expected {:?}, got {:?}",
            key, expected_sorted, val
        );
    }

    // ── Server introspection ─────────────────────────────────────────

    /// Number of currently connected clients.
    pub fn current_connection_count(&self) -> u64 {
        self.state
            .connected_clients
            .load(std::sync::atomic::Ordering::Relaxed)
    }

    /// Total number of connections received since startup.
    pub fn total_connection_count(&self) -> u64 {
        self.state
            .total_connections_received
            .load(std::sync::atomic::Ordering::Relaxed)
    }

    // ── Direct DB access ──────────────────────────────────────────────

    /// Access a specific database by index (0-15) without changing the
    /// selected database.
    ///
    /// The returned [`DbRef`] borrows `self` and provides the same
    /// direct-access methods (get, set, keys, etc.) scoped to the given DB.
    pub fn db(&self, id: usize) -> DbRef<'_> {
        assert!(id < 16, "database index must be 0-15");
        DbRef {
            state: &self.state,
            db_id: id,
        }
    }

    // ── Restart ─────────────────────────────────────────────────────

    /// Restart a closed server on a new port. All data is preserved.
    /// The previous server must have been closed with [`close()`](Self::close).
    pub async fn restart(&mut self) -> Result<()> {
        let listener = TcpListener::bind("127.0.0.1:0").await?;
        self.addr = listener.local_addr()?;
        let state_clone = Arc::clone(&self.state);
        let shutdown_rx = self.state.shutdown_tx.subscribe();

        tokio::spawn(async move {
            server::run(listener, state_clone, shutdown_rx, None).await;
        });

        Ok(())
    }

    // ── Dump ────────────────────────────────────────────────────────

    /// Return a text representation of the selected database, useful for
    /// debugging.
    pub fn dump(&self) -> String {
        let inner = self.state.lock();
        let db = inner.db(self.selected_db);
        dump_db(db)
    }

    // ── Internals (for advanced usage) ───────────────────────────────

    /// Get a reference to the shared state (for custom commands, etc.).
    pub fn shared_state(&self) -> &Arc<SharedState> {
        &self.state
    }

    /// Number of keys in the selected database.
    pub fn db_size(&self) -> usize {
        let inner = self.state.lock();
        inner.db(self.selected_db).keys.len()
    }
}

/// A handle to a specific database, returned by [`Miniredis::db()`].
/// Provides the same direct-access methods scoped to the given DB index.
pub struct DbRef<'a> {
    state: &'a Arc<SharedState>,
    db_id: usize,
}

impl DbRef<'_> {
    /// Return all keys, sorted.
    pub fn keys(&self) -> Vec<String> {
        let inner = self.state.lock();
        inner.db(self.db_id).all_keys()
    }

    /// Get a string key value.
    pub fn get(&self, key: &str) -> Option<String> {
        let inner = self.state.lock();
        inner
            .db(self.db_id)
            .string_get(key)
            .map(|v| String::from_utf8_lossy(v).into_owned())
    }

    /// Set a string key. Removes any existing TTL.
    pub fn set(&self, key: &str, value: &str) {
        let mut inner = self.state.lock();
        let now = inner.effective_now();
        let db = inner.db_mut(self.db_id);
        db.string_set(key, value.as_bytes().to_vec(), now);
        db.ttl.remove(key);
    }

    /// Check if a key exists.
    pub fn exists(&self, key: &str) -> bool {
        let mut inner = self.state.lock();
        let now = inner.effective_now();
        inner.db_mut(self.db_id).exists(key, now)
    }

    /// Return the type of a key.
    pub fn key_type(&self, key: &str) -> &'static str {
        let inner = self.state.lock();
        match inner.db(self.db_id).key_type(key) {
            Some(t) => t.as_str(),
            None => "none",
        }
    }

    /// Number of keys in this database.
    pub fn db_size(&self) -> usize {
        let inner = self.state.lock();
        inner.db(self.db_id).keys.len()
    }

    /// Return a text representation of this database.
    pub fn dump(&self) -> String {
        let inner = self.state.lock();
        dump_db(inner.db(self.db_id))
    }
}

/// Maximum number of characters to show per value in [`Miniredis::dump()`].
const DUMP_MAX_LINE_LEN: usize = 200;

fn dump_db(db: &db::RedisDB) -> String {
    use std::fmt::Write;
    use types::Direction;

    let indent = "   ";
    let mut r = String::new();

    let truncate = |s: &str| -> String {
        if s.len() > DUMP_MAX_LINE_LEN {
            let suffix = format!("...({})", s.len());
            let end = DUMP_MAX_LINE_LEN - suffix.len();
            format!("{:?}{}", &s[..end], suffix)
        } else {
            format!("{:?}", s)
        }
    };

    for k in db.all_keys() {
        let _ = writeln!(r, "- {}", k);
        match db.key_type(&k) {
            Some(types::KeyType::String) => {
                if let Some(v) = db.string_get(&k) {
                    let _ = writeln!(r, "{}{}", indent, truncate(&String::from_utf8_lossy(v)));
                }
            }
            Some(types::KeyType::Hash) => {
                if let Some(hash) = db.hash_keys.get(&k) {
                    let mut fields: Vec<&String> = hash.keys().collect();
                    fields.sort();
                    for f in fields {
                        let v = String::from_utf8_lossy(&hash[f]);
                        let _ = writeln!(r, "{}{}: {}", indent, f, truncate(&v));
                    }
                }
            }
            Some(types::KeyType::List) => {
                if let Some(list) = db.list_keys.get(&k) {
                    for item in list {
                        let _ =
                            writeln!(r, "{}{}", indent, truncate(&String::from_utf8_lossy(item)));
                    }
                }
            }
            Some(types::KeyType::Set) => {
                if let Some(set) = db.set_keys.get(&k) {
                    let mut members: Vec<&String> = set.iter().collect();
                    members.sort();
                    for m in members {
                        let _ = writeln!(r, "{}{}", indent, truncate(m));
                    }
                }
            }
            Some(types::KeyType::SortedSet) => {
                if let Some(ss) = db.sorted_set_keys.get(&k) {
                    for el in ss.by_score(Direction::Asc) {
                        let _ = writeln!(r, "{}{}: {}", indent, el.score, truncate(&el.member));
                    }
                }
            }
            Some(types::KeyType::Stream) => {
                if let Some(stream) = db.stream_keys.get(&k) {
                    for entry in &stream.entries {
                        let _ = writeln!(r, "{}{}", indent, entry.id);
                        let ev = &entry.values;
                        let mut i = 0;
                        while i + 1 < ev.len() {
                            let _ = writeln!(
                                r,
                                "{}{}{}: {}",
                                indent,
                                indent,
                                truncate(&ev[i]),
                                truncate(&ev[i + 1])
                            );
                            i += 2;
                        }
                    }
                }
            }
            Some(types::KeyType::HyperLogLog) => {
                let _ = writeln!(r, "{}(HyperLogLog)", indent);
            }
            None => {}
        }
    }
    r
}
