use std::collections::HashSet;
use std::sync::Arc;

use rand::Rng;
use rand::seq::SliceRandom;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INVALID_CURSOR, MSG_INVALID_INT, MSG_INVALID_KEYS_NUMBER, MSG_OUT_OF_RANGE,
    MSG_SYNTAX_ERROR, MSG_WRONG_TYPE, err_wrong_number,
};
use crate::frame::Frame;
use crate::types::KeyType;

use super::parse_int;

pub fn register(table: &mut CommandTable) {
    table.add("SADD", cmd_sadd, false);
    table.add("SREM", cmd_srem, false);
    table.add("SCARD", cmd_scard, true);
    table.add("SMEMBERS", cmd_smembers, true);
    table.add("SISMEMBER", cmd_sismember, true);
    table.add("SMISMEMBER", cmd_smismember, true);
    table.add("SDIFF", cmd_sdiff, true);
    table.add("SDIFFSTORE", cmd_sdiffstore, false);
    table.add("SINTER", cmd_sinter, true);
    table.add("SINTERSTORE", cmd_sinterstore, false);
    table.add("SINTERCARD", cmd_sintercard, true);
    table.add("SUNION", cmd_sunion, true);
    table.add("SUNIONSTORE", cmd_sunionstore, false);
    table.add("SMOVE", cmd_smove, false);
    table.add("SPOP", cmd_spop, false);
    table.add("SRANDMEMBER", cmd_srandmember, true);
    table.add("SSCAN", cmd_sscan, true);
}

