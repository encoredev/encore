use std::sync::Arc;

use super::parse_int;
use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INVALID_INT, MSG_INVALID_TIMEOUT, MSG_KEY_NOT_FOUND, MSG_OUT_OF_RANGE,
    MSG_SYNTAX_ERROR, MSG_TIMEOUT_IS_OUT_OF_RANGE, MSG_TIMEOUT_NEGATIVE, MSG_WRONG_TYPE,
    err_wrong_number,
};
use crate::frame::Frame;
use crate::types::KeyType;

pub fn register(table: &mut CommandTable) {
    table.add("LPUSH", cmd_lpush, false, -3);
    table.add("RPUSH", cmd_rpush, false, -3);
    table.add("LPUSHX", cmd_lpushx, false, -3);
    table.add("RPUSHX", cmd_rpushx, false, -3);
    table.add("LPOP", cmd_lpop, false, -2);
    table.add("RPOP", cmd_rpop, false, -2);
    table.add("LLEN", cmd_llen, true, 2);
    table.add("LINDEX", cmd_lindex, true, 3);
    table.add("LRANGE", cmd_lrange, true, 4);
    table.add("LSET", cmd_lset, false, 4);
    table.add("LINSERT", cmd_linsert, false, 5);
    table.add("LREM", cmd_lrem, false, 4);
    table.add("LTRIM", cmd_ltrim, false, 4);
    table.add("RPOPLPUSH", cmd_rpoplpush, false, 3);
    table.add("LMOVE", cmd_lmove, false, 5);
    table.add("LPOS", cmd_lpos, true, -3);
    // Blocking commands: registered for MULTI/EXEC queueing (non-blocking attempt)
    table.add("BLPOP", cmd_blpop, false, -3);
    table.add("BRPOP", cmd_brpop, false, -3);
    table.add("BRPOPLPUSH", cmd_brpoplpush, false, 4);
    table.add("BLMOVE", cmd_blmove, false, 6);
}

/// LPUSH key element [element ...]
fn cmd_lpush(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xpush(state, ctx, args, true, false)
}

/// RPUSH key element [element ...]
fn cmd_rpush(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xpush(state, ctx, args, false, false)
}

/// LPUSHX key element [element ...]
fn cmd_lpushx(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xpush(state, ctx, args, true, true)
}

/// RPUSHX key element [element ...]
fn cmd_rpushx(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xpush(state, ctx, args, false, true)
}

fn cmd_xpush(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    left: bool,
    only_existing: bool,
) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // PUSHX: only push to existing keys
    if only_existing && !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    let values: Vec<Vec<u8>> = args[1..].to_vec();
    let len = if left {
        db.list_lpush(&key, &values, now)
    } else {
        db.list_rpush(&key, &values, now)
    };

    Frame::Integer(len)
}

/// LPOP key [count]
fn cmd_lpop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xpop(state, ctx, args, true)
}

/// RPOP key [count]
fn cmd_rpop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xpop(state, ctx, args, false)
}

