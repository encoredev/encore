use std::sync::Arc;
use std::time::Duration;

use rand::Rng;

use super::parse_int;
use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_DB_INDEX_OUT_OF_RANGE, MSG_INVALID_CURSOR, MSG_INVALID_INT,
    MSG_KEY_NOT_FOUND, MSG_SYNTAX_ERROR, MSG_TIMEOUT_NEGATIVE, err_wrong_number,
};
use crate::frame::Frame;
use crate::types::KeyType;

pub fn register(table: &mut CommandTable) {
    table.add("DEL", cmd_del, false);
    table.add("UNLINK", cmd_del, false); // alias
    table.add("EXISTS", cmd_exists, true);
    table.add("TYPE", cmd_type, true);
    table.add("RENAME", cmd_rename, false);
    table.add("RENAMENX", cmd_renamenx, false);
    table.add("EXPIRE", cmd_expire, false);
    table.add("EXPIREAT", cmd_expireat, false);
    table.add("PEXPIRE", cmd_pexpire, false);
    table.add("PEXPIREAT", cmd_pexpireat, false);
    table.add("PERSIST", cmd_persist, false);
    table.add("TTL", cmd_ttl, true);
    table.add("PTTL", cmd_pttl, true);
    table.add("KEYS", cmd_keys, true);
    table.add("SCAN", cmd_scan, true);
    table.add("TOUCH", cmd_touch, true);
    table.add("WAIT", cmd_wait, true);
    table.add("RANDOMKEY", cmd_randomkey, true);
    table.add("OBJECT", cmd_object, true);
    table.add("EXPIRETIME", cmd_expiretime, true);
    table.add("PEXPIRETIME", cmd_pexpiretime, true);
    table.add("COPY", cmd_copy, false);
    table.add("MOVE", cmd_move, false);
    table.add("DUMP", cmd_dump, true);
    table.add("RESTORE", cmd_restore, false);
}

/// DEL key [key ...]
fn cmd_del(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("del"));
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    let mut count = 0i64;

    for arg in args {
        let key = String::from_utf8_lossy(arg);
        db.check_ttl(&key);
        if db.del(&key) {
            count += 1;
        }
    }

    Frame::Integer(count)
}

/// EXISTS key [key ...]
fn cmd_exists(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("exists"));
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    let mut count = 0i64;

    for arg in args {
        let key = String::from_utf8_lossy(arg);
        db.check_ttl(&key);
        if db.exists(&key, now) {
            count += 1;
        }
    }

    Frame::Integer(count)
}

/// TYPE key
fn cmd_type(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("type"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    let t = match db.key_type(&key) {
        Some(t) => t.as_str(),
        None => "none",
    };
    Frame::Simple(t.to_owned())
}

/// RENAME key newkey
fn cmd_rename(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("rename"));
    }

    let from = String::from_utf8_lossy(&args[0]).into_owned();
    let to = String::from_utf8_lossy(&args[1]).into_owned();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if !db.keys.contains_key(&from) {
        return Frame::error(MSG_KEY_NOT_FOUND);
    }

    db.rename(&from, &to, now);
    Frame::ok()
}

/// RENAMENX key newkey
fn cmd_renamenx(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("renamenx"));
    }

    let from = String::from_utf8_lossy(&args[0]).into_owned();
    let to = String::from_utf8_lossy(&args[1]).into_owned();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if !db.keys.contains_key(&from) {
        return Frame::error(MSG_KEY_NOT_FOUND);
    }

    if db.keys.contains_key(&to) {
        return Frame::Integer(0);
    }

    db.rename(&from, &to, now);
    Frame::Integer(1)
}

/// EXPIRE key seconds [NX|XX|GT|LT]
fn cmd_expire(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    expire_impl(state, ctx, args, "expire", |secs, _| {
        Duration::from_secs(secs as u64)
    })
}

/// EXPIREAT key timestamp [NX|XX|GT|LT]
fn cmd_expireat(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    expire_impl(state, ctx, args, "expireat", |ts, now| {
        let target = std::time::UNIX_EPOCH + Duration::from_secs(ts as u64);
        target.duration_since(now).unwrap_or(Duration::ZERO)
    })
}