/// SADD key member [member ...]
fn cmd_sadd(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("sadd"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let members: Vec<String> = args[1..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();

    let added = db.set_add(&key, &members, now);
    Frame::Integer(added)
}

/// SREM key member [member ...]
fn cmd_srem(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("srem"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    let members: Vec<String> = args[1..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();

    let removed = db.set_rem(&key, &members, now);
    Frame::Integer(removed)
}

/// SCARD key
fn cmd_scard(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("scard"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let count = db.set_keys.get(key.as_ref()).map(|s| s.len()).unwrap_or(0);
    Frame::Integer(count as i64)
}

/// SMEMBERS key
fn cmd_smembers(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("smembers"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let members = db.set_members(&key);
    let items: Vec<Frame> = members.into_iter().map(|m| Frame::Bulk(m.into())).collect();

    if ctx.resp3 {
        Frame::Set(items)
    } else {
        Frame::Array(items)
    }
}

/// SISMEMBER key member
fn cmd_sismember(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("sismember"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let member = String::from_utf8_lossy(&args[1]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if db.set_is_member(&key, &member) {
        Frame::Integer(1)
    } else {
        Frame::Integer(0)
    }
}

/// SMISMEMBER key member [member ...]
fn cmd_smismember(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("smismember"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let results: Vec<Frame> = args[1..]
        .iter()
        .map(|a| {
            let member = String::from_utf8_lossy(a);
            if db.set_is_member(&key, &member) {
                Frame::Integer(1)
            } else {
                Frame::Integer(0)
            }
        })
        .collect();

    Frame::Array(results)
}

// ── Set operations (diff, inter, union) ──────────────────────────────

enum SetOp {
    Diff,
    Inter,
    Union,
}

fn set_op(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    keys: &[String],
    op: SetOp,
) -> Result<HashSet<String>, Frame> {
    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    for key in keys {
        if let Some(t) = db.key_type(key)
            && t != KeyType::Set
        {
            return Err(Frame::error(MSG_WRONG_TYPE));
        }
    }

    if keys.is_empty() {
        return Ok(HashSet::new());
    }

    match op {
        SetOp::Diff => {
            let first = db.set_keys.get(&keys[0]).cloned().unwrap_or_default();
            let mut result: HashSet<String> = first;
            for key in &keys[1..] {
                if let Some(other) = db.set_keys.get(key) {
                    result = result.difference(other).cloned().collect();
                }
            }
            Ok(result)
        }
        SetOp::Inter => {
            for key in keys {
                if !db.keys.contains_key(key) {
                    return Ok(HashSet::new());
                }
            }
            let first = db.set_keys.get(&keys[0]).cloned().unwrap_or_default();
            let mut result: HashSet<String> = first;
            for key in &keys[1..] {
                if let Some(other) = db.set_keys.get(key) {
                    result = result.intersection(other).cloned().collect();
                } else {
                    return Ok(HashSet::new());
                }
            }
            Ok(result)
        }
        SetOp::Union => {
            let mut result = HashSet::new();
            for key in keys {
                if let Some(set) = db.set_keys.get(key) {
                    result = result.union(set).cloned().collect();
                }
            }
            Ok(result)
        }
    }
}

fn set_to_frame(set: &HashSet<String>, resp3: bool) -> Frame {
    let mut members: Vec<String> = set.iter().cloned().collect();
    members.sort();
    let items: Vec<Frame> = members.into_iter().map(|m| Frame::Bulk(m.into())).collect();
    if resp3 {
        Frame::Set(items)
    } else {
        Frame::Array(items)
    }
}

fn cmd_set_op(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    cmd_name: &str,
    op: SetOp,
) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number(cmd_name));
    }
    let keys: Vec<String> = args
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();
    match set_op(state, ctx, &keys, op) {
        Ok(set) => set_to_frame(&set, ctx.resp3),
        Err(e) => e,
    }
}

fn cmd_set_store(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    cmd_name: &str,
    op: SetOp,
) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number(cmd_name));
    }
    let dest = String::from_utf8_lossy(&args[0]).into_owned();
    let keys: Vec<String> = args[1..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();
    let result = match set_op(state, ctx, &keys, op) {
        Ok(set) => set,
        Err(e) => return e,
    };
    let count = result.len() as i64;
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.del(&dest);
    if !result.is_empty() {
        db.set_set(&dest, result, now);
    }
    Frame::Integer(count)
}

/// SDIFF key [key ...]
fn cmd_sdiff(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_set_op(state, ctx, args, "sdiff", SetOp::Diff)
}

/// SDIFFSTORE destination key [key ...]
fn cmd_sdiffstore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_set_store(state, ctx, args, "sdiffstore", SetOp::Diff)
}

/// SINTER key [key ...]
fn cmd_sinter(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_set_op(state, ctx, args, "sinter", SetOp::Inter)
}

/// SINTERSTORE destination key [key ...]
fn cmd_sinterstore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_set_store(state, ctx, args, "sinterstore", SetOp::Inter)
}

/// SUNION key [key ...]
fn cmd_sunion(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_set_op(state, ctx, args, "sunion", SetOp::Union)
}

/// SUNIONSTORE destination key [key ...]
fn cmd_sunionstore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_set_store(state, ctx, args, "sunionstore", SetOp::Union)
}

/// SMOVE source destination member
fn cmd_smove(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("smove"));
    }

    let src = String::from_utf8_lossy(&args[0]).into_owned();
    let dst = String::from_utf8_lossy(&args[1]).into_owned();
    let member = String::from_utf8_lossy(&args[2]).into_owned();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&src);
    db.check_ttl(&dst);

    // Type checks
    if let Some(t) = db.key_type(&src)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }
    if let Some(t) = db.key_type(&dst)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if !db.keys.contains_key(&src) {
        return Frame::Integer(0);
    }

    if !db.set_is_member(&src, &member) {
        return Frame::Integer(0);
    }

    db.set_rem(&src, std::slice::from_ref(&member), now);
    db.set_add(&dst, std::slice::from_ref(&member), now);
    Frame::Integer(1)
}

