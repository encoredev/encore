use std::sync::Arc;
use std::time::Duration;

use super::parse_int;
use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INT_OVERFLOW, MSG_INVALID_FLOAT, MSG_INVALID_INT, MSG_INVALID_PSETEX_TIME,
    MSG_INVALID_SE_TIME, MSG_INVALID_SETEX_TIME, MSG_SYNTAX_ERROR, MSG_WRONG_TYPE, MSG_XX_AND_NX,
    err_wrong_number,
};
use crate::frame::Frame;
use crate::types::KeyType;

pub fn register(table: &mut CommandTable) {
    table.add("GET", cmd_get, true);
    table.add("SET", cmd_set, false);
    table.add("SETNX", cmd_setnx, false);
    table.add("GETSET", cmd_getset, false);
    table.add("SETEX", cmd_setex, false);
    table.add("PSETEX", cmd_psetex, false);
    table.add("MGET", cmd_mget, true);
    table.add("MSET", cmd_mset, false);
    table.add("MSETNX", cmd_msetnx, false);
    table.add("INCR", cmd_incr, false);
    table.add("INCRBY", cmd_incrby, false);
    table.add("INCRBYFLOAT", cmd_incrbyfloat, false);
    table.add("DECR", cmd_decr, false);
    table.add("DECRBY", cmd_decrby, false);
    table.add("STRLEN", cmd_strlen, true);
    table.add("APPEND", cmd_append, false);
    table.add("GETRANGE", cmd_getrange, true);
    table.add("SUBSTR", cmd_getrange, true); // alias
    table.add("SETRANGE", cmd_setrange, false);
    table.add("GETDEL", cmd_getdel, false);
    table.add("GETEX", cmd_getex, false);
    table.add("GETBIT", cmd_getbit, true);
    table.add("SETBIT", cmd_setbit, false);
    table.add("BITCOUNT", cmd_bitcount, true);
    table.add("BITOP", cmd_bitop, false);
    table.add("BITPOS", cmd_bitpos, true);
}

// ── Helpers ──────────────────────────────────────────────────────────

fn string_incr(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    key: &str,
    delta: i64,
) -> Result<i64, Frame> {
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(key);

    // Check type
    if let Some(t) = db.key_type(key)
        && t != KeyType::String
    {
        return Err(Frame::error(MSG_WRONG_TYPE));
    }

    let current: i64 = match db.string_get(key) {
        Some(v) => match String::from_utf8_lossy(v).parse::<i64>() {
            Ok(n) => n,
            Err(_) => return Err(Frame::error(MSG_INVALID_INT)),
        },
        None => 0,
    };

    let new_val = match current.checked_add(delta) {
        Some(n) => n,
        None => return Err(Frame::error(MSG_INT_OVERFLOW)),
    };

    db.string_set(key, new_val.to_string().into_bytes(), now);
    Ok(new_val)
}

// ── Commands ─────────────────────────────────────────────────────────

