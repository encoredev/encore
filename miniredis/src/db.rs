use std::collections::{HashMap, HashSet, VecDeque};
use std::sync::Arc;
use std::sync::atomic::AtomicU64;
use std::time::{Duration, SystemTime};

use rand::SeedableRng;
use rand::rngs::StdRng;
use tokio::sync::{Notify, broadcast};

use crate::hll::HyperLogLog;
use crate::types::{KeyType, SortedSet, Stream};

/// A single numbered Redis database (0-15).
#[derive(Debug)]
pub struct RedisDB {
    /// Master map: key name -> type tag.
    pub keys: HashMap<String, KeyType>,
    /// String values.
    pub string_keys: HashMap<String, Vec<u8>>,
    /// Hash values: key -> (field -> value).
    pub hash_keys: HashMap<String, HashMap<String, Vec<u8>>>,
    /// List values.
    pub list_keys: HashMap<String, VecDeque<Vec<u8>>>,
    /// Set values.
    pub set_keys: HashMap<String, HashSet<String>>,
    /// Sorted set values.
    pub sorted_set_keys: HashMap<String, SortedSet>,
    /// Stream values.
    pub stream_keys: HashMap<String, Stream>,
    /// HyperLogLog values.
    pub hll_keys: HashMap<String, HyperLogLog>,
    /// Key TTLs (remaining duration).
    pub ttl: HashMap<String, Duration>,
    /// Hash field TTLs: key -> (field -> remaining duration).
    pub hash_field_ttls: HashMap<String, HashMap<String, Duration>>,
    /// Key versions (bumped on every mutation, used by WATCH).
    pub key_version: HashMap<String, u64>,
    /// Last-recently-used timestamps.
    pub lru: HashMap<String, SystemTime>,
}

impl Default for RedisDB {
    fn default() -> Self {
        Self::new()
    }
}

impl RedisDB {
    pub fn new() -> Self {
        RedisDB {
            keys: HashMap::new(),
            string_keys: HashMap::new(),
            hash_keys: HashMap::new(),
            list_keys: HashMap::new(),
            set_keys: HashMap::new(),
            sorted_set_keys: HashMap::new(),
            stream_keys: HashMap::new(),
            hll_keys: HashMap::new(),
            ttl: HashMap::new(),
            hash_field_ttls: HashMap::new(),
            key_version: HashMap::new(),
            lru: HashMap::new(),
        }
    }

    /// Check if a key exists (also updates LRU).
    pub fn exists(&mut self, key: &str, now: SystemTime) -> bool {
        if self.keys.contains_key(key) {
            self.lru.insert(key.to_owned(), now);
            true
        } else {
            false
        }
    }

    /// Get the type of a key, or None.
    pub fn key_type(&self, key: &str) -> Option<KeyType> {
        self.keys.get(key).copied()
    }

    /// Increment the key version and update LRU.
    pub fn incr_version(&mut self, key: &str, now: SystemTime) {
        self.lru.insert(key.to_owned(), now);
        let v = self.key_version.entry(key.to_owned()).or_insert(0);
        *v += 1;
    }

    /// Delete a key and its data. Returns true if the key existed.
    pub fn del(&mut self, key: &str) -> bool {
        let key_type = match self.keys.remove(key) {
            Some(t) => t,
            None => return false,
        };

        self.lru.remove(key);
        self.ttl.remove(key);
        self.hash_field_ttls.remove(key);
        let v = self.key_version.entry(key.to_owned()).or_insert(0);
        *v += 1;

        match key_type {
            KeyType::String => {
                self.string_keys.remove(key);
            }
            KeyType::Hash => {
                self.hash_keys.remove(key);
            }
            KeyType::List => {
                self.list_keys.remove(key);
            }
            KeyType::Set => {
                self.set_keys.remove(key);
            }
            KeyType::SortedSet => {
                self.sorted_set_keys.remove(key);
            }
            KeyType::Stream => {
                self.stream_keys.remove(key);
            }
            KeyType::HyperLogLog => {
                self.hll_keys.remove(key);
            }
        }

        true
    }