/// PEXPIRE key milliseconds [NX|XX|GT|LT]
fn cmd_pexpire(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    expire_impl(state, ctx, args, "pexpire", |ms, _| {
        Duration::from_millis(ms as u64)
    })
}

/// PEXPIREAT key timestamp-ms [NX|XX|GT|LT]
fn cmd_pexpireat(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    expire_impl(state, ctx, args, "pexpireat", |ts, now| {
        let target = std::time::UNIX_EPOCH + Duration::from_millis(ts as u64);
        target.duration_since(now).unwrap_or(Duration::ZERO)
    })
}

fn expire_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    cmd: &str,
    to_duration: impl Fn(i64, std::time::SystemTime) -> Duration,
) -> Frame {
    if args.len() < 2 || args.len() > 3 {
        return Frame::error(err_wrong_number(cmd));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let value: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    // Parse optional flag
    let mut nx = false;
    let mut xx = false;
    let mut gt = false;
    let mut lt = false;

    if args.len() == 3 {
        match String::from_utf8_lossy(&args[2]).to_uppercase().as_str() {
            "NX" => nx = true,
            "XX" => xx = true,
            "GT" => gt = true,
            "LT" => lt = true,
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    let new_ttl = to_duration(value, now);
    let has_ttl = db.ttl.contains_key(&key);

    // NX: only if no existing TTL
    if nx && has_ttl {
        return Frame::Integer(0);
    }
    // XX: only if has existing TTL
    if xx && !has_ttl {
        return Frame::Integer(0);
    }
    // GT: only if new TTL is greater
    if gt
        && let Some(&old_ttl) = db.ttl.get(&key)
        && new_ttl <= old_ttl
    {
        return Frame::Integer(0);
    }
    // LT: only if new TTL is less
    if lt {
        if let Some(&old_ttl) = db.ttl.get(&key) {
            if new_ttl >= old_ttl {
                return Frame::Integer(0);
            }
        } else {
            // No existing TTL, LT always applies
        }
    }

    db.ttl.insert(key.clone(), new_ttl);
    db.incr_version(&key, now);

    // Check if key already expired
    db.check_ttl(&key);

    Frame::Integer(1)
}

/// PERSIST key
fn cmd_persist(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("persist"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    if db.ttl.remove(&key).is_some() {
        db.incr_version(&key, now);
        Frame::Integer(1)
    } else {
        Frame::Integer(0)
    }
}

/// TTL key
fn cmd_ttl(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("ttl"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(-2);
    }

    match db.ttl.get(key.as_ref()) {
        Some(ttl) => Frame::Integer(ttl.as_secs() as i64),
        None => Frame::Integer(-1),
    }
}

/// PTTL key
fn cmd_pttl(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("pttl"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(-2);
    }

    match db.ttl.get(key.as_ref()) {
        Some(ttl) => Frame::Integer(ttl.as_millis() as i64),
        None => Frame::Integer(-1),
    }
}

/// KEYS pattern
fn cmd_keys(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("keys"));
    }

    let pattern = String::from_utf8_lossy(&args[0]);
    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    let all_keys = db.all_keys();
    let matched = match_keys(&all_keys, &pattern);

    Frame::Array(matched.into_iter().map(|k| Frame::Bulk(k.into())).collect())
}

/// SCAN cursor [MATCH pattern] [COUNT count] [TYPE type]
fn cmd_scan(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("scan"));
    }

    let cursor: i64 = match parse_int(&args[0]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_CURSOR),
    };

    let opts = match super::parse_scan_opts(&args[1..], true) {
        Ok(o) => o,
        Err(e) => return e,
    };

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    let mut all_keys = db.all_keys();

    // Filter by type
    if let Some(ref tf) = opts.type_filter {
        all_keys.retain(|k| {
            db.key_type(k)
                .map(|t| t.as_str() == tf.as_str())
                .unwrap_or(false)
        });
    }

    // Filter by pattern
    let matched = if let Some(ref pat) = opts.pattern {
        match_keys(&all_keys, pat)
    } else {
        all_keys
    };

    // Simple implementation: return all results at once, no real cursor pagination
    if cursor != 0 {
        return Frame::Array(vec![Frame::Bulk("0".into()), Frame::Array(vec![])]);
    }

    Frame::Array(vec![
        Frame::Bulk("0".into()),
        Frame::Array(matched.into_iter().map(|k| Frame::Bulk(k.into())).collect()),
    ])
}