fn cmd_xpop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>], left: bool) -> Frame {
    let cmd_name = if left { "lpop" } else { "rpop" };

    if args.len() > 2 {
        return Frame::error(err_wrong_number(cmd_name));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let count = if args.len() > 1 {
        match parse_int(&args[1]) {
            Some(n) if n < 0 => return Frame::error(MSG_OUT_OF_RANGE),
            Some(n) => Some(n as usize),
            None => return Frame::error(MSG_INVALID_INT),
        }
    } else {
        None
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if !db.keys.contains_key(&key) {
        return if count.is_some() {
            Frame::NullArray
        } else {
            Frame::Null
        };
    }

    match count {
        Some(n) => {
            let mut results = Vec::new();
            for _ in 0..n {
                let val = if left {
                    db.list_lpop(&key, now)
                } else {
                    db.list_rpop(&key, now)
                };
                match val {
                    Some(v) => results.push(Frame::Bulk(v.into())),
                    None => break,
                }
            }
            Frame::Array(results)
        }
        None => {
            let val = if left {
                db.list_lpop(&key, now)
            } else {
                db.list_rpop(&key, now)
            };
            match val {
                Some(v) => Frame::Bulk(v.into()),
                None => Frame::Null,
            }
        }
    }
}

/// LLEN key
fn cmd_llen(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let len = db.list_keys.get(key.as_ref()).map(|l| l.len()).unwrap_or(0);
    Frame::Integer(len as i64)
}

/// LINDEX key index
fn cmd_lindex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    // Reject "-0" (Go miniredis compat)
    if args[1] == b"-0" {
        return Frame::error(MSG_INVALID_INT);
    }
    let index: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let list = match db.list_keys.get(key.as_ref()) {
        Some(l) => l,
        None => return Frame::Null,
    };

    let len = list.len() as i64;
    let mut idx = index;
    if idx < 0 {
        idx += len;
    }
    if idx < 0 || idx >= len {
        return Frame::Null;
    }

    Frame::Bulk(list[idx as usize].clone().into())
}

/// LRANGE key start stop
fn cmd_lrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
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
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let list = match db.list_keys.get(key.as_ref()) {
        Some(l) => l,
        None => return Frame::Array(vec![]),
    };

    let len = list.len() as i64;
    let (rs, re) = redis_range(start, end, len);
    if rs > re || rs >= len {
        return Frame::Array(vec![]);
    }

    let results: Vec<Frame> = (rs..=re)
        .map(|i| Frame::Bulk(list[i as usize].clone().into()))
        .collect();

    Frame::Array(results)
}

/// LSET key index element
fn cmd_lset(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let index: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let value = args[2].clone();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::error(MSG_KEY_NOT_FOUND);
    }

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let list = match db.list_keys.get_mut(&key) {
        Some(l) => l,
        None => return Frame::error(MSG_KEY_NOT_FOUND),
    };

    let len = list.len() as i64;
    let mut idx = index;
    if idx < 0 {
        idx += len;
    }
    if idx < 0 || idx >= len {
        return Frame::error(MSG_OUT_OF_RANGE);
    }

    list[idx as usize] = value;
    db.incr_version(&key, now);
    Frame::ok()
}

/// LINSERT key BEFORE|AFTER pivot element
fn cmd_linsert(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let position = String::from_utf8_lossy(&args[1]).to_uppercase();
    let before = match position.as_str() {
        "BEFORE" => true,
        "AFTER" => false,
        _ => return Frame::error(MSG_SYNTAX_ERROR),
    };
    let pivot = &args[2];
    let value = args[3].clone();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    let list = match db.list_keys.get_mut(&key) {
        Some(l) => l,
        None => return Frame::Integer(0),
    };

    // Find pivot
    let pos = list.iter().position(|el| el == pivot);
    match pos {
        Some(i) => {
            let insert_at = if before { i } else { i + 1 };
            list.insert(insert_at, value);
            let new_len = list.len() as i64;
            db.incr_version(&key, now);
            Frame::Integer(new_len)
        }
        None => Frame::Integer(-1),
    }
}

/// LREM key count element
fn cmd_lrem(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let count: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let element = &args[2];

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let list = match db.list_keys.get_mut(&key) {
        Some(l) => l,
        None => return Frame::Integer(0),
    };

    let mut removed = 0i64;
    let max_remove = if count == 0 {
        list.len()
    } else {
        count.unsigned_abs() as usize
    };

    if count >= 0 {
        // Remove from head
        let mut i = 0;
        while i < list.len() && (removed as usize) < max_remove {
            if &list[i] == element {
                list.remove(i);
                removed += 1;
            } else {
                i += 1;
            }
        }
    } else {
        // Remove from tail
        let mut i = list.len();
        while i > 0 && (removed as usize) < max_remove {
            i -= 1;
            if &list[i] == element {
                list.remove(i);
                removed += 1;
            }
        }
    }

    if list.is_empty() {
        db.del(&key);
    } else {
        db.incr_version(&key, now);
    }

    Frame::Integer(removed)
}