    /// Delete a key without removing its TTL (used by string_set etc.).
    pub fn del_keep_ttl(&mut self, key: &str) {
        let key_type = match self.keys.remove(key) {
            Some(t) => t,
            None => return,
        };

        match key_type {
            KeyType::String => {
                self.string_keys.remove(key);
            }
            KeyType::Hash => {
                self.hash_keys.remove(key);
            }
            KeyType::List => {
                self.list_keys.remove(key);
            }
            KeyType::Set => {
                self.set_keys.remove(key);
            }
            KeyType::SortedSet => {
                self.sorted_set_keys.remove(key);
            }
            KeyType::Stream => {
                self.stream_keys.remove(key);
            }
            KeyType::HyperLogLog => {
                self.hll_keys.remove(key);
            }
        }
    }

    /// GET: returns the value of a string key, or None.
    pub fn string_get(&self, key: &str) -> Option<&Vec<u8>> {
        if self.keys.get(key) != Some(&KeyType::String) {
            return None;
        }
        self.string_keys.get(key)
    }

    /// SET: force-set a string key. Does NOT remove TTL.
    pub fn string_set(&mut self, key: &str, value: Vec<u8>, now: SystemTime) {
        self.del_keep_ttl(key);
        self.keys.insert(key.to_owned(), KeyType::String);
        self.string_keys.insert(key.to_owned(), value);
        self.incr_version(key, now);
    }

    // ── Hash helpers ──────────────────────────────────────────────────

    /// Set hash fields. Returns the number of NEW fields added.
    pub fn hash_set(&mut self, key: &str, pairs: &[(String, Vec<u8>)], now: SystemTime) -> i64 {
        self.keys.entry(key.to_owned()).or_insert(KeyType::Hash);
        let hash = self.hash_keys.entry(key.to_owned()).or_default();
        let mut new_count = 0i64;
        for (field, value) in pairs {
            if !hash.contains_key(field) {
                new_count += 1;
            }
            hash.insert(field.clone(), value.clone());
        }
        self.incr_version(key, now);
        new_count
    }

    /// Get a hash field value.
    pub fn hash_get(&self, key: &str, field: &str) -> Option<&Vec<u8>> {
        self.hash_keys.get(key)?.get(field)
    }

    /// Delete hash fields. Returns the number deleted. Removes key if hash becomes empty.
    pub fn hash_del(&mut self, key: &str, fields: &[String], now: SystemTime) -> i64 {
        let hash = match self.hash_keys.get_mut(key) {
            Some(h) => h,
            None => return 0,
        };
        let mut count = 0i64;
        for field in fields {
            if hash.remove(field).is_some() {
                count += 1;
            }
        }
        if hash.is_empty() {
            self.del(key);
        } else {
            self.incr_version(key, now);
        }
        count
    }

    /// Get all hash field names, sorted.
    pub fn hash_fields(&self, key: &str) -> Vec<String> {
        match self.hash_keys.get(key) {
            Some(h) => {
                let mut fields: Vec<String> = h.keys().cloned().collect();
                fields.sort();
                fields
            }
            None => Vec::new(),
        }
    }

    /// Get all hash values in field-sorted order.
    pub fn hash_values(&self, key: &str) -> Vec<Vec<u8>> {
        let fields = self.hash_fields(key);
        let hash = match self.hash_keys.get(key) {
            Some(h) => h,
            None => return Vec::new(),
        };
        fields.iter().filter_map(|f| hash.get(f).cloned()).collect()
    }

    // ── List helpers ─────────────────────────────────────────────────

    /// LPUSH: prepend value(s) to a list. Returns new length.
    pub fn list_lpush(&mut self, key: &str, values: &[Vec<u8>], now: SystemTime) -> i64 {
        self.keys.entry(key.to_owned()).or_insert(KeyType::List);
        let list = self.list_keys.entry(key.to_owned()).or_default();
        for v in values {
            list.push_front(v.clone());
        }
        let len = list.len() as i64;
        self.incr_version(key, now);
        len
    }

    /// RPUSH: append value(s) to a list. Returns new length.
    pub fn list_rpush(&mut self, key: &str, values: &[Vec<u8>], now: SystemTime) -> i64 {
        self.keys.entry(key.to_owned()).or_insert(KeyType::List);
        let list = self.list_keys.entry(key.to_owned()).or_default();
        for v in values {
            list.push_back(v.clone());
        }
        let len = list.len() as i64;
        self.incr_version(key, now);
        len
    }