/// TOUCH key [key ...]
fn cmd_touch(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("touch"));
    }

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);
    let mut count = 0i64;

    for arg in args {
        let key = String::from_utf8_lossy(arg);
        if db.keys.contains_key(key.as_ref()) {
            count += 1;
        }
    }

    Frame::Integer(count)
}

/// WAIT numreplicas timeout — always returns 0 (standalone)
fn cmd_wait(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("wait"));
    }

    let _replicas: i64 = match parse_int(&args[0]) {
        Some(n) if n >= 0 => n,
        _ => return Frame::error(MSG_INVALID_INT),
    };
    let timeout: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    if timeout < 0 {
        return Frame::error(MSG_TIMEOUT_NEGATIVE);
    }

    Frame::Integer(0)
}

/// RANDOMKEY
fn cmd_randomkey(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if !args.is_empty() {
        return Frame::error(err_wrong_number("randomkey"));
    }

    let mut inner = state.lock();
    let key_count = inner.db(ctx.selected_db).keys.len();

    if key_count == 0 {
        return Frame::Null;
    }

    let idx = inner.rng.random_range(0..key_count);
    let key = inner
        .db(ctx.selected_db)
        .keys
        .keys()
        .nth(idx)
        .unwrap()
        .clone();
    Frame::Bulk(key.into())
}

/// OBJECT subcommand ... — stub
fn cmd_object(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("object"));
    }

    let sub = String::from_utf8_lossy(&args[0]).to_uppercase();
    match sub.as_str() {
        "HELP" => Frame::Array(vec![Frame::Bulk("OBJECT subcommand [arguments]".into())]),
        "ENCODING" => {
            if args.len() != 2 {
                return Frame::error(err_wrong_number("object|encoding"));
            }
            // Stub: always return "raw"
            Frame::Bulk("raw".into())
        }
        "IDLETIME" => {
            if args.len() != 2 {
                return Frame::error(err_wrong_number("object|idletime"));
            }
            Frame::Integer(0)
        }
        "REFCOUNT" => {
            if args.len() != 2 {
                return Frame::error(err_wrong_number("object|refcount"));
            }
            Frame::Integer(1)
        }
        "FREQ" => {
            if args.len() != 2 {
                return Frame::error(err_wrong_number("object|freq"));
            }
            Frame::Integer(0)
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand or wrong number of arguments for 'object|{}' command",
            sub.to_lowercase()
        )),
    }
}

/// EXPIRETIME key
fn cmd_expiretime(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("expiretime"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(-2);
    }

    match db.ttl.get(key.as_ref()) {
        Some(ttl) => {
            let expire_at = now + *ttl;
            let secs = expire_at
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or(Duration::ZERO)
                .as_secs();
            Frame::Integer(secs as i64)
        }
        None => Frame::Integer(-1),
    }
}

/// PEXPIRETIME key
fn cmd_pexpiretime(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("pexpiretime"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(-2);
    }

    match db.ttl.get(key.as_ref()) {
        Some(ttl) => {
            let expire_at = now + *ttl;
            let ms = expire_at
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or(Duration::ZERO)
                .as_millis();
            Frame::Integer(ms as i64)
        }
        None => Frame::Integer(-1),
    }
}