/// GET key
fn cmd_get(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("get"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.string_get(&key) {
        Some(val) => Frame::Bulk(val.clone().into()),
        None => Frame::Null,
    }
}

/// SET key value [EX seconds] [PX milliseconds] [NX|XX] [KEEPTTL] [GET]
fn cmd_set(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("set"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let value = args[1].clone();

    let mut ex: Option<Duration> = None;
    let mut nx = false;
    let mut xx = false;
    let mut keepttl = false;
    let mut get = false;

    let mut i = 2;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "EX" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let secs: i64 = match String::from_utf8_lossy(&args[i]).parse() {
                    Ok(n) => n,
                    Err(_) => return Frame::error(MSG_INVALID_INT),
                };
                if secs <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                ex = Some(Duration::from_secs(secs as u64));
            }
            "PX" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let ms: i64 = match String::from_utf8_lossy(&args[i]).parse() {
                    Ok(n) => n,
                    Err(_) => return Frame::error(MSG_INVALID_INT),
                };
                if ms <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                ex = Some(Duration::from_millis(ms as u64));
            }
            "EXAT" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let ts: i64 = match String::from_utf8_lossy(&args[i]).parse() {
                    Ok(n) => n,
                    Err(_) => return Frame::error(MSG_INVALID_INT),
                };
                if ts <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                let inner = state.lock();
                let now = inner.effective_now();
                let target = std::time::UNIX_EPOCH + Duration::from_secs(ts as u64);
                match target.duration_since(now) {
                    Ok(d) => ex = Some(d),
                    Err(_) => ex = Some(Duration::ZERO),
                }
                drop(inner);
            }
            "PXAT" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let ts: i64 = match String::from_utf8_lossy(&args[i]).parse() {
                    Ok(n) => n,
                    Err(_) => return Frame::error(MSG_INVALID_INT),
                };
                if ts <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                let inner = state.lock();
                let now = inner.effective_now();
                let target = std::time::UNIX_EPOCH + Duration::from_millis(ts as u64);
                match target.duration_since(now) {
                    Ok(d) => ex = Some(d),
                    Err(_) => ex = Some(Duration::ZERO),
                }
                drop(inner);
            }
            "NX" => nx = true,
            "XX" => xx = true,
            "KEEPTTL" => keepttl = true,
            "GET" => get = true,
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
        i += 1;
    }

    if nx && xx {
        return Frame::error(MSG_XX_AND_NX);
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    let old_value = if get {
        match db.key_type(&key) {
            Some(KeyType::String) => db.string_get(&key).map(|v| Frame::Bulk(v.clone().into())),
            Some(_) => return Frame::error(MSG_WRONG_TYPE),
            None => Some(Frame::Null),
        }
    } else {
        None
    };

    let key_exists = db.keys.contains_key(&key);

    if nx && key_exists {
        return old_value.unwrap_or(Frame::Null);
    }
    if xx && !key_exists {
        return old_value.unwrap_or(Frame::Null);
    }

    let old_ttl = if keepttl {
        db.ttl.get(&key).copied()
    } else {
        None
    };

    db.string_set(&key, value, now);

    if let Some(ttl) = ex {
        db.ttl.insert(key.clone(), ttl);
    } else if let Some(old_ttl) = old_ttl {
        db.ttl.insert(key.clone(), old_ttl);
    } else {
        db.ttl.remove(&key);
    }

    old_value.unwrap_or(Frame::ok())
}

/// SETNX key value
fn cmd_setnx(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("setnx"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let value = args[1].clone();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    db.string_set(&key, value, now);
    db.ttl.remove(&key);
    Frame::Integer(1)
}

/// SETEX key seconds value
fn cmd_setex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("setex"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let secs: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    if secs <= 0 {
        return Frame::error(MSG_INVALID_SETEX_TIME);
    }
    let value = args[2].clone();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.del(&key);
    db.string_set(&key, value, now);
    db.ttl.insert(key, Duration::from_secs(secs as u64));
    Frame::ok()
}

/// PSETEX key milliseconds value
fn cmd_psetex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("psetex"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let ms: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    if ms <= 0 {
        return Frame::error(MSG_INVALID_PSETEX_TIME);
    }
    let value = args[2].clone();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.del(&key);
    db.string_set(&key, value, now);
    db.ttl.insert(key, Duration::from_millis(ms as u64));
    Frame::ok()
}

/// GETSET key value
fn cmd_getset(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("getset"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let value = args[1].clone();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let old = db
        .string_get(&key)
        .map(|v| Frame::Bulk(v.clone().into()))
        .unwrap_or(Frame::Null);

    db.string_set(&key, value, now);
    db.ttl.remove(&key);
    old
}

/// MGET key [key ...]
fn cmd_mget(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("mget"));
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    let mut results = Vec::with_capacity(args.len());

    for arg in args {
        let key = String::from_utf8_lossy(arg);
        db.check_ttl(&key);
        match db.key_type(&key) {
            Some(KeyType::String) => {
                if let Some(val) = db.string_get(&key) {
                    results.push(Frame::Bulk(val.clone().into()));
                } else {
                    results.push(Frame::Null);
                }
            }
            _ => results.push(Frame::Null),
        }
    }

    Frame::Array(results)
}

/// MSET key value [key value ...]
fn cmd_mset(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 || !args.len().is_multiple_of(2) {
        return Frame::error(err_wrong_number("mset"));
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    for pair in args.chunks_exact(2) {
        let key = String::from_utf8_lossy(&pair[0]).into_owned();
        let value = pair[1].clone();
        db.del(&key);
        db.string_set(&key, value, now);
    }

    Frame::ok()
}

/// MSETNX key value [key value ...]
fn cmd_msetnx(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 || !args.len().is_multiple_of(2) {
        return Frame::error(err_wrong_number("msetnx"));
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    // Check if ANY key already exists
    for pair in args.chunks_exact(2) {
        let key = String::from_utf8_lossy(&pair[0]);
        if db.keys.contains_key(key.as_ref()) {
            return Frame::Integer(0);
        }
    }

    // Set all
    for pair in args.chunks_exact(2) {
        let key = String::from_utf8_lossy(&pair[0]).into_owned();
        let value = pair[1].clone();
        db.string_set(&key, value, now);
    }

    Frame::Integer(1)
}

/// INCR key
fn cmd_incr(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("incr"));
    }
    let key = String::from_utf8_lossy(&args[0]);
    match string_incr(state, ctx, &key, 1) {
        Ok(n) => Frame::Integer(n),
        Err(f) => f,
    }
}

/// INCRBY key increment
fn cmd_incrby(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("incrby"));
    }
    let key = String::from_utf8_lossy(&args[0]);
    let delta: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    match string_incr(state, ctx, &key, delta) {
        Ok(n) => Frame::Integer(n),
        Err(f) => f,
    }
}

