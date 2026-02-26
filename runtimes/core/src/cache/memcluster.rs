//! In-memory cache cluster implementation for Encore Cloud fallback.
//!
//! This module provides an in-memory implementation of the cache cluster
//! that is used when running in Encore Cloud without a configured Redis instance.

use std::collections::{HashMap, HashSet, VecDeque};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

use crate::cache::error::{Error, Result};
use crate::cache::manager::Cluster;
use crate::cache::pool::{ListDirection, Pool, TtlOp};
use crate::names::EncoreName;
use crate::trace::Tracer;

/// Maximum number of keys before cleanup is triggered.
const MAX_KEYS: usize = 100;

const TYPE_ERR_STRING: &str = "expected string";
const TYPE_ERR_LIST: &str = "expected list";
const TYPE_ERR_SET: &str = "expected set";

/// In-memory cache cluster that stores data in memory.
/// Used as a fallback when running in Encore Cloud without configured Redis.
pub struct MemoryCluster {
    store: Arc<MemoryStore>,
    tracer: Tracer,
}

impl MemoryCluster {
    pub fn new(tracer: Tracer) -> Self {
        Self {
            store: Arc::new(MemoryStore::new()),
            tracer,
        }
    }
}

impl Cluster for MemoryCluster {
    fn name(&self) -> &EncoreName {
        static NAME: std::sync::OnceLock<EncoreName> = std::sync::OnceLock::new();
        NAME.get_or_init(|| EncoreName::from("memory-cluster".to_string()))
    }

    fn pool(&self) -> anyhow::Result<Pool> {
        Ok(Pool::in_memory(self.store.clone(), self.tracer.clone()))
    }
}

/// Value types stored in the cache.
#[derive(Clone)]
enum Value {
    String(Vec<u8>),
    List(VecDeque<Vec<u8>>),
    Set(HashSet<Vec<u8>>),
}

/// Entry with expiration tracking.
struct Entry {
    value: Value,
    expires_at: Option<Instant>,
}

impl Entry {
    fn new(value: Value) -> Self {
        Self {
            value,
            expires_at: None,
        }
    }

    fn with_ttl(value: Value, ttl_ms: u64) -> Self {
        Self {
            value,
            expires_at: Some(Instant::now() + Duration::from_millis(ttl_ms)),
        }
    }

    fn is_expired(&self) -> bool {
        self.expires_at.is_some_and(|exp| Instant::now() >= exp)
    }

    fn set_ttl(&mut self, ttl_ms: u64) {
        self.expires_at = Some(Instant::now() + Duration::from_millis(ttl_ms));
    }

    fn apply_ttl_op(&mut self, ttl: Option<TtlOp>) {
        match ttl {
            None | Some(TtlOp::Keep) => {} // preserve existing TTL
            Some(TtlOp::SetMs(ms)) => self.set_ttl(ms),
            Some(TtlOp::Persist) => {
                self.expires_at = None;
            }
        }
    }

    fn new_with_ttl_op(value: Value, ttl: Option<TtlOp>) -> Self {
        match ttl {
            Some(TtlOp::SetMs(ms)) => Self::with_ttl(value, ms),
            _ => Self::new(value),
        }
    }
}

/// In-memory key-value store with TTL support.
pub struct MemoryStore {
    data: RwLock<HashMap<String, Entry>>,
    cleanup_counter: AtomicU64,
}

impl MemoryStore {
    pub(crate) fn new() -> Self {
        Self {
            data: RwLock::new(HashMap::new()),
            cleanup_counter: AtomicU64::new(0),
        }
    }

    /// Periodically clean up expired keys and enforce max keys limit.
    fn maybe_cleanup(&self) {
        let count = self.cleanup_counter.fetch_add(1, Ordering::Relaxed);
        // Clean up every 10 operations
        if !count.is_multiple_of(10) {
            return;
        }

        let mut data = self.data.write().unwrap();

        // Remove expired keys
        data.retain(|_, entry| !entry.is_expired());

        // If still over limit, remove random keys
        while data.len() > MAX_KEYS {
            if let Some(key) = data.keys().next().cloned() {
                data.remove(&key);
            }
        }
    }