    /// LPOP: remove and return the first element.
    pub fn list_lpop(&mut self, key: &str, now: SystemTime) -> Option<Vec<u8>> {
        let list = self.list_keys.get_mut(key)?;
        let val = list.pop_front()?;
        if list.is_empty() {
            self.del(key);
        } else {
            self.incr_version(key, now);
        }
        Some(val)
    }

    /// RPOP: remove and return the last element.
    pub fn list_rpop(&mut self, key: &str, now: SystemTime) -> Option<Vec<u8>> {
        let list = self.list_keys.get_mut(key)?;
        let val = list.pop_back()?;
        if list.is_empty() {
            self.del(key);
        } else {
            self.incr_version(key, now);
        }
        Some(val)
    }

    // ── Set helpers ──────────────────────────────────────────────────

    /// SADD: add members to a set. Returns count of new members added.
    pub fn set_add(&mut self, key: &str, members: &[String], now: SystemTime) -> i64 {
        self.keys.entry(key.to_owned()).or_insert(KeyType::Set);
        let set = self.set_keys.entry(key.to_owned()).or_default();
        let mut added = 0i64;
        for m in members {
            if set.insert(m.clone()) {
                added += 1;
            }
        }
        self.incr_version(key, now);
        added
    }

    /// SREM: remove members from a set. Returns count removed.
    pub fn set_rem(&mut self, key: &str, members: &[String], now: SystemTime) -> i64 {
        let set = match self.set_keys.get_mut(key) {
            Some(s) => s,
            None => return 0,
        };
        let mut removed = 0i64;
        for m in members {
            if set.remove(m) {
                removed += 1;
            }
        }
        if set.is_empty() {
            self.del(key);
        } else {
            self.incr_version(key, now);
        }
        removed
    }

    /// Get all members of a set, sorted.
    pub fn set_members(&self, key: &str) -> Vec<String> {
        match self.set_keys.get(key) {
            Some(s) => {
                let mut members: Vec<String> = s.iter().cloned().collect();
                members.sort();
                members
            }
            None => Vec::new(),
        }
    }

    /// Check if a member is in a set.
    pub fn set_is_member(&self, key: &str, member: &str) -> bool {
        self.set_keys
            .get(key)
            .map(|s| s.contains(member))
            .unwrap_or(false)
    }

    /// Replace a set entirely (used by set operations like SDIFFSTORE).
    pub fn set_set(&mut self, key: &str, members: HashSet<String>, now: SystemTime) {
        if members.is_empty() {
            return;
        }
        self.del(key);
        self.keys.insert(key.to_owned(), KeyType::Set);
        self.set_keys.insert(key.to_owned(), members);
        self.incr_version(key, now);
    }

    // ── Sorted set helpers ───────────────────────────────────────────

    /// ZADD: add a member with score. Returns true if the member was new.
    pub fn sset_add(&mut self, key: &str, score: f64, member: &str, now: SystemTime) -> bool {
        self.keys
            .entry(key.to_owned())
            .or_insert(KeyType::SortedSet);
        let ss = self.sorted_set_keys.entry(key.to_owned()).or_default();
        let is_new = ss.set(score, member);
        self.incr_version(key, now);
        is_new
    }

    /// Check if a member exists in a sorted set.
    pub fn sset_exists(&self, key: &str, member: &str) -> bool {
        self.sorted_set_keys
            .get(key)
            .map(|ss| ss.exists(member))
            .unwrap_or(false)
    }

    /// Get a member's score.
    pub fn sset_score(&self, key: &str, member: &str) -> Option<f64> {
        self.sorted_set_keys.get(key)?.get(member)
    }

    /// Get cardinality of sorted set.
    pub fn sset_card(&self, key: &str) -> usize {
        self.sorted_set_keys
            .get(key)
            .map(|ss| ss.card())
            .unwrap_or(0)
    }

    /// ZINCRBY: increment member's score. Returns new score.
    pub fn sset_incrby(&mut self, key: &str, member: &str, delta: f64, now: SystemTime) -> f64 {
        self.keys
            .entry(key.to_owned())
            .or_insert(KeyType::SortedSet);
        let ss = self.sorted_set_keys.entry(key.to_owned()).or_default();
        let new_score = ss.incrby(member, delta);
        self.incr_version(key, now);
        new_score
    }