/// SPOP key [count]
fn cmd_spop(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("spop"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut with_count = false;
    let mut count: usize = 1;

    if args.len() > 1 {
        match parse_int(&args[1]) {
            Some(n) if n < 0 => return Frame::error(MSG_OUT_OF_RANGE),
            Some(n) => {
                count = n as usize;
                with_count = true;
            }
            None => return Frame::error(MSG_INVALID_INT),
        }
    }
    if args.len() > 2 {
        return Frame::error(MSG_INVALID_INT);
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return if with_count {
            Frame::Array(vec![])
        } else {
            Frame::Null
        };
    }

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut members = db.set_members(&key);
    let mut deleted = Vec::new();
    for _ in 0..count {
        if members.is_empty() {
            break;
        }
        let idx = inner.rng.random_range(0..members.len());
        let member = members.remove(idx);
        let db = inner.db_mut(ctx.selected_db);
        db.set_rem(&key, std::slice::from_ref(&member), now);
        deleted.push(member);
    }

    if !with_count {
        if deleted.is_empty() {
            Frame::Null
        } else {
            Frame::Bulk(deleted[0].clone().into())
        }
    } else {
        Frame::Array(deleted.into_iter().map(|m| Frame::Bulk(m.into())).collect())
    }
}

/// SRANDMEMBER key [count]
fn cmd_srandmember(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() || args.len() > 2 {
        return Frame::error(err_wrong_number("srandmember"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut count: i64 = 0;
    let mut with_count = false;

    if args.len() == 2 {
        match parse_int(&args[1]) {
            Some(n) => {
                count = n;
                with_count = true;
            }
            None => return Frame::error(MSG_INVALID_INT),
        }
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return if with_count {
            Frame::Array(vec![])
        } else {
            Frame::Null
        };
    }

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut members = db.set_members(&key);

    if count < 0 {
        // Negative count: allow duplicates
        let abs_count = (-count) as usize;
        let mut result = Vec::with_capacity(abs_count);
        for _ in 0..abs_count {
            let idx = inner.rng.random_range(0..members.len());
            result.push(Frame::Bulk(members[idx].clone().into()));
        }
        return Frame::Array(result);
    }

    // Positive count: unique members, shuffle
    members.shuffle(&mut inner.rng);
    let take = (count as usize).min(members.len());

    if !with_count {
        return Frame::Bulk(members[0].clone().into());
    }

    Frame::Array(
        members[..take]
            .iter()
            .map(|m| Frame::Bulk(m.clone().into()))
            .collect(),
    )
}

/// SSCAN key cursor [MATCH pattern] [COUNT count]
fn cmd_sscan(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("sscan"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let cursor: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_CURSOR),
    };

    let opts = match super::parse_scan_opts(&args[2..], false) {
        Ok(o) => o,
        Err(e) => return e,
    };

    // SSCAN validates COUNT more strictly
    let scan_count: usize = match opts.count {
        Some(n) if n < 0 => return Frame::error(MSG_INVALID_INT),
        Some(0) => return Frame::error(MSG_SYNTAX_ERROR),
        Some(n) => n as usize,
        None => 0,
    };

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if db.keys.contains_key(key.as_ref())
        && let Some(t) = db.key_type(&key)
        && t != KeyType::Set
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut members = db.set_members(&key);
    members.sort();

    // Apply MATCH filter
    if let Some(ref pat) = opts.pattern {
        members = crate::keys::match_keys_vec(&members, pat);
    }

    let low = cursor as usize;
    let high = if scan_count > 0 {
        (low + scan_count).min(members.len())
    } else {
        members.len()
    };

    if low >= members.len() {
        return Frame::Array(vec![Frame::Bulk("0".into()), Frame::Array(vec![])]);
    }

    let cursor_value = if high >= members.len() { 0 } else { high };

    let selected = &members[low..high];
    Frame::Array(vec![
        Frame::Bulk(cursor_value.to_string().into()),
        Frame::Array(
            selected
                .iter()
                .map(|m| Frame::Bulk(m.clone().into()))
                .collect(),
        ),
    ])
}

/// SINTERCARD numkeys key [key ...] [LIMIT limit]
fn cmd_sintercard(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("sintercard"));
    }

    let num_keys = match parse_int(&args[0]) {
        Some(n) if n < 1 => {
            return Frame::error("ERR numkeys should be greater than 0");
        }
        Some(n) => n as usize,
        None => {
            return Frame::error("ERR numkeys should be greater than 0");
        }
    };

    if args.len() < 1 + num_keys {
        return Frame::error(MSG_INVALID_KEYS_NUMBER);
    }

    let keys: Vec<String> = args[1..1 + num_keys]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();

    let mut limit: usize = 0;
    let rest = &args[1 + num_keys..];

    if rest.len() == 2 {
        let opt = String::from_utf8_lossy(&rest[0]).to_uppercase();
        if opt == "LIMIT" {
            match parse_int(&rest[1]) {
                Some(n) if n < 0 => {
                    return Frame::error("ERR LIMIT can't be negative");
                }
                Some(n) => limit = n as usize,
                None => return Frame::error(MSG_INVALID_INT),
            }
        } else {
            return Frame::error(MSG_SYNTAX_ERROR);
        }
    } else if !rest.is_empty() {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let result = match set_op(state, ctx, &keys, SetOp::Inter) {
        Ok(set) => set,
        Err(e) => return e,
    };

    let count = result.len();
    if limit > 0 && count > limit {
        Frame::Integer(limit as i64)
    } else {
        Frame::Integer(count as i64)
    }
}
