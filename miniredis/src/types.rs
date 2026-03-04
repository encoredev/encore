use std::collections::HashMap;

/// The type tag for a Redis key.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum KeyType {
    String,
    Hash,
    List,
    Set,
    SortedSet,
    Stream,
    HyperLogLog,
}

impl KeyType {
    /// Return the Redis TYPE command string for this key type.
    pub fn as_str(&self) -> &'static str {
        match self {
            KeyType::String => "string",
            KeyType::Hash => "hash",
            KeyType::List => "list",
            KeyType::Set => "set",
            KeyType::SortedSet => "zset",
            KeyType::Stream => "stream",
            KeyType::HyperLogLog => "hll", // not "string" — miniredis uses a distinct type
        }
    }
}

/// A sorted set element with score and member.
#[derive(Clone, Debug)]
pub struct SSElem {
    pub score: f64,
    pub member: String,
}

/// Ascending or descending direction for sorted set operations.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Direction {
    Asc,
    Desc,
}

/// Redis sorted set — uses a HashMap for O(1) score lookups, sorts on demand for range queries.
#[derive(Clone, Debug, Default)]
pub struct SortedSet {
    pub scores: HashMap<String, f64>,
}

impl SortedSet {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn card(&self) -> usize {
        self.scores.len()
    }

    /// Add or update a member. Returns true if the member was new.
    pub fn set(&mut self, score: f64, member: &str) -> bool {
        let is_new = !self.scores.contains_key(member);
        self.scores.insert(member.to_owned(), score);
        is_new
    }

    /// Get a member's score.
    pub fn get(&self, member: &str) -> Option<f64> {
        self.scores.get(member).copied()
    }

    /// Check if a member exists.
    pub fn exists(&self, member: &str) -> bool {
        self.scores.contains_key(member)
    }

    /// Remove a member. Returns true if it existed.
    pub fn remove(&mut self, member: &str) -> bool {
        self.scores.remove(member).is_some()
    }

    /// Return all elements sorted by (score, member).
    pub fn by_score(&self, dir: Direction) -> Vec<SSElem> {
        let mut elems: Vec<SSElem> = self
            .scores
            .iter()
            .map(|(m, &s)| SSElem {
                score: s,
                member: m.clone(),
            })
            .collect();
        elems.sort_by(|a, b| {
            a.score
                .partial_cmp(&b.score)
                .unwrap_or(std::cmp::Ordering::Equal)
                .then_with(|| a.member.cmp(&b.member))
        });
        if dir == Direction::Desc {
            elems.reverse();
        }
        elems
    }

    /// Return sorted member names (all same score → lex order).
    pub fn members_sorted(&self) -> Vec<String> {
        self.by_score(Direction::Asc)
            .into_iter()
            .map(|e| e.member)
            .collect()
    }

    /// Get the rank (0-based index) of a member in sorted order.
    pub fn rank(&self, member: &str, dir: Direction) -> Option<usize> {
        if !self.scores.contains_key(member) {
            return None;
        }
        let elems = self.by_score(dir);
        elems.iter().position(|e| e.member == member)
    }

    /// Increment a member's score by delta. Creates the member if it doesn't exist.
    /// Returns the new score.
    pub fn incrby(&mut self, member: &str, delta: f64) -> f64 {
        let score = self.scores.entry(member.to_owned()).or_insert(0.0);
        *score += delta;
        *score
    }
}

/// A single stream entry.
#[derive(Clone, Debug)]
pub struct StreamEntry {
    pub id: String,
    /// Alternating key-value pairs.
    pub values: Vec<String>,
}

/// A pending entry in a consumer group's PEL.
#[derive(Clone, Debug)]
pub struct PendingEntry {
    pub id: String,
    pub consumer: String,
    pub delivery_count: i64,
    pub last_delivery: std::time::SystemTime,
}