    /// Remove a member from a sorted set. Returns true if it existed.
    pub fn sset_rem(&mut self, key: &str, member: &str, now: SystemTime) -> bool {
        let ss = match self.sorted_set_keys.get_mut(key) {
            Some(ss) => ss,
            None => return false,
        };
        let removed = ss.remove(member);
        if ss.card() == 0 {
            self.del(key);
        } else {
            self.incr_version(key, now);
        }
        removed
    }

    /// Replace a sorted set entirely.
    pub fn sset_set(&mut self, key: &str, ss: SortedSet, now: SystemTime) {
        if ss.card() == 0 {
            self.del(key);
            return;
        }
        self.del(key);
        self.keys.insert(key.to_owned(), KeyType::SortedSet);
        self.sorted_set_keys.insert(key.to_owned(), ss);
        self.incr_version(key, now);
    }

    // ── HyperLogLog helpers ────────────────────────────────────────

    /// PFADD: add items to a HyperLogLog. Returns 1 if any register changed, 0 otherwise.
    pub fn hll_add(&mut self, key: &str, items: &[&str], now: SystemTime) -> i64 {
        self.keys
            .entry(key.to_owned())
            .or_insert(KeyType::HyperLogLog);
        let hll = self.hll_keys.entry(key.to_owned()).or_default();
        let mut changed = false;
        for item in items {
            if hll.add(item.as_bytes()) {
                changed = true;
            }
        }
        self.incr_version(key, now);
        if changed { 1 } else { 0 }
    }