/// LTRIM key start stop
fn cmd_ltrim(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let start: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let end: i64 = match parse_int(&args[2]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if !db.keys.contains_key(&key) {
        return Frame::ok();
    }

    let list = match db.list_keys.get(&key) {
        Some(l) => l,
        None => return Frame::ok(),
    };

    let len = list.len() as i64;
    let (rs, re) = redis_range(start, end, len);

    if rs > re || rs >= len {
        db.del(&key);
        return Frame::ok();
    }

    let trimmed: std::collections::VecDeque<Vec<u8>> = list
        .iter()
        .skip(rs as usize)
        .take((re - rs + 1) as usize)
        .cloned()
        .collect();

    if trimmed.is_empty() {
        db.del(&key);
    } else {
        db.list_keys.insert(key.clone(), trimmed);
        db.incr_version(&key, now);
    }

    Frame::ok()
}

/// RPOPLPUSH source destination
fn cmd_rpoplpush(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let src = String::from_utf8_lossy(&args[0]).into_owned();
    let dst = String::from_utf8_lossy(&args[1]).into_owned();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&src);
    db.check_ttl(&dst);

    // Type checks
    if let Some(t) = db.key_type(&src)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }
    if let Some(t) = db.key_type(&dst)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // Save TTL when src == dst so we can restore it after pop+push cycle
    let saved_ttl = if src == dst {
        db.ttl.get(&src).cloned()
    } else {
        None
    };

    let val = match db.list_rpop(&src, now) {
        Some(v) => v,
        None => return Frame::Null,
    };

    db.list_lpush(&dst, std::slice::from_ref(&val), now);

    // Restore TTL if src == dst (pop may have deleted the key and its TTL)
    if let Some(ttl) = saved_ttl {
        db.ttl.insert(dst.clone(), ttl);
    }

    Frame::Bulk(val.into())
}

/// LMOVE source destination LEFT|RIGHT LEFT|RIGHT
fn cmd_lmove(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let src = String::from_utf8_lossy(&args[0]).into_owned();
    let dst = String::from_utf8_lossy(&args[1]).into_owned();
    let src_dir = String::from_utf8_lossy(&args[2]).to_uppercase();
    let dst_dir = String::from_utf8_lossy(&args[3]).to_uppercase();

    let pop_left = match src_dir.as_str() {
        "LEFT" => true,
        "RIGHT" => false,
        _ => return Frame::error(MSG_SYNTAX_ERROR),
    };
    let push_left = match dst_dir.as_str() {
        "LEFT" => true,
        "RIGHT" => false,
        _ => return Frame::error(MSG_SYNTAX_ERROR),
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&src);
    db.check_ttl(&dst);

    if let Some(t) = db.key_type(&src)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }
    if let Some(t) = db.key_type(&dst)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // Save TTL when src == dst so we can restore it after pop+push cycle
    let saved_ttl = if src == dst {
        db.ttl.get(&src).cloned()
    } else {
        None
    };

    let val = if pop_left {
        db.list_lpop(&src, now)
    } else {
        db.list_rpop(&src, now)
    };

    match val {
        Some(v) => {
            if push_left {
                db.list_lpush(&dst, std::slice::from_ref(&v), now);
            } else {
                db.list_rpush(&dst, std::slice::from_ref(&v), now);
            }
            // Restore TTL if src == dst (pop may have deleted the key and its TTL)
            if let Some(ttl) = saved_ttl {
                db.ttl.insert(dst.clone(), ttl);
            }
            Frame::Bulk(v.into())
        }
        None => Frame::Null,
    }
}

// ── Utility ──────────────────────────────────────────────────────────