/// INCRBYFLOAT key increment
fn cmd_incrbyfloat(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("incrbyfloat"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let delta: f64 = match String::from_utf8_lossy(&args[1]).parse() {
        Ok(n) => n,
        Err(_) => return Frame::error(MSG_INVALID_FLOAT),
    };
    if delta.is_nan() || delta.is_infinite() {
        return Frame::error(MSG_INVALID_FLOAT);
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let current: f64 = match db.string_get(&key) {
        Some(v) => match String::from_utf8_lossy(v).parse::<f64>() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_FLOAT),
        },
        None => 0.0,
    };

    let new_val = current + delta;
    if new_val.is_infinite() {
        return Frame::error(MSG_INT_OVERFLOW);
    }

    // Format like Redis: remove trailing zeros, but always have at least one decimal
    let formatted = format_float(new_val);
    db.string_set(&key, formatted.as_bytes().to_vec(), now);
    Frame::Bulk(formatted.into_bytes().into())
}

/// DECR key
fn cmd_decr(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("decr"));
    }
    let key = String::from_utf8_lossy(&args[0]);
    match string_incr(state, ctx, &key, -1) {
        Ok(n) => Frame::Integer(n),
        Err(f) => f,
    }
}

/// DECRBY key decrement
fn cmd_decrby(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("decrby"));
    }
    let key = String::from_utf8_lossy(&args[0]);
    let delta: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    match string_incr(state, ctx, &key, -delta) {
        Ok(n) => Frame::Integer(n),
        Err(f) => f,
    }
}