    /// PFCOUNT: count across one or more HLL keys. Returns error if any key is wrong type.
    pub fn hll_count(&self, keys: &[&str]) -> Result<i64, &'static str> {
        if keys.len() == 1 {
            let key = keys[0];
            if let Some(kt) = self.keys.get(key)
                && *kt != KeyType::HyperLogLog
            {
                return Err("WRONGTYPE Key is not a valid HyperLogLog string value.");
            }
            match self.hll_keys.get(key) {
                Some(hll) => Ok(hll.count() as i64),
                None => Ok(0),
            }
        } else {
            // Multiple keys: merge into temporary HLL and count
            let mut merged = HyperLogLog::new();
            for &key in keys {
                if let Some(kt) = self.keys.get(key)
                    && *kt != KeyType::HyperLogLog
                {
                    return Err("WRONGTYPE Key is not a valid HyperLogLog string value.");
                }
                if let Some(hll) = self.hll_keys.get(key) {
                    merged.merge(hll);
                }
            }
            Ok(merged.count() as i64)
        }
    }

    /// PFMERGE: merge source HLLs into dest. keys[0] is dest, rest are sources.
    /// Returns error if any key is wrong type.
    pub fn hll_merge(&mut self, keys: &[&str], now: SystemTime) -> Result<(), &'static str> {
        // Validate all keys first
        for &key in keys {
            if let Some(kt) = self.keys.get(key)
                && *kt != KeyType::HyperLogLog
            {
                return Err("WRONGTYPE Key is not a valid HyperLogLog string value.");
            }
        }

        let dest = keys[0];

        // Collect source HLLs into a merged result
        let mut merged = self.hll_keys.get(dest).cloned().unwrap_or_default();

        for &key in &keys[1..] {
            if let Some(hll) = self.hll_keys.get(key) {
                merged.merge(hll);
            }
        }

        // Store the result
        self.keys.insert(dest.to_owned(), KeyType::HyperLogLog);
        self.hll_keys.insert(dest.to_owned(), merged);
        self.incr_version(dest, now);
        Ok(())
    }

    // ── Key rename helper ────────────────────────────────────────────

    /// Rename a key. Returns false if source doesn't exist.
    pub fn rename(&mut self, from: &str, to: &str, now: SystemTime) -> bool {
        let key_type = match self.keys.remove(from) {
            Some(t) => t,
            None => return false,
        };

        // Remove destination if it exists
        self.del(to);

        // Move the type tag
        self.keys.insert(to.to_owned(), key_type);

        // Move the actual data
        match key_type {
            KeyType::String => {
                if let Some(v) = self.string_keys.remove(from) {
                    self.string_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::Hash => {
                if let Some(v) = self.hash_keys.remove(from) {
                    self.hash_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::List => {
                if let Some(v) = self.list_keys.remove(from) {
                    self.list_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::Set => {
                if let Some(v) = self.set_keys.remove(from) {
                    self.set_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::SortedSet => {
                if let Some(v) = self.sorted_set_keys.remove(from) {
                    self.sorted_set_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::Stream => {
                if let Some(v) = self.stream_keys.remove(from) {
                    self.stream_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::HyperLogLog => {
                if let Some(v) = self.hll_keys.remove(from) {
                    self.hll_keys.insert(to.to_owned(), v);
                }
            }
        }

        // Move TTL
        if let Some(ttl) = self.ttl.remove(from) {
            self.ttl.insert(to.to_owned(), ttl);
        }

        // Move hash field TTLs
        if let Some(field_ttls) = self.hash_field_ttls.remove(from) {
            self.hash_field_ttls.insert(to.to_owned(), field_ttls);
        }

        // Move LRU
        if let Some(lru) = self.lru.remove(from) {
            self.lru.insert(to.to_owned(), lru);
        }

        // Update versions
        self.incr_version(from, now);
        self.incr_version(to, now);

        true
    }

    /// Check and delete a key if its TTL has expired.
    pub fn check_ttl(&mut self, key: &str) -> bool {
        if let Some(&ttl) = self.ttl.get(key)
            && ttl <= Duration::ZERO
        {
            self.del(key);
            return true; // key was expired
        }
        false // key still alive or has no TTL
    }

    /// Remove all keys and values.
    pub fn flush(&mut self) {
        self.keys.clear();
        self.string_keys.clear();
        self.hash_keys.clear();
        self.list_keys.clear();
        self.set_keys.clear();
        self.sorted_set_keys.clear();
        self.stream_keys.clear();
        self.hll_keys.clear();
        self.ttl.clear();
        self.hash_field_ttls.clear();
        self.key_version.clear();
        self.lru.clear();
    }

    /// Deep-copy a key's data (type, value, TTL) within the same DB. Returns true on success.
    pub fn copy_key(&mut self, from: &str, to: &str, now: SystemTime) -> bool {
        let key_type = match self.keys.get(from) {
            Some(t) => *t,
            None => return false,
        };

        self.keys.insert(to.to_owned(), key_type);

        match key_type {
            KeyType::String => {
                if let Some(v) = self.string_keys.get(from).cloned() {
                    self.string_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::Hash => {
                if let Some(v) = self.hash_keys.get(from).cloned() {
                    self.hash_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::List => {
                if let Some(v) = self.list_keys.get(from).cloned() {
                    self.list_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::Set => {
                if let Some(v) = self.set_keys.get(from).cloned() {
                    self.set_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::SortedSet => {
                if let Some(v) = self.sorted_set_keys.get(from).cloned() {
                    self.sorted_set_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::Stream => {
                if let Some(v) = self.stream_keys.get(from).cloned() {
                    self.stream_keys.insert(to.to_owned(), v);
                }
            }
            KeyType::HyperLogLog => {
                if let Some(v) = self.hll_keys.get(from).cloned() {
                    self.hll_keys.insert(to.to_owned(), v);
                }
            }
        }

        // Copy TTL
        if let Some(ttl) = self.ttl.get(from).copied() {
            self.ttl.insert(to.to_owned(), ttl);
        }

        // Copy hash field TTLs
        if let Some(field_ttls) = self.hash_field_ttls.get(from).cloned() {
            self.hash_field_ttls.insert(to.to_owned(), field_ttls);
        }

        self.incr_version(to, now);
        true
    }

    /// Return all keys, sorted.
    pub fn all_keys(&self) -> Vec<String> {
        let mut keys: Vec<String> = self.keys.keys().cloned().collect();
        keys.sort();
        keys
    }

    /// Decrease all TTLs by `duration`, deleting expired keys.
    pub fn fast_forward(&mut self, duration: Duration) {
        let keys: Vec<String> = self.ttl.keys().cloned().collect();
        for key in keys {
            if let Some(ttl) = self.ttl.get_mut(&key) {
                *ttl = ttl.saturating_sub(duration);
            }
            self.check_ttl(&key);
        }

        // Handle hash field TTLs
        let hash_keys: Vec<String> = self.hash_field_ttls.keys().cloned().collect();
        for key in hash_keys {
            self.check_hash_field_ttls(&key, duration);
        }
    }

    /// Check and expire hash field TTLs. Removes expired fields, and if
    /// the hash becomes empty, deletes the key entirely.
    pub fn check_hash_field_ttls(&mut self, key: &str, duration: Duration) {
        let field_ttls = match self.hash_field_ttls.get_mut(key) {
            Some(t) => t,
            None => return,
        };

        let mut expired_fields = Vec::new();
        for (field, ttl) in field_ttls.iter_mut() {
            *ttl = ttl.saturating_sub(duration);
            if *ttl <= Duration::ZERO {
                expired_fields.push(field.clone());
            }
        }

        for field in &expired_fields {
            field_ttls.remove(field);
            if let Some(hash) = self.hash_keys.get_mut(key) {
                hash.remove(field);
            }
        }

        if field_ttls.is_empty() {
            self.hash_field_ttls.remove(key);
        }

        // If hash is now empty, delete the key entirely
        if let Some(hash) = self.hash_keys.get(key)
            && hash.is_empty()
        {
            self.del(key);
        }
    }
}

/// The inner state shared across all connections.
/// Protected by a `std::sync::Mutex` (never held across .await).
#[derive(Debug)]
pub struct Inner {
    /// 16 databases (0-15).
    pub dbs: Vec<RedisDB>,
    /// Cached Lua scripts: SHA1 hex -> source.
    pub scripts: HashMap<String, String>,
    /// AUTH passwords: username -> password.
    pub passwords: HashMap<String, String>,
    /// Mock time. If None, use real time.
    pub now: Option<SystemTime>,
    /// Seeded RNG for deterministic tests.
    pub rng: StdRng,
}

impl Default for Inner {
    fn default() -> Self {
        Self::new()
    }
}

impl Inner {
    pub fn new() -> Self {
        let mut dbs = Vec::with_capacity(16);
        for _ in 0..16 {
            dbs.push(RedisDB::new());
        }
        Inner {
            dbs,
            scripts: HashMap::new(),
            passwords: HashMap::new(),
            now: None,
            rng: StdRng::from_os_rng(),
        }
    }

    /// Get the effective "now" time (mock or real).
    pub fn effective_now(&self) -> SystemTime {
        self.now.unwrap_or_else(SystemTime::now)
    }

    /// Advance mock time and expire keys in all databases.
    pub fn fast_forward(&mut self, duration: Duration) {
        if let Some(ref mut now) = self.now {
            *now += duration;
        }
        for db in &mut self.dbs {
            db.fast_forward(duration);
        }
    }

    /// Get a reference to a database.
    pub fn db(&self, idx: usize) -> &RedisDB {
        &self.dbs[idx]
    }

    /// Get a mutable reference to a database.
    pub fn db_mut(&mut self, idx: usize) -> &mut RedisDB {
        &mut self.dbs[idx]
    }
}

/// The shared state wrapper used across all connections.
/// `inner` is a std::sync::Mutex (not tokio::sync::Mutex) because we never
/// hold the lock across an .await point.
pub struct SharedState {
    /// The inner database state.
    pub inner: std::sync::Mutex<Inner>,
    /// Notifies blocking commands (BLPOP, XREAD BLOCK, etc.) when data changes.
    pub notify: Notify,
    /// Shutdown signal broadcaster.
    pub shutdown_tx: broadcast::Sender<()>,
    /// Total connections received (cumulative).
    pub total_connections_received: AtomicU64,
    /// Currently connected clients.
    pub connected_clients: AtomicU64,
    /// Total commands processed (cumulative).
    pub total_commands_processed: AtomicU64,
    /// Pub/Sub subscriber registry.
    pub pubsub: std::sync::Mutex<crate::pubsub::PubsubRegistry>,
    /// Command dispatch table (set once at server startup, used by Lua scripting).
    pub command_table: std::sync::OnceLock<Arc<crate::dispatch::CommandTable>>,
}

impl SharedState {
    pub fn new() -> Arc<Self> {
        let (shutdown_tx, _) = broadcast::channel(1);
        Arc::new(SharedState {
            inner: std::sync::Mutex::new(Inner::new()),
            notify: Notify::new(),
            shutdown_tx,
            total_connections_received: AtomicU64::new(0),
            connected_clients: AtomicU64::new(0),
            total_commands_processed: AtomicU64::new(0),
            pubsub: std::sync::Mutex::new(crate::pubsub::PubsubRegistry::new()),
            command_table: std::sync::OnceLock::new(),
        })
    }

    /// Lock the inner state.
    pub fn lock(&self) -> std::sync::MutexGuard<'_, Inner> {
        self.inner.lock().unwrap()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_redis_db_string_set_get() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("hello", b"world".to_vec(), now);
        assert_eq!(db.string_get("hello"), Some(&b"world".to_vec()));
        assert_eq!(db.key_type("hello"), Some(KeyType::String));
    }

    #[test]
    fn test_redis_db_del() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("key", b"val".to_vec(), now);
        assert!(db.exists("key", now));
        assert!(db.del("key"));
        assert!(!db.exists("key", now));
        assert_eq!(db.string_get("key"), None);
    }

    #[test]
    fn test_redis_db_del_nonexistent() {
        let mut db = RedisDB::new();
        assert!(!db.del("nope"));
    }

    #[test]
    fn test_redis_db_type_check() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("str", b"val".to_vec(), now);
        assert_eq!(db.key_type("str"), Some(KeyType::String));
        assert_eq!(db.key_type("nonexistent"), None);
    }

    #[test]
    fn test_redis_db_ttl_expiration() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("ephemeral", b"data".to_vec(), now);
        db.ttl
            .insert("ephemeral".to_owned(), Duration::from_secs(10));

        // Fast forward 5s -- key should still be alive
        db.fast_forward(Duration::from_secs(5));
        assert!(db.keys.contains_key("ephemeral"));

        // Fast forward another 6s -- key should be gone
        db.fast_forward(Duration::from_secs(6));
        assert!(!db.keys.contains_key("ephemeral"));
        assert_eq!(db.string_get("ephemeral"), None);
    }

    #[test]
    fn test_redis_db_flush() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("a", b"1".to_vec(), now);
        db.string_set("b", b"2".to_vec(), now);
        assert_eq!(db.keys.len(), 2);

        db.flush();
        assert!(db.keys.is_empty());
        assert!(db.string_keys.is_empty());
    }

    #[test]
    fn test_redis_db_all_keys_sorted() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("charlie", b"3".to_vec(), now);
        db.string_set("alpha", b"1".to_vec(), now);
        db.string_set("bravo", b"2".to_vec(), now);

        assert_eq!(db.all_keys(), vec!["alpha", "bravo", "charlie"]);
    }

    #[test]
    fn test_redis_db_key_version() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("k", b"v1".to_vec(), now);
        let v1 = db.key_version["k"];

        db.string_set("k", b"v2".to_vec(), now);
        let v2 = db.key_version["k"];

        assert!(v2 > v1);
    }

    #[test]
    fn test_redis_db_overwrite_type() {
        let mut db = RedisDB::new();
        let now = SystemTime::now();

        db.string_set("key", b"val".to_vec(), now);
        assert_eq!(db.key_type("key"), Some(KeyType::String));

        db.string_set("key", b"new_val".to_vec(), now);
        assert_eq!(db.string_get("key"), Some(&b"new_val".to_vec()));
    }

    #[test]
    fn test_inner_16_dbs() {
        let inner = Inner::new();
        assert_eq!(inner.dbs.len(), 16);
    }

    #[test]
    fn test_inner_effective_now_real() {
        let inner = Inner::new();
        let now = inner.effective_now();
        let real_now = SystemTime::now();
        let diff = real_now.duration_since(now).unwrap_or(Duration::ZERO);
        assert!(diff < Duration::from_secs(1));
    }

    #[test]
    fn test_inner_effective_now_mock() {
        let mut inner = Inner::new();
        let mock_time = SystemTime::UNIX_EPOCH + Duration::from_secs(1_000_000);
        inner.now = Some(mock_time);
        assert_eq!(inner.effective_now(), mock_time);
    }

    #[test]
    fn test_shared_state_lock() {
        let state = SharedState::new();
        {
            let mut inner = state.lock();
            inner
                .db_mut(0)
                .string_set("test", b"value".to_vec(), SystemTime::now());
        }
        {
            let inner = state.lock();
            assert_eq!(inner.db(0).string_get("test"), Some(&b"value".to_vec()));
        }
    }
}