/// COPY source destination [DB db] [REPLACE]
fn cmd_copy(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("copy"));
    }

    let src = String::from_utf8_lossy(&args[0]).into_owned();
    let dst = String::from_utf8_lossy(&args[1]).into_owned();
    let mut dest_db = ctx.selected_db;
    let mut replace = false;

    let mut i = 2;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "DB" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                match parse_int(&args[i]) {
                    Some(n) if (0..16).contains(&n) => dest_db = n as usize,
                    _ => return Frame::error(MSG_DB_INDEX_OUT_OF_RANGE),
                }
            }
            "REPLACE" => replace = true,
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
        i += 1;
    }

    let mut inner = state.lock();
    let now = inner.effective_now();

    // Check source exists
    {
        let src_db = inner.db_mut(ctx.selected_db);
        src_db.check_ttl(&src);
        if !src_db.keys.contains_key(&src) {
            return Frame::Integer(0);
        }
    }

    // Check destination
    {
        let dst_db = inner.db_mut(dest_db);
        dst_db.check_ttl(&dst);
        if dst_db.keys.contains_key(&dst) && !replace {
            return Frame::Integer(0);
        }
        if replace {
            dst_db.del(&dst);
        }
    }

    if ctx.selected_db == dest_db {
        // Same DB: use copy_key
        let db = inner.db_mut(ctx.selected_db);
        db.copy_key(&src, &dst, now);
    } else {
        // Cross-DB copy: manually clone data
        let key_type = *inner.db(ctx.selected_db).keys.get(&src).unwrap();
        let ttl = inner.db(ctx.selected_db).ttl.get(&src).copied();

        match key_type {
            KeyType::String => {
                let val = inner.db(ctx.selected_db).string_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner.db_mut(dest_db).string_set(&dst, v, now);
                }
            }
            KeyType::Hash => {
                let val = inner.db(ctx.selected_db).hash_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner
                        .db_mut(dest_db)
                        .keys
                        .insert(dst.clone(), KeyType::Hash);
                    inner.db_mut(dest_db).hash_keys.insert(dst.clone(), v);
                    inner.db_mut(dest_db).incr_version(&dst, now);
                }
            }
            KeyType::List => {
                let val = inner.db(ctx.selected_db).list_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner
                        .db_mut(dest_db)
                        .keys
                        .insert(dst.clone(), KeyType::List);
                    inner.db_mut(dest_db).list_keys.insert(dst.clone(), v);
                    inner.db_mut(dest_db).incr_version(&dst, now);
                }
            }
            KeyType::Set => {
                let val = inner.db(ctx.selected_db).set_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner.db_mut(dest_db).set_set(&dst, v, now);
                }
            }
            KeyType::SortedSet => {
                let val = inner.db(ctx.selected_db).sorted_set_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner.db_mut(dest_db).sset_set(&dst, v, now);
                }
            }
            KeyType::Stream => {
                let val = inner.db(ctx.selected_db).stream_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner
                        .db_mut(dest_db)
                        .keys
                        .insert(dst.clone(), KeyType::Stream);
                    inner.db_mut(dest_db).stream_keys.insert(dst.clone(), v);
                    inner.db_mut(dest_db).incr_version(&dst, now);
                }
            }
            KeyType::HyperLogLog => {
                let val = inner.db(ctx.selected_db).hll_keys.get(&src).cloned();
                if let Some(v) = val {
                    inner
                        .db_mut(dest_db)
                        .keys
                        .insert(dst.clone(), KeyType::HyperLogLog);
                    inner.db_mut(dest_db).hll_keys.insert(dst.clone(), v);
                    inner.db_mut(dest_db).incr_version(&dst, now);
                }
            }
        }

        if let Some(ttl) = ttl {
            inner.db_mut(dest_db).ttl.insert(dst, ttl);
        }
    }

    Frame::Integer(1)
}