/// STRLEN key
fn cmd_strlen(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("strlen"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.string_get(&key) {
        Some(val) => Frame::Integer(val.len() as i64),
        None => Frame::Integer(0),
    }
}

/// APPEND key value
fn cmd_append(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("append"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let value = &args[1];

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut current = db.string_get(&key).cloned().unwrap_or_default();
    current.extend_from_slice(value);
    let new_len = current.len() as i64;
    db.string_set(&key, current, now);
    Frame::Integer(new_len)
}

/// GETRANGE key start end (also aliased as SUBSTR)
fn cmd_getrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("getrange"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let start: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let end: i64 = match parse_int(&args[2]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let val = match db.string_get(&key) {
        Some(v) => v.clone(),
        None => return Frame::Bulk(bytes::Bytes::new()),
    };

    let len = val.len() as i64;
    let (rs, re) = redis_range(start, end, len, true);
    if rs > re || rs >= len {
        return Frame::Bulk(bytes::Bytes::new());
    }

    Frame::Bulk(val[rs as usize..=re as usize].to_vec().into())
}

/// SETRANGE key offset value
fn cmd_setrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("setrange"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let offset: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    if offset < 0 {
        return Frame::error("ERR offset is out of range");
    }
    let offset = offset as usize;
    let replacement = &args[2];

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut val = db.string_get(&key).cloned().unwrap_or_default();

    // Extend with zeros if needed
    let needed = offset + replacement.len();
    if val.len() < needed {
        val.resize(needed, 0);
    }

    // Copy replacement bytes
    val[offset..offset + replacement.len()].copy_from_slice(replacement);
    let new_len = val.len() as i64;
    db.string_set(&key, val, now);
    Frame::Integer(new_len)
}

/// GETDEL key
fn cmd_getdel(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("getdel"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Null;
    }

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let val = db
        .string_get(&key)
        .map(|v| Frame::Bulk(v.clone().into()))
        .unwrap_or(Frame::Null);

    db.del(&key);
    val
}

/// GETEX key [PERSIST | EX seconds | PX ms | EXAT ts | PXAT ts]
fn cmd_getex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("getex"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();

    // Parse options
    let mut persist = false;
    let mut ex: Option<Duration> = None;

    if args.len() > 1 {
        let opt = String::from_utf8_lossy(&args[1]).to_uppercase();
        match opt.as_str() {
            "PERSIST" => {
                if args.len() != 2 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                persist = true;
            }
            "EX" => {
                if args.len() != 3 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let secs: i64 = match parse_int(&args[2]) {
                    Some(n) => n,
                    None => return Frame::error(MSG_INVALID_INT),
                };
                if secs <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                ex = Some(Duration::from_secs(secs as u64));
            }
            "PX" => {
                if args.len() != 3 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let ms: i64 = match parse_int(&args[2]) {
                    Some(n) => n,
                    None => return Frame::error(MSG_INVALID_INT),
                };
                if ms <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                ex = Some(Duration::from_millis(ms as u64));
            }
            "EXAT" => {
                if args.len() != 3 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let ts: i64 = match parse_int(&args[2]) {
                    Some(n) => n,
                    None => return Frame::error(MSG_INVALID_INT),
                };
                if ts <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                let inner = state.lock();
                let now = inner.effective_now();
                let target = std::time::UNIX_EPOCH + Duration::from_secs(ts as u64);
                ex = Some(target.duration_since(now).unwrap_or(Duration::ZERO));
                drop(inner);
            }
            "PXAT" => {
                if args.len() != 3 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                let ts: i64 = match parse_int(&args[2]) {
                    Some(n) => n,
                    None => return Frame::error(MSG_INVALID_INT),
                };
                if ts <= 0 {
                    return Frame::error(MSG_INVALID_SE_TIME);
                }
                let inner = state.lock();
                let now = inner.effective_now();
                let target = std::time::UNIX_EPOCH + Duration::from_millis(ts as u64);
                ex = Some(target.duration_since(now).unwrap_or(Duration::ZERO));
                drop(inner);
            }
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Null;
    }

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // Apply TTL changes
    if persist {
        db.ttl.remove(&key);
    } else if let Some(ttl) = ex {
        db.ttl.insert(key.clone(), ttl);
    }

    match db.string_get(&key) {
        Some(val) => Frame::Bulk(val.clone().into()),
        None => Frame::Null,
    }
}

// ── Utility functions ────────────────────────────────────────────────

/// Normalize Redis-style range indices. Returns (start, end) inclusive.
/// `string_mode`: for GETRANGE, the range is inclusive and never returns negative spans.
fn redis_range(start: i64, end: i64, len: i64, string_mode: bool) -> (i64, i64) {
    let mut s = start;
    let mut e = end;

    if s < 0 {
        s += len;
    }
    if e < 0 {
        e += len;
    }

    if s < 0 {
        s = 0;
    }
    if e < 0 {
        e = 0;
    }

    if string_mode && e >= len {
        e = len - 1;
    }

    (s, e)
}

/// Format a float value the way Redis does.
pub fn format_float(v: f64) -> String {
    if v == 0.0 && v.is_sign_negative() {
        return "0".to_string();
    }
    // Use ryu for fast formatting, then strip trailing zeros after decimal point
    let mut buf = ryu::Buffer::new();
    let s = buf.format(v);

    // ryu uses 'e' notation for very large/small numbers; check for that
    if s.contains('e') || s.contains('E') {
        // Fall back to standard formatting
        return format!("{}", v);
    }

    if s.contains('.') {
        let trimmed = s.trim_end_matches('0');
        if trimmed.ends_with('.') {
            // Keep at least one decimal place (like Redis does for INCRBYFLOAT)
            // Actually Redis removes trailing zeros completely: "3.0" -> "3"
            // But "3.14000" -> "3.14"
            // And "3.0" -> "3" not "3.0"
            return trimmed.trim_end_matches('.').to_string();
        }
        return trimmed.to_string();
    }

    s.to_string()
}

// ── Bit operations ───────────────────────────────────────────────────

/// GETBIT key offset
fn cmd_getbit(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("getbit"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let offset: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error("ERR bit offset is not an integer or out of range"),
    };
    if offset < 0 {
        return Frame::error("ERR bit offset is not an integer or out of range");
    }
    let offset = offset as usize;

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let val = db.string_get(&key).cloned().unwrap_or_default();
    let byte_idx = offset / 8;
    let bit_idx = 7 - (offset % 8);

    if byte_idx >= val.len() {
        return Frame::Integer(0);
    }

    let bit = (val[byte_idx] >> bit_idx) & 1;
    Frame::Integer(bit as i64)
}

/// SETBIT key offset value
fn cmd_setbit(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("setbit"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let offset: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error("ERR bit offset is not an integer or out of range"),
    };
    if offset < 0 {
        return Frame::error("ERR bit offset is not an integer or out of range");
    }
    let offset = offset as usize;

    let bit_val: i64 = match parse_int(&args[2]) {
        Some(n) => n,
        None => return Frame::error("ERR bit is not an integer or out of range"),
    };
    if bit_val != 0 && bit_val != 1 {
        return Frame::error("ERR bit is not an integer or out of range");
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut val = db.string_get(&key).cloned().unwrap_or_default();
    let byte_idx = offset / 8;
    let bit_idx = 7 - (offset % 8);

    // Expand if needed
    if byte_idx >= val.len() {
        val.resize(byte_idx + 1, 0);
    }

    let old_bit = (val[byte_idx] >> bit_idx) & 1;

    if bit_val == 1 {
        val[byte_idx] |= 1 << bit_idx;
    } else {
        val[byte_idx] &= !(1 << bit_idx);
    }

    db.string_set(&key, val, now);
    Frame::Integer(old_bit as i64)
}

/// BITCOUNT key [start end [BYTE|BIT]]
fn cmd_bitcount(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("bitcount"));
    }

    let key = String::from_utf8_lossy(&args[0]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let val = db.string_get(&key).cloned().unwrap_or_default();

    if args.len() == 1 {
        // Count all bits
        let count: u32 = val.iter().map(|b| b.count_ones()).sum();
        return Frame::Integer(count as i64);
    }

    if args.len() < 3 {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let start: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let end: i64 = match parse_int(&args[2]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    let bit_mode = if args.len() > 3 {
        let mode = String::from_utf8_lossy(&args[3]).to_uppercase();
        match mode.as_str() {
            "BYTE" => false,
            "BIT" => true,
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    } else {
        false
    };

    if bit_mode {
        let bit_len = val.len() as i64 * 8;
        let (rs, re) = bitcount_range(start, end, bit_len);
        if rs > re {
            return Frame::Integer(0);
        }
        let mut count = 0u32;
        for i in rs..=re {
            let byte_idx = (i / 8) as usize;
            let bit_idx = 7 - (i % 8) as usize;
            if byte_idx < val.len() && (val[byte_idx] >> bit_idx) & 1 == 1 {
                count += 1;
            }
        }
        Frame::Integer(count as i64)
    } else {
        let byte_len = val.len() as i64;
        let (rs, re) = bitcount_range(start, end, byte_len);
        if rs > re {
            return Frame::Integer(0);
        }
        let count: u32 = val[rs as usize..=re as usize]
            .iter()
            .map(|b| b.count_ones())
            .sum();
        Frame::Integer(count as i64)
    }
}

fn bitcount_range(start: i64, end: i64, len: i64) -> (i64, i64) {
    let mut s = start;
    let mut e = end;
    if s < 0 {
        s += len;
    }
    if e < 0 {
        e += len;
    }
    if s < 0 {
        s = 0;
    }
    if e >= len {
        e = len - 1;
    }
    (s, e)
}

/// BITOP operation destkey key [key ...]
fn cmd_bitop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 3 {
        return Frame::error(err_wrong_number("bitop"));
    }

    let op = String::from_utf8_lossy(&args[0]).to_uppercase();
    let dest = String::from_utf8_lossy(&args[1]).into_owned();
    let src_keys: Vec<String> = args[2..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();

    if op == "NOT" && src_keys.len() != 1 {
        return Frame::error("ERR BITOP NOT requires one and only one key");
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    // Collect all source values
    let mut values: Vec<Vec<u8>> = Vec::new();
    let mut max_len = 0;
    for key in &src_keys {
        db.check_ttl(key);
        if let Some(t) = db.key_type(key)
            && t != KeyType::String
        {
            return Frame::error(MSG_WRONG_TYPE);
        }
        let val = db.string_get(key).cloned().unwrap_or_default();
        if val.len() > max_len {
            max_len = val.len();
        }
        values.push(val);
    }

    let mut result = vec![0u8; max_len];
    match op.as_str() {
        "AND" => {
            if !values.is_empty() {
                result = vec![0xFF; max_len];
                for val in &values {
                    for i in 0..max_len {
                        let b = if i < val.len() { val[i] } else { 0 };
                        result[i] &= b;
                    }
                }
            }
        }
        "OR" => {
            for val in &values {
                for i in 0..max_len {
                    let b = if i < val.len() { val[i] } else { 0 };
                    result[i] |= b;
                }
            }
        }
        "XOR" => {
            for val in &values {
                for i in 0..max_len {
                    let b = if i < val.len() { val[i] } else { 0 };
                    result[i] ^= b;
                }
            }
        }
        "NOT" => {
            for i in 0..max_len {
                let b = if i < values[0].len() { values[0][i] } else { 0 };
                result[i] = !b;
            }
        }
        _ => return Frame::error(MSG_SYNTAX_ERROR),
    }

    let len = result.len() as i64;
    db.string_set(&dest, result, now);
    Frame::Integer(len)
}

/// BITPOS key bit [start [end [BYTE|BIT]]]
fn cmd_bitpos(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("bitpos"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let target_bit: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    if target_bit != 0 && target_bit != 1 {
        return Frame::error("ERR The bit argument must be 1 or 0.");
    }
    let target = target_bit as u8;

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::String
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let val = db.string_get(&key).cloned().unwrap_or_default();
    if val.is_empty() {
        // Empty key: if looking for 0, return 0 (Redis returns 0 for BITPOS of 0 on empty).
        // If looking for 1, return -1.
        return if target == 0 {
            Frame::Integer(0)
        } else {
            Frame::Integer(-1)
        };
    }

    let has_range = args.len() > 2;
    let has_end = args.len() > 3;
    let bit_mode = if args.len() > 4 {
        let mode = String::from_utf8_lossy(&args[4]).to_uppercase();
        match mode.as_str() {
            "BYTE" => false,
            "BIT" => true,
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    } else {
        false
    };

    let byte_len = val.len() as i64;
    let bit_len = byte_len * 8;

    if bit_mode {
        let start = if args.len() > 2 {
            match parse_int(&args[2]) {
                Some(n) => n,
                None => return Frame::error(MSG_INVALID_INT),
            }
        } else {
            0
        };
        let end = if args.len() > 3 {
            match parse_int(&args[3]) {
                Some(n) => n,
                None => return Frame::error(MSG_INVALID_INT),
            }
        } else {
            bit_len - 1
        };

        let (rs, re) = bitcount_range(start, end, bit_len);
        if rs > re {
            return Frame::Integer(-1);
        }
        for i in rs..=re {
            let byte_idx = (i / 8) as usize;
            let bit_idx = 7 - (i % 8) as usize;
            let bit = if byte_idx < val.len() {
                (val[byte_idx] >> bit_idx) & 1
            } else {
                0
            };
            if bit == target {
                return Frame::Integer(i);
            }
        }
        Frame::Integer(-1)
    } else {
        // BYTE mode
        let start = if args.len() > 2 {
            match parse_int(&args[2]) {
                Some(n) => n,
                None => return Frame::error(MSG_INVALID_INT),
            }
        } else {
            0
        };
        let end = if args.len() > 3 {
            match parse_int(&args[3]) {
                Some(n) => n,
                None => return Frame::error(MSG_INVALID_INT),
            }
        } else {
            byte_len - 1
        };

        let (rs, re) = bitcount_range(start, end, byte_len);
        if rs > re {
            return Frame::Integer(-1);
        }

        for byte_idx in rs..=re {
            let b = val[byte_idx as usize];
            for bit_idx in 0..8 {
                let bit = (b >> (7 - bit_idx)) & 1;
                if bit == target {
                    return Frame::Integer(byte_idx * 8 + bit_idx);
                }
            }
        }

        // If looking for 0 and no end was specified, the 0 bit is at end+1
        if target == 0 && !has_end && !has_range {
            return Frame::Integer(bit_len);
        }
        if target == 0 && has_range && !has_end {
            return Frame::Integer(bit_len);
        }

        Frame::Integer(-1)
    }
}