/// LPOS key element [RANK rank] [COUNT count] [MAXLEN maxlen]
fn cmd_lpos(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let element = &args[1];
    let mut rank: i64 = 1;
    let mut count: Option<i64> = None;
    let mut maxlen: i64 = 0;

    let mut i = 2;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "RANK" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                match parse_int(&args[i]) {
                    Some(n) => {
                        if n == 0 {
                            return Frame::error(
                                "ERR RANK can't be zero: use 1 to start from the first match, 2 from the second ... or use negative values meaning from the last match",
                            );
                        }
                        rank = n;
                    }
                    None => return Frame::error(MSG_INVALID_INT),
                }
            }
            "COUNT" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                match parse_int(&args[i]) {
                    Some(n) if n >= 0 => count = Some(n),
                    _ => return Frame::error("ERR COUNT can't be negative"),
                }
            }
            "MAXLEN" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                match parse_int(&args[i]) {
                    Some(n) if n >= 0 => maxlen = n,
                    _ => return Frame::error("ERR MAXLEN can't be negative"),
                }
            }
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
        i += 1;
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let list = match db.list_keys.get(&key) {
        Some(l) => l,
        None => {
            return if count.is_some() {
                Frame::Array(vec![])
            } else {
                Frame::Null
            };
        }
    };

    let len = list.len();
    let max_count = count.unwrap_or(1);
    let scan_max = if maxlen > 0 { maxlen as usize } else { len };

    let mut matches: Vec<i64> = Vec::new();
    let mut match_count = 0i64;

    if rank > 0 {
        // Forward scan
        let mut skip = rank - 1;
        for (idx, item) in list.iter().enumerate().take(len.min(scan_max)) {
            if item == element {
                if skip > 0 {
                    skip -= 1;
                    continue;
                }
                matches.push(idx as i64);
                match_count += 1;
                if max_count > 0 && match_count >= max_count {
                    break;
                }
            }
        }
    } else {
        // Reverse scan
        let mut skip = (-rank) - 1;
        let start = len.saturating_sub(scan_max);
        for idx in (start..len).rev() {
            if &list[idx] == element {
                if skip > 0 {
                    skip -= 1;
                    continue;
                }
                matches.push(idx as i64);
                match_count += 1;
                if max_count > 0 && match_count >= max_count {
                    break;
                }
            }
        }
    }

    if count.is_some() {
        Frame::Array(matches.into_iter().map(Frame::Integer).collect())
    } else {
        matches
            .first()
            .map(|&idx| Frame::Integer(idx))
            .unwrap_or(Frame::Null)
    }
}

// ── Blocking command stubs (non-blocking for MULTI/EXEC) ─────────────

/// BLPOP key [key ...] timeout — non-blocking attempt (for MULTI/EXEC)
pub fn cmd_blpop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    // Last arg is timeout — validate it
    if let Some(err) = validate_timeout(&args[args.len() - 1]) {
        return err;
    }

    // Last arg is timeout, keys are all but last
    let keys = &args[..args.len() - 1];

    let mut inner = state.lock();
    let now = inner.effective_now();
    for key_bytes in keys {
        let key = String::from_utf8_lossy(key_bytes).into_owned();
        let db = inner.db_mut(ctx.selected_db);
        db.check_ttl(&key);

        if let Some(t) = db.key_type(&key)
            && t != KeyType::List
        {
            return Frame::error(MSG_WRONG_TYPE);
        }

        if let Some(val) = db.list_lpop(&key, now) {
            return Frame::Array(vec![Frame::Bulk(key.into()), Frame::Bulk(val.into())]);
        }
    }

    Frame::NullArray
}