/// A consumer within a group.
#[derive(Clone, Debug)]
pub struct StreamConsumer {
    pub num_pending: i64,
    pub last_seen: std::time::SystemTime,
    pub last_success: std::time::SystemTime,
}

/// A consumer group on a stream.
#[derive(Clone, Debug)]
pub struct StreamGroup {
    pub last_id: String,
    pub pending: Vec<PendingEntry>,
    pub consumers: HashMap<String, StreamConsumer>,
}

/// Redis stream.
#[derive(Clone, Debug, Default)]
pub struct Stream {
    pub entries: Vec<StreamEntry>,
    pub groups: HashMap<String, StreamGroup>,
    pub last_allocated_id: String,
}

impl Stream {
    pub fn new() -> Self {
        Self::default()
    }

    /// Parse a stream ID string "ms-seq" into (ms, seq).
    pub fn parse_id(id: &str) -> Result<(u64, u64), &'static str> {
        let parts: Vec<&str> = id.splitn(2, '-').collect();
        let ms = parts[0]
            .parse::<u64>()
            .map_err(|_| "ERR Invalid stream ID specified as stream command argument")?;
        let seq = if parts.len() > 1 {
            parts[1]
                .parse::<u64>()
                .map_err(|_| "ERR Invalid stream ID specified as stream command argument")?
        } else {
            0
        };
        Ok((ms, seq))
    }

    /// Compare two stream IDs. Returns Ordering.
    pub fn cmp_ids(a: &str, b: &str) -> std::cmp::Ordering {
        let a_parsed = Self::parse_id(a).unwrap_or((0, 0));
        let b_parsed = Self::parse_id(b).unwrap_or((0, 0));
        a_parsed.cmp(&b_parsed)
    }

    /// Format a stream ID from parts.
    pub fn format_id(ms: u64, seq: u64) -> String {
        format!("{}-{}", ms, seq)
    }

    /// Normalize a partial ID to full "ms-seq" format.
    pub fn normalize_id(id: &str) -> String {
        if id.contains('-') {
            id.to_string()
        } else {
            format!("{}-0", id)
        }
    }

    /// Get the last entry's ID, or "0-0" if empty.
    pub fn last_id(&self) -> &str {
        self.entries.last().map(|e| e.id.as_str()).unwrap_or("0-0")
    }

    /// Generate a new ID based on timestamp.
    pub fn generate_id(&mut self, ms: u64) -> String {
        let mut new_ms = ms;
        let mut new_seq = 0u64;

        // Check against lastAllocatedID
        if !self.last_allocated_id.is_empty()
            && let Ok((alloc_ms, alloc_seq)) = Self::parse_id(&self.last_allocated_id)
            && new_ms <= alloc_ms
        {
            new_ms = alloc_ms;
            new_seq = alloc_seq + 1;
        }

        // Check against last entry
        if let Some(last) = self.entries.last()
            && let Ok((last_ms, last_seq)) = Self::parse_id(&last.id)
            && (new_ms < last_ms || (new_ms == last_ms && new_seq <= last_seq))
        {
            new_ms = last_ms;
            new_seq = last_seq + 1;
        }

        let id = Self::format_id(new_ms, new_seq);
        self.last_allocated_id = id.clone();
        id
    }

    /// Generate ID with a specific timestamp and auto-sequence.
    pub fn generate_id_seq(&mut self, ms: u64) -> String {
        self.generate_id(ms)
    }

    /// Add an entry. Returns the assigned ID or error.
    pub fn add(
        &mut self,
        id: &str,
        values: Vec<String>,
        now_ms: u64,
    ) -> Result<String, &'static str> {
        let final_id = if id.is_empty() || id == "*" {
            self.generate_id(now_ms)
        } else if let Some(ms_str) = id.strip_suffix("-*") {
            let ms = ms_str
                .parse::<u64>()
                .map_err(|_| "ERR Invalid stream ID specified as stream command argument")?;
            self.generate_id_seq(ms)
        } else {
            let normalized = Self::normalize_id(id);
            // Validate the ID
            let (ms, seq) = Self::parse_id(&normalized)?;
            if ms == 0 && seq == 0 {
                return Err("ERR The ID specified in XADD must be greater than 0-0");
            }
            // Must be greater than the last entry
            if let Some(last) = self.entries.last()
                && Self::cmp_ids(&normalized, &last.id) != std::cmp::Ordering::Greater
            {
                return Err(
                    "ERR The ID specified in XADD is equal or smaller than the target stream top item",
                );
            }
            self.last_allocated_id = normalized.clone();
            normalized
        };

        self.entries.push(StreamEntry {
            id: final_id.clone(),
            values,
        });

        Ok(final_id)
    }

    /// Trim to at most n entries (MAXLEN).
    pub fn trim_maxlen(&mut self, n: usize) -> i64 {
        if self.entries.len() <= n {
            return 0;
        }
        let remove = self.entries.len() - n;
        self.entries.drain(..remove);
        remove as i64
    }

    /// Remove all entries with ID < threshold (MINID).
    pub fn trim_minid(&mut self, threshold: &str) -> i64 {
        let before = self.entries.len();
        self.entries
            .retain(|e| Self::cmp_ids(&e.id, threshold) != std::cmp::Ordering::Less);
        (before - self.entries.len()) as i64
    }

    /// Get entries after the given ID.
    pub fn after(&self, id: &str) -> Vec<&StreamEntry> {
        self.entries
            .iter()
            .filter(|e| Self::cmp_ids(&e.id, id) == std::cmp::Ordering::Greater)
            .collect()
    }

    /// Get entries in range [start, end].
    pub fn range(&self, start: &str, end: &str, count: Option<usize>) -> Vec<&StreamEntry> {
        let mut result: Vec<&StreamEntry> = self
            .entries
            .iter()
            .filter(|e| {
                Self::cmp_ids(&e.id, start) != std::cmp::Ordering::Less
                    && Self::cmp_ids(&e.id, end) != std::cmp::Ordering::Greater
            })
            .collect();
        if let Some(c) = count {
            result.truncate(c);
        }
        result
    }

    /// Get entries in reverse range [end, start] (for XREVRANGE).
    pub fn rev_range(&self, start: &str, end: &str, count: Option<usize>) -> Vec<&StreamEntry> {
        let mut result: Vec<&StreamEntry> = self
            .entries
            .iter()
            .filter(|e| {
                Self::cmp_ids(&e.id, start) != std::cmp::Ordering::Less
                    && Self::cmp_ids(&e.id, end) != std::cmp::Ordering::Greater
            })
            .rev()
            .collect();
        if let Some(c) = count {
            result.truncate(c);
        }
        result
    }

    /// Delete entries by ID. Returns count deleted.
    pub fn del(&mut self, ids: &[&str]) -> i64 {
        let mut count = 0i64;
        for id in ids {
            let before = self.entries.len();
            self.entries.retain(|e| e.id != **id);
            if self.entries.len() < before {
                count += 1;
            }
        }
        count
    }

    /// Get an entry by ID.
    pub fn get(&self, id: &str) -> Option<&StreamEntry> {
        self.entries.iter().find(|e| e.id == id)
    }

    /// Check if an entry exists.
    pub fn entry_exists(&self, id: &str) -> bool {
        self.entries.iter().any(|e| e.id == id)
    }

    /// Create a consumer group. Returns error if already exists.
    pub fn create_group(&mut self, name: &str, id: &str) -> Result<(), String> {
        if self.groups.contains_key(name) {
            return Err("BUSYGROUP Consumer Group name already exists".to_string());
        }
        let last_id = if id == "$" {
            self.last_id().to_string()
        } else {
            Self::normalize_id(id)
        };
        self.groups.insert(
            name.to_string(),
            StreamGroup {
                last_id,
                pending: Vec::new(),
                consumers: HashMap::new(),
            },
        );
        Ok(())
    }

    /// Read from a consumer group. Returns entries and updates PEL.
    pub fn read_group(
        &mut self,
        group_name: &str,
        consumer_name: &str,
        id: &str,
        count: Option<usize>,
        noack: bool,
        now: std::time::SystemTime,
    ) -> Result<Vec<StreamEntry>, String> {
        let group = self.groups.get_mut(group_name).ok_or_else(|| {
            format!(
                "NOGROUP No such consumer group '{}' for key name",
                group_name
            )
        })?;

        // Ensure consumer exists
        group
            .consumers
            .entry(consumer_name.to_string())
            .or_insert(StreamConsumer {
                num_pending: 0,
                last_seen: now,
                last_success: now,
            });

        if id == ">" {
            // New undelivered messages
            let entries: Vec<StreamEntry> = self
                .entries
                .iter()
                .filter(|e| Self::cmp_ids(&e.id, &group.last_id) == std::cmp::Ordering::Greater)
                .cloned()
                .collect();

            let entries = if let Some(c) = count {
                entries.into_iter().take(c).collect::<Vec<_>>()
            } else {
                entries
            };

            if let Some(last) = entries.last() {
                group.last_id = last.id.clone();
            }

            if !noack {
                for entry in &entries {
                    group.pending.push(PendingEntry {
                        id: entry.id.clone(),
                        consumer: consumer_name.to_string(),
                        delivery_count: 1,
                        last_delivery: now,
                    });
                    if let Some(c) = group.consumers.get_mut(consumer_name) {
                        c.num_pending += 1;
                        c.last_success = now;
                    }
                }
            }

            Ok(entries)
        } else {
            // Re-deliver from PEL
            let normalized = Self::normalize_id(id);
            let mut result = Vec::new();
            for pe in &mut group.pending {
                if pe.consumer != consumer_name {
                    continue;
                }
                if Self::cmp_ids(&pe.id, &normalized) == std::cmp::Ordering::Less {
                    continue;
                }
                // Find the entry in the stream
                if let Some(entry) = self.entries.iter().find(|e| e.id == pe.id) {
                    pe.delivery_count += 1;
                    pe.last_delivery = now;
                    result.push(entry.clone());
                }
                if let Some(c) = count
                    && result.len() >= c
                {
                    break;
                }
            }
            Ok(result)
        }
    }

    /// Acknowledge entries. Returns count acknowledged.
    pub fn ack(&mut self, group_name: &str, ids: &[&str]) -> Result<i64, String> {
        let group = self.groups.get_mut(group_name).ok_or_else(|| {
            format!(
                "NOGROUP No such consumer group '{}' for key name",
                group_name
            )
        })?;

        let mut count = 0i64;
        for id in ids {
            let before = group.pending.len();
            let consumer_name = group
                .pending
                .iter()
                .find(|pe| pe.id == **id)
                .map(|pe| pe.consumer.clone());
            group.pending.retain(|pe| pe.id != **id);
            if group.pending.len() < before {
                count += 1;
                if let Some(cname) = consumer_name
                    && let Some(c) = group.consumers.get_mut(&cname)
                {
                    c.num_pending -= 1;
                }
            }
        }
        Ok(count)
    }
}

/// Format a range bound for XRANGE/XREVRANGE.
pub fn format_stream_range_bound(id: &str, is_start: bool) -> String {
    match id {
        "-" => "0-0".to_string(),
        "+" => format!("{}-{}", u64::MAX, u64::MAX),
        _ => {
            if id.contains('-') {
                id.to_string()
            } else if is_start {
                format!("{}-0", id)
            } else {
                format!("{}-{}", id, u64::MAX)
            }
        }
    }
}

// HyperLogLog is implemented in src/hll.rs.