    pub fn get(&self, key: &str) -> Result<Option<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();
        match data.get(key) {
            Some(entry) if !entry.is_expired() => {
                if let Value::String(v) = &entry.value {
                    Ok(Some(v.clone()))
                } else {
                    Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
                }
            }
            _ => Ok(None),
        }
    }

    pub fn set(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();
        match ttl {
            Some(TtlOp::Keep) => {
                // Preserve existing TTL: get old expiry, insert new value with same expiry
                let old_expires =
                    data.get(key)
                        .and_then(|e| if e.is_expired() { None } else { e.expires_at });
                let mut entry = Entry::new(Value::String(value.to_vec()));
                entry.expires_at = old_expires;
                data.insert(key.to_string(), entry);
            }
            _ => {
                let entry = Entry::new_with_ttl_op(Value::String(value.to_vec()), ttl);
                data.insert(key.to_string(), entry);
            }
        }
        Ok(())
    }

    pub fn set_if_not_exists(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<bool> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        // Check if key exists and is not expired
        if let Some(entry) = data.get(key) {
            if !entry.is_expired() {
                return Ok(false);
            }
        }

        let entry = Entry::new_with_ttl_op(Value::String(value.to_vec()), ttl);
        data.insert(key.to_string(), entry);
        Ok(true)
    }

    pub fn replace(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<bool> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        // Check if key exists and is not expired
        let old_expires = if let Some(entry) = data.get(key) {
            if entry.is_expired() {
                data.remove(key);
                return Ok(false);
            }
            entry.expires_at
        } else {
            return Ok(false);
        };

        match ttl {
            Some(TtlOp::Keep) => {
                let mut entry = Entry::new(Value::String(value.to_vec()));
                entry.expires_at = old_expires;
                data.insert(key.to_string(), entry);
            }
            _ => {
                let entry = Entry::new_with_ttl_op(Value::String(value.to_vec()), ttl);
                data.insert(key.to_string(), entry);
            }
        }
        Ok(true)
    }

    pub fn get_and_set(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<Option<Vec<u8>>> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let (old_value, old_expires) = match data.get(key) {
            Some(entry) if !entry.is_expired() => match &entry.value {
                Value::String(v) => (Some(v.clone()), entry.expires_at),
                _ => return Err(Error::TypeMismatch(TYPE_ERR_STRING.into())),
            },
            _ => (None, None),
        };

        match ttl {
            Some(TtlOp::Keep) => {
                let mut entry = Entry::new(Value::String(value.to_vec()));
                entry.expires_at = old_expires;
                data.insert(key.to_string(), entry);
            }
            _ => {
                let entry = Entry::new_with_ttl_op(Value::String(value.to_vec()), ttl);
                data.insert(key.to_string(), entry);
            }
        }
        Ok(old_value)
    }

    pub fn get_and_delete(&self, key: &str) -> Result<Option<Vec<u8>>> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        match data.remove(key) {
            Some(entry) if !entry.is_expired() => {
                if let Value::String(v) = entry.value {
                    Ok(Some(v))
                } else {
                    Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
                }
            }
            _ => Ok(None),
        }
    }

    pub fn delete(&self, keys: &[&str]) -> Result<u64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();
        let mut count = 0u64;
        for key in keys {
            if let Some(entry) = data.remove(*key) {
                if !entry.is_expired() {
                    count += 1;
                }
            }
        }
        Ok(count)
    }

    pub fn mget(&self, keys: &[&str]) -> Result<Vec<Option<Vec<u8>>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();
        let mut results = Vec::with_capacity(keys.len());
        for key in keys {
            let value = data.get(*key).and_then(|entry| {
                if entry.is_expired() {
                    None
                } else if let Value::String(v) = &entry.value {
                    Some(v.clone())
                } else {
                    None
                }
            });
            results.push(value);
        }
        Ok(results)
    }

    pub fn append(&self, key: &str, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = data
            .entry(key.to_string())
            .or_insert_with(|| Entry::new(Value::String(Vec::new())));

        if entry.is_expired() {
            *entry = Entry::new(Value::String(value.to_vec()));
            entry.apply_ttl_op(ttl);
            return Ok(value.len() as i64);
        }

        let result = if let Value::String(ref mut v) = entry.value {
            v.extend_from_slice(value);
            Ok(v.len() as i64)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn get_range(&self, key: &str, start: i64, end: i64) -> Result<Vec<u8>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        match data.get(key) {
            Some(entry) if !entry.is_expired() => {
                if let Value::String(v) = &entry.value {
                    let len = v.len() as i64;
                    let start = if start < 0 {
                        (len + start).max(0)
                    } else {
                        start.min(len)
                    } as usize;
                    let end = if end < 0 {
                        (len + end).max(0)
                    } else {
                        end.min(len - 1)
                    } as usize;
                    if start > end || start >= v.len() {
                        Ok(Vec::new())
                    } else {
                        Ok(v[start..=end.min(v.len() - 1)].to_vec())
                    }
                } else {
                    Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
                }
            }
            _ => Ok(Vec::new()),
        }
    }

    pub fn set_range(
        &self,
        key: &str,
        offset: i64,
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = data
            .entry(key.to_string())
            .or_insert_with(|| Entry::new(Value::String(Vec::new())));

        if entry.is_expired() {
            *entry = Entry::new(Value::String(Vec::new()));
        }

        let result = if let Value::String(ref mut v) = entry.value {
            let offset = offset as usize;
            // Extend with zeros if needed
            if v.len() < offset {
                v.resize(offset, 0);
            }
            // Overwrite or extend
            let end = offset + value.len();
            if end > v.len() {
                v.resize(end, 0);
            }
            v[offset..end].copy_from_slice(value);
            Ok(v.len() as i64)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn strlen(&self, key: &str) -> Result<i64> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        match data.get(key) {
            Some(entry) if !entry.is_expired() => {
                if let Value::String(v) = &entry.value {
                    Ok(v.len() as i64)
                } else {
                    Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
                }
            }
            _ => Ok(0),
        }
    }

    pub fn incr_by(&self, key: &str, delta: i64, ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = data
            .entry(key.to_string())
            .or_insert_with(|| Entry::new(Value::String(b"0".to_vec())));

        if entry.is_expired() {
            *entry = Entry::new(Value::String(b"0".to_vec()));
        }

        let result = if let Value::String(ref mut v) = entry.value {
            let current: i64 = std::str::from_utf8(v)
                .map_err(|_| Error::InvalidValue("value is not a valid integer".to_string()))?
                .parse()
                .map_err(|_| Error::InvalidValue("value is not a valid integer".to_string()))?;

            let new_val = current + delta;
            *v = new_val.to_string().into_bytes();
            Ok(new_val)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn incr_by_float(&self, key: &str, delta: f64, ttl: Option<TtlOp>) -> Result<f64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = data
            .entry(key.to_string())
            .or_insert_with(|| Entry::new(Value::String(b"0".to_vec())));

        if entry.is_expired() {
            *entry = Entry::new(Value::String(b"0".to_vec()));
        }

        let result = if let Value::String(ref mut v) = entry.value {
            let current: f64 = std::str::from_utf8(v)
                .map_err(|_| Error::InvalidValue("value is not a valid float".to_string()))?
                .parse()
                .map_err(|_| Error::InvalidValue("value is not a valid float".to_string()))?;

            let new_val = current + delta;
            *v = new_val.to_string().into_bytes();
            Ok(new_val)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_STRING.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    fn get_or_create_list<'a>(
        &self,
        data: &'a mut HashMap<String, Entry>,
        key: &str,
    ) -> Result<&'a mut VecDeque<Vec<u8>>> {
        let entry = data
            .entry(key.to_string())
            .or_insert_with(|| Entry::new(Value::List(VecDeque::new())));

        if entry.is_expired() {
            *entry = Entry::new(Value::List(VecDeque::new()));
        }

        match &mut entry.value {
            Value::List(list) => Ok(list),
            _ => Err(Error::TypeMismatch(TYPE_ERR_LIST.into())),
        }
    }

    pub fn lpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();
        let list = self.get_or_create_list(&mut data, key)?;

        for v in values.iter().rev() {
            list.push_front(v.to_vec());
        }
        let len = list.len() as i64;
        if let Some(entry) = data.get_mut(key) {
            entry.apply_ttl_op(ttl);
        }
        Ok(len)
    }

    pub fn rpush(&self, key: &str, values: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();
        let list = self.get_or_create_list(&mut data, key)?;

        for v in values {
            list.push_back(v.to_vec());
        }
        let len = list.len() as i64;
        if let Some(entry) = data.get_mut(key) {
            entry.apply_ttl_op(ttl);
        }
        Ok(len)
    }

    pub fn lpop(
        &self,
        key: &str,
        count: Option<usize>,
        ttl: Option<TtlOp>,
    ) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => {
                return if count.is_none() {
                    Err(Error::KeyNotFound)
                } else {
                    Ok(Vec::new())
                };
            }
        };

        let result = if let Value::List(list) = &mut entry.value {
            let n = count.unwrap_or(1);
            let mut results = Vec::with_capacity(n);
            for _ in 0..n {
                if let Some(v) = list.pop_front() {
                    results.push(v);
                } else {
                    break;
                }
            }
            if count.is_none() && results.is_empty() {
                Err(Error::KeyNotFound)
            } else {
                Ok(results)
            }
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn rpop(
        &self,
        key: &str,
        count: Option<usize>,
        ttl: Option<TtlOp>,
    ) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => {
                return if count.is_none() {
                    Err(Error::KeyNotFound)
                } else {
                    Ok(Vec::new())
                };
            }
        };

        let result = if let Value::List(list) = &mut entry.value {
            let n = count.unwrap_or(1);
            let mut results = Vec::with_capacity(n);
            for _ in 0..n {
                if let Some(v) = list.pop_back() {
                    results.push(v);
                } else {
                    break;
                }
            }
            if count.is_none() && results.is_empty() {
                Err(Error::KeyNotFound)
            } else {
                Ok(results)
            }
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn lindex(&self, key: &str, index: i64) -> Result<Option<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        let entry = match data.get(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(None),
        };

        if let Value::List(list) = &entry.value {
            let len = list.len() as i64;
            let idx = if index < 0 { len + index } else { index };
            if idx < 0 || idx >= len {
                Ok(None)
            } else {
                Ok(list.get(idx as usize).cloned())
            }
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        }
    }

    pub fn lset(&self, key: &str, index: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<()> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Err(Error::NoSuchKey),
        };

        let result = if let Value::List(list) = &mut entry.value {
            let len = list.len() as i64;
            let idx = if index < 0 { len + index } else { index };
            if idx < 0 || idx >= len {
                return Err(Error::InvalidValue("index out of range".to_string()));
            }
            list[idx as usize] = value.to_vec();
            Ok(())
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn lrange(&self, key: &str, start: i64, stop: i64) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        let entry = match data.get(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(Vec::new()),
        };

        if let Value::List(list) = &entry.value {
            let len = list.len() as i64;
            let start = if start < 0 {
                (len + start).max(0)
            } else {
                start
            } as usize;
            let stop = if stop < 0 {
                (len + stop).max(0)
            } else {
                stop.min(len - 1)
            } as usize;

            if start > stop || start >= list.len() {
                Ok(Vec::new())
            } else {
                Ok(list
                    .iter()
                    .skip(start)
                    .take(stop - start + 1)
                    .cloned()
                    .collect())
            }
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        }
    }

    pub fn ltrim(&self, key: &str, start: i64, stop: i64, ttl: Option<TtlOp>) -> Result<()> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(()),
        };

        let result = if let Value::List(list) = &mut entry.value {
            let len = list.len() as i64;
            let start = if start < 0 {
                (len + start).max(0)
            } else {
                start
            } as usize;
            let stop = if stop < 0 {
                (len + stop).max(0)
            } else {
                stop.min(len - 1)
            } as usize;

            if start > stop || start >= list.len() {
                list.clear();
            } else {
                let new_list: VecDeque<_> = list
                    .iter()
                    .skip(start)
                    .take(stop - start + 1)
                    .cloned()
                    .collect();
                *list = new_list;
            }
            Ok(())
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn linsert_before(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(-1),
        };

        let result = if let Value::List(list) = &mut entry.value {
            if let Some(pos) = list.iter().position(|v| v == pivot) {
                list.insert(pos, value.to_vec());
                Ok(list.len() as i64)
            } else {
                Ok(-1)
            }
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn linsert_after(
        &self,
        key: &str,
        pivot: &[u8],
        value: &[u8],
        ttl: Option<TtlOp>,
    ) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(-1),
        };

        let result = if let Value::List(list) = &mut entry.value {
            if let Some(pos) = list.iter().position(|v| v == pivot) {
                list.insert(pos + 1, value.to_vec());
                Ok(list.len() as i64)
            } else {
                Ok(-1)
            }
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn lrem(&self, key: &str, count: i64, value: &[u8], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(0),
        };

        let result = if let Value::List(list) = &mut entry.value {
            let mut removed = 0i64;
            let abs_count = count.unsigned_abs() as i64;

            if count > 0 {
                // Remove first count occurrences
                let mut i = 0;
                while i < list.len() && (abs_count == 0 || removed < abs_count) {
                    if list[i] == value {
                        list.remove(i);
                        removed += 1;
                    } else {
                        i += 1;
                    }
                }
            } else if count < 0 {
                // Remove last count occurrences
                let mut i = list.len();
                while i > 0 && (abs_count == 0 || removed < abs_count) {
                    i -= 1;
                    if list[i] == value {
                        list.remove(i);
                        removed += 1;
                    }
                }
            } else {
                // Remove all occurrences
                list.retain(|v| {
                    if v == value {
                        removed += 1;
                        false
                    } else {
                        true
                    }
                });
            }
            Ok(removed)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn lmove(
        &self,
        src: &str,
        dst: &str,
        src_dir: ListDirection,
        dst_dir: ListDirection,
        ttl: Option<TtlOp>,
    ) -> Result<Option<Vec<u8>>> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        // Pop from source
        let value = {
            let entry = match data.get_mut(src) {
                Some(e) if !e.is_expired() => e,
                _ => return Ok(None),
            };

            if let Value::List(list) = &mut entry.value {
                match src_dir {
                    ListDirection::Left => list.pop_front(),
                    ListDirection::Right => list.pop_back(),
                }
            } else {
                return Err(Error::TypeMismatch(TYPE_ERR_LIST.into()));
            }
        };

        let value = match value {
            Some(v) => v,
            None => return Ok(None),
        };

        // Push to destination
        let ret = {
            let entry = data
                .entry(dst.to_string())
                .or_insert_with(|| Entry::new(Value::List(VecDeque::new())));

            if entry.is_expired() {
                *entry = Entry::new(Value::List(VecDeque::new()));
            }

            if let Value::List(list) = &mut entry.value {
                let ret = value.clone();
                match dst_dir {
                    ListDirection::Left => list.push_front(value),
                    ListDirection::Right => list.push_back(value),
                }
                ret
            } else {
                return Err(Error::TypeMismatch(TYPE_ERR_LIST.into()));
            }
        };

        if let Some(entry) = data.get_mut(dst) {
            entry.apply_ttl_op(ttl);
        }
        Ok(Some(ret))
    }

    pub fn llen(&self, key: &str) -> Result<i64> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        let entry = match data.get(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(0),
        };

        if let Value::List(list) = &entry.value {
            Ok(list.len() as i64)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_LIST.into()))
        }
    }

    fn get_or_create_set<'a>(
        &self,
        data: &'a mut HashMap<String, Entry>,
        key: &str,
    ) -> Result<&'a mut HashSet<Vec<u8>>> {
        let entry = data
            .entry(key.to_string())
            .or_insert_with(|| Entry::new(Value::Set(HashSet::new())));

        if entry.is_expired() {
            *entry = Entry::new(Value::Set(HashSet::new()));
        }

        match &mut entry.value {
            Value::Set(set) => Ok(set),
            _ => Err(Error::TypeMismatch(TYPE_ERR_SET.into())),
        }
    }

    fn get_set<'a>(
        &self,
        data: &'a HashMap<String, Entry>,
        key: &str,
    ) -> Result<Option<&'a HashSet<Vec<u8>>>> {
        match data.get(key) {
            Some(e) if !e.is_expired() => {
                if let Value::Set(set) = &e.value {
                    Ok(Some(set))
                } else {
                    Err(Error::TypeMismatch(TYPE_ERR_SET.into()))
                }
            }
            _ => Ok(None),
        }
    }

    pub fn sadd(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();
        let set = self.get_or_create_set(&mut data, key)?;

        let mut added = 0i64;
        for m in members {
            if set.insert(m.to_vec()) {
                added += 1;
            }
        }
        if let Some(entry) = data.get_mut(key) {
            entry.apply_ttl_op(ttl);
        }
        Ok(added)
    }

    pub fn srem(&self, key: &str, members: &[&[u8]], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(0),
        };

        let result = if let Value::Set(set) = &mut entry.value {
            let mut removed = 0i64;
            for m in members {
                if set.remove(*m) {
                    removed += 1;
                }
            }
            Ok(removed)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_SET.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn sismember(&self, key: &str, member: &[u8]) -> Result<bool> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        match self.get_set(&data, key)? {
            Some(set) => Ok(set.contains(member)),
            None => Ok(false),
        }
    }

    pub fn spop(
        &self,
        key: &str,
        count: Option<usize>,
        ttl: Option<TtlOp>,
    ) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let entry = match data.get_mut(key) {
            Some(e) if !e.is_expired() => e,
            _ => return Ok(Vec::new()),
        };

        let result = if let Value::Set(set) = &mut entry.value {
            let count = count.unwrap_or(1);
            let mut results = Vec::with_capacity(count);
            for _ in 0..count {
                if let Some(v) = set.iter().next().cloned() {
                    set.remove(&v);
                    results.push(v);
                } else {
                    break;
                }
            }
            Ok(results)
        } else {
            Err(Error::TypeMismatch(TYPE_ERR_SET.into()))
        };

        if result.is_ok() {
            entry.apply_ttl_op(ttl);
        }
        result
    }

    pub fn srandmember(&self, key: &str, count: i64) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        match self.get_set(&data, key)? {
            Some(set) => {
                let abs_count = count.unsigned_abs() as usize;
                let allow_duplicates = count < 0;

                if set.is_empty() {
                    return Ok(Vec::new());
                }

                let members: Vec<_> = set.iter().cloned().collect();
                let mut results = Vec::with_capacity(abs_count);

                if allow_duplicates {
                    // Allow duplicates — each pick is independently random
                    use std::collections::hash_map::RandomState;
                    use std::hash::{BuildHasher, Hasher};

                    for _ in 0..abs_count {
                        // RandomState::new() uses per-call random seeds
                        let idx =
                            RandomState::new().build_hasher().finish() as usize % members.len();
                        results.push(members[idx].clone());
                    }
                } else {
                    // No duplicates
                    for m in members.iter().take(abs_count) {
                        results.push(m.clone());
                    }
                }
                Ok(results)
            }
            None => Ok(Vec::new()),
        }
    }

    pub fn smembers(&self, key: &str) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        match self.get_set(&data, key)? {
            Some(set) => Ok(set.iter().cloned().collect()),
            None => Ok(Vec::new()),
        }
    }

    pub fn scard(&self, key: &str) -> Result<i64> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        match self.get_set(&data, key)? {
            Some(set) => Ok(set.len() as i64),
            None => Ok(0),
        }
    }

    pub fn sdiff(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        if keys.is_empty() {
            return Ok(Vec::new());
        }

        let first = match self.get_set(&data, keys[0])? {
            Some(set) => set.clone(),
            None => return Ok(Vec::new()),
        };

        let mut result = first;
        for key in &keys[1..] {
            if let Some(set) = self.get_set(&data, key)? {
                result = result.difference(set).cloned().collect();
            }
        }
        Ok(result.into_iter().collect())
    }

    pub fn sdiffstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        if keys.is_empty() {
            data.insert(
                dest.to_string(),
                Entry::new_with_ttl_op(Value::Set(HashSet::new()), ttl),
            );
            return Ok(0);
        }

        let first = match self.get_set(&data, keys[0])? {
            Some(set) => set.clone(),
            None => {
                data.insert(
                    dest.to_string(),
                    Entry::new_with_ttl_op(Value::Set(HashSet::new()), ttl),
                );
                return Ok(0);
            }
        };

        let mut result = first;
        for key in &keys[1..] {
            if let Some(set) = self.get_set(&data, key)? {
                result = result.difference(set).cloned().collect();
            }
        }

        let count = result.len() as i64;
        data.insert(
            dest.to_string(),
            Entry::new_with_ttl_op(Value::Set(result), ttl),
        );
        Ok(count)
    }

    pub fn sinter(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        if keys.is_empty() {
            return Ok(Vec::new());
        }

        let first = match self.get_set(&data, keys[0])? {
            Some(set) => set.clone(),
            None => return Ok(Vec::new()),
        };

        let mut result = first;
        for key in &keys[1..] {
            match self.get_set(&data, key)? {
                Some(set) => {
                    result = result.intersection(set).cloned().collect();
                }
                None => return Ok(Vec::new()),
            }
        }
        Ok(result.into_iter().collect())
    }

    pub fn sinterstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        if keys.is_empty() {
            data.insert(
                dest.to_string(),
                Entry::new_with_ttl_op(Value::Set(HashSet::new()), ttl),
            );
            return Ok(0);
        }

        let first = match self.get_set(&data, keys[0])? {
            Some(set) => set.clone(),
            None => {
                data.insert(
                    dest.to_string(),
                    Entry::new_with_ttl_op(Value::Set(HashSet::new()), ttl),
                );
                return Ok(0);
            }
        };

        let mut result = first;
        for key in &keys[1..] {
            match self.get_set(&data, key)? {
                Some(set) => {
                    result = result.intersection(set).cloned().collect();
                }
                None => {
                    data.insert(
                        dest.to_string(),
                        Entry::new_with_ttl_op(Value::Set(HashSet::new()), ttl),
                    );
                    return Ok(0);
                }
            }
        }

        let count = result.len() as i64;
        data.insert(
            dest.to_string(),
            Entry::new_with_ttl_op(Value::Set(result), ttl),
        );
        Ok(count)
    }

    pub fn sunion(&self, keys: &[&str]) -> Result<Vec<Vec<u8>>> {
        self.maybe_cleanup();
        let data = self.data.read().unwrap();

        let mut result = HashSet::new();
        for key in keys {
            if let Some(set) = self.get_set(&data, key)? {
                result.extend(set.iter().cloned());
            }
        }
        Ok(result.into_iter().collect())
    }

    pub fn sunionstore(&self, dest: &str, keys: &[&str], ttl: Option<TtlOp>) -> Result<i64> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        let mut result = HashSet::new();
        for key in keys {
            if let Some(set) = self.get_set(&data, key)? {
                result.extend(set.iter().cloned());
            }
        }

        let count = result.len() as i64;
        data.insert(
            dest.to_string(),
            Entry::new_with_ttl_op(Value::Set(result), ttl),
        );
        Ok(count)
    }

    pub fn smove(&self, src: &str, dst: &str, member: &[u8], ttl: Option<TtlOp>) -> Result<bool> {
        self.maybe_cleanup();
        let mut data = self.data.write().unwrap();

        // Remove from source
        let removed = {
            let entry = match data.get_mut(src) {
                Some(e) if !e.is_expired() => e,
                _ => return Ok(false),
            };

            if let Value::Set(set) = &mut entry.value {
                set.remove(member)
            } else {
                return Err(Error::TypeMismatch(TYPE_ERR_SET.into()));
            }
        };

        if !removed {
            return Ok(false);
        }

        // Add to destination
        let set = self.get_or_create_set(&mut data, dst)?;
        set.insert(member.to_vec());
        if let Some(entry) = data.get_mut(dst) {
            entry.apply_ttl_op(ttl);
        }
        Ok(true)
    }
}

#[cfg(test)]
#[path = "memcluster_tests.rs"]
mod memcluster_tests;