/// BRPOP key [key ...] timeout — non-blocking attempt (for MULTI/EXEC)
pub fn cmd_brpop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    // Last arg is timeout — validate it
    if let Some(err) = validate_timeout(&args[args.len() - 1]) {
        return err;
    }

    let keys = &args[..args.len() - 1];

    let mut inner = state.lock();
    let now = inner.effective_now();
    for key_bytes in keys {
        let key = String::from_utf8_lossy(key_bytes).into_owned();
        let db = inner.db_mut(ctx.selected_db);
        db.check_ttl(&key);

        if let Some(t) = db.key_type(&key)
            && t != KeyType::List
        {
            return Frame::error(MSG_WRONG_TYPE);
        }

        if let Some(val) = db.list_rpop(&key, now) {
            return Frame::Array(vec![Frame::Bulk(key.into()), Frame::Bulk(val.into())]);
        }
    }

    Frame::NullArray
}

/// BRPOPLPUSH source destination timeout — non-blocking attempt
pub fn cmd_brpoplpush(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    // Last arg is timeout — validate it
    if let Some(err) = validate_timeout(&args[2]) {
        return err;
    }

    let src = String::from_utf8_lossy(&args[0]).into_owned();
    let dst = String::from_utf8_lossy(&args[1]).into_owned();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&src);
    db.check_ttl(&dst);

    if let Some(t) = db.key_type(&src)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }
    if let Some(t) = db.key_type(&dst)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.list_rpop(&src, now) {
        Some(val) => {
            db.list_lpush(&dst, std::slice::from_ref(&val), now);
            Frame::Bulk(val.into())
        }
        None => Frame::Null,
    }
}

/// BLMOVE source destination LEFT|RIGHT LEFT|RIGHT timeout — non-blocking attempt
pub fn cmd_blmove(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let src = String::from_utf8_lossy(&args[0]).into_owned();
    let dst = String::from_utf8_lossy(&args[1]).into_owned();
    let src_dir = String::from_utf8_lossy(&args[2]).to_uppercase();
    let dst_dir = String::from_utf8_lossy(&args[3]).to_uppercase();

    let pop_left = match src_dir.as_str() {
        "LEFT" => true,
        "RIGHT" => false,
        _ => return Frame::error(MSG_SYNTAX_ERROR),
    };
    let push_left = match dst_dir.as_str() {
        "LEFT" => true,
        "RIGHT" => false,
        _ => return Frame::error(MSG_SYNTAX_ERROR),
    };

    if let Some(err) = validate_timeout(&args[4]) {
        return err;
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&src);
    db.check_ttl(&dst);

    if let Some(t) = db.key_type(&src)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }
    if let Some(t) = db.key_type(&dst)
        && t != KeyType::List
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // Save TTL when src == dst so we can restore it after pop+push cycle
    let saved_ttl = if src == dst {
        db.ttl.get(&src).cloned()
    } else {
        None
    };

    let val = if pop_left {
        db.list_lpop(&src, now)
    } else {
        db.list_rpop(&src, now)
    };

    match val {
        Some(v) => {
            if push_left {
                db.list_lpush(&dst, std::slice::from_ref(&v), now);
            } else {
                db.list_rpush(&dst, std::slice::from_ref(&v), now);
            }
            // Restore TTL if src == dst
            if let Some(ttl) = saved_ttl {
                db.ttl.insert(dst.clone(), ttl);
            }
            Frame::Bulk(v.into())
        }
        None => Frame::Null,
    }
}

// ── Utility ──────────────────────────────────────────────────────────

/// Normalize Redis-style range indices for lists. Returns (start, end) inclusive.
fn redis_range(start: i64, end: i64, len: i64) -> (i64, i64) {
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

/// Validate a blocking command timeout argument.
/// Returns Some(Frame) with error if invalid, None if OK.
fn validate_timeout(arg: &[u8]) -> Option<Frame> {
    let s = String::from_utf8_lossy(arg);
    let s_lower = s.to_lowercase();
    if s_lower == "inf" || s_lower == "+inf" || s_lower == "-inf" {
        return Some(Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE));
    }
    match s.parse::<f64>() {
        Ok(t) if t < 0.0 => Some(Frame::error(MSG_TIMEOUT_NEGATIVE)),
        Ok(_) => None,
        Err(_) => Some(Frame::error(MSG_INVALID_TIMEOUT)),
    }
}