/// MOVE key db
fn cmd_move(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("move"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let target_db = match parse_int(&args[1]) {
        Some(n) if (0..16).contains(&n) => n as usize,
        _ => return Frame::error(MSG_DB_INDEX_OUT_OF_RANGE),
    };

    if target_db == ctx.selected_db {
        return Frame::error("ERR source and destination objects are the same");
    }

    let mut inner = state.lock();
    let now = inner.effective_now();

    // Check source exists
    {
        let src_db = inner.db_mut(ctx.selected_db);
        src_db.check_ttl(&key);
        if !src_db.keys.contains_key(&key) {
            return Frame::Integer(0);
        }
    }

    // Check target doesn't have the key
    {
        let dst_db = inner.db_mut(target_db);
        dst_db.check_ttl(&key);
        if dst_db.keys.contains_key(&key) {
            return Frame::Integer(0);
        }
    }

    // Copy to target, then delete from source
    let key_type = *inner.db(ctx.selected_db).keys.get(&key).unwrap();
    let ttl = inner.db(ctx.selected_db).ttl.get(&key).copied();

    match key_type {
        KeyType::String => {
            let val = inner.db(ctx.selected_db).string_keys.get(&key).cloned();
            if let Some(v) = val {
                inner.db_mut(target_db).string_set(&key, v, now);
            }
        }
        KeyType::Hash => {
            let val = inner.db(ctx.selected_db).hash_keys.get(&key).cloned();
            if let Some(v) = val {
                inner
                    .db_mut(target_db)
                    .keys
                    .insert(key.clone(), KeyType::Hash);
                inner.db_mut(target_db).hash_keys.insert(key.clone(), v);
                inner.db_mut(target_db).incr_version(&key, now);
            }
        }
        KeyType::List => {
            let val = inner.db(ctx.selected_db).list_keys.get(&key).cloned();
            if let Some(v) = val {
                inner
                    .db_mut(target_db)
                    .keys
                    .insert(key.clone(), KeyType::List);
                inner.db_mut(target_db).list_keys.insert(key.clone(), v);
                inner.db_mut(target_db).incr_version(&key, now);
            }
        }
        KeyType::Set => {
            let val = inner.db(ctx.selected_db).set_keys.get(&key).cloned();
            if let Some(v) = val {
                inner.db_mut(target_db).set_set(&key, v, now);
            }
        }
        KeyType::SortedSet => {
            let val = inner.db(ctx.selected_db).sorted_set_keys.get(&key).cloned();
            if let Some(v) = val {
                inner.db_mut(target_db).sset_set(&key, v, now);
            }
        }
        KeyType::Stream => {
            let val = inner.db(ctx.selected_db).stream_keys.get(&key).cloned();
            if let Some(v) = val {
                inner
                    .db_mut(target_db)
                    .keys
                    .insert(key.clone(), KeyType::Stream);
                inner.db_mut(target_db).stream_keys.insert(key.clone(), v);
                inner.db_mut(target_db).incr_version(&key, now);
            }
        }
        KeyType::HyperLogLog => {
            let val = inner.db(ctx.selected_db).hll_keys.get(&key).cloned();
            if let Some(v) = val {
                inner
                    .db_mut(target_db)
                    .keys
                    .insert(key.clone(), KeyType::HyperLogLog);
                inner.db_mut(target_db).hll_keys.insert(key.clone(), v);
                inner.db_mut(target_db).incr_version(&key, now);
            }
        }
    }

    if let Some(ttl) = ttl {
        inner.db_mut(target_db).ttl.insert(key.clone(), ttl);
    }

    // Delete from source
    inner.db_mut(ctx.selected_db).del(&key);
    Frame::Integer(1)
}

/// DUMP key — stub: returns raw string value or null
fn cmd_dump(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("dump"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Null;
    }

    // Stub: only dump string values
    match db.key_type(&key) {
        Some(KeyType::String) => match db.string_get(&key) {
            Some(val) => Frame::Bulk(val.clone().into()),
            None => Frame::Null,
        },
        _ => Frame::Null,
    }
}

/// RESTORE key ttl serialized-value [REPLACE]
fn cmd_restore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 3 {
        return Frame::error(err_wrong_number("restore"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let ttl_ms: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let value = args[2].clone();

    let mut replace = false;
    for arg in &args[3..] {
        let opt = String::from_utf8_lossy(arg).to_uppercase();
        if opt == "REPLACE" {
            replace = true;
        }
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if db.keys.contains_key(&key) {
        if !replace {
            return Frame::error("BUSYKEY Target key name already exists.");
        }
        db.del(&key);
    }

    // Stub: store as string value
    db.string_set(&key, value, now);

    if ttl_ms > 0 {
        db.ttl.insert(key, Duration::from_millis(ttl_ms as u64));
    }

    Frame::ok()
}

// ── Pattern matching ─────────────────────────────────────────────────

/// Match keys against a glob-style pattern (like Redis KEYS/SCAN).
fn match_keys(keys: &[String], pattern: &str) -> Vec<String> {
    if pattern == "*" {
        return keys.to_vec();
    }

    keys.iter()
        .filter(|k| crate::keys::glob_match(pattern, k))
        .cloned()
        .collect()
}
