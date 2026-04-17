use std::sync::Arc;

use rand::Rng;
use rand::seq::SliceRandom;

use super::{parse_float, parse_int};
use crate::cmd::string::format_float;
use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INVALID_CURSOR, MSG_INVALID_FLOAT, MSG_INVALID_INT, MSG_INVALID_MIN_MAX,
    MSG_INVALID_RANGE_ITEM, MSG_SINGLE_ELEMENT_PAIR, MSG_SYNTAX_ERROR, MSG_WRONG_TYPE,
    MSG_XX_AND_NX, err_wrong_number,
};
use crate::frame::Frame;
use crate::types::{Direction, SSElem, SortedSet};

pub fn register(table: &mut CommandTable) {
    table.add("ZADD", cmd_zadd, false, -4);
    table.add("ZCARD", cmd_zcard, true, 2);
    table.add("ZCOUNT", cmd_zcount, true, 4);
    table.add("ZINCRBY", cmd_zincrby, false, 4);
    table.add("ZSCORE", cmd_zscore, true, 3);
    table.add("ZMSCORE", cmd_zmscore, true, -3);
    table.add("ZRANK", cmd_zrank, true, -3);
    table.add("ZREVRANK", cmd_zrevrank, true, -3);
    table.add("ZREM", cmd_zrem, false, -3);
    table.add("ZRANGE", cmd_zrange, true, -4);
    table.add("ZREVRANGE", cmd_zrevrange, true, -4);
    table.add("ZRANGEBYSCORE", cmd_zrangebyscore, true, -4);
    table.add("ZREVRANGEBYSCORE", cmd_zrevrangebyscore, true, -4);
    table.add("ZRANGEBYLEX", cmd_zrangebylex, true, -4);
    table.add("ZREVRANGEBYLEX", cmd_zrevrangebylex, true, -4);
    table.add("ZLEXCOUNT", cmd_zlexcount, true, 4);
    table.add("ZREMRANGEBYRANK", cmd_zremrangebyrank, false, 4);
    table.add("ZREMRANGEBYSCORE", cmd_zremrangebyscore, false, 4);
    table.add("ZREMRANGEBYLEX", cmd_zremrangebylex, false, 4);
    table.add("ZUNIONSTORE", cmd_zunionstore, false, -4);
    table.add("ZINTERSTORE", cmd_zinterstore, false, -4);
    table.add("ZPOPMIN", cmd_zpopmin, false, -2);
    table.add("ZPOPMAX", cmd_zpopmax, false, -2);
    table.add("ZSCAN", cmd_zscan, true, -3);
    table.add("ZINTER", cmd_zinter, true, -3);
    table.add("ZUNION", cmd_zunion, true, -3);
    table.add("ZRANDMEMBER", cmd_zrandmember, true, -2);
}

/// Format a float for Redis output (scores).
fn write_float(f: f64) -> String {
    if f == f64::INFINITY {
        "inf".to_string()
    } else if f == f64::NEG_INFINITY {
        "-inf".to_string()
    } else {
        format_float(f)
    }
}

/// Parse a score range like "1.5", "(1.5", "+inf", "-inf".
fn parse_float_range(s: &str) -> Result<(f64, bool), ()> {
    if s.is_empty() {
        return Err(());
    }
    let (s, inclusive) = if let Some(rest) = s.strip_prefix('(') {
        (rest, false)
    } else {
        (s, true)
    };
    match s.to_lowercase().as_str() {
        "+inf" | "inf" => Ok((f64::INFINITY, true)),
        "-inf" => Ok((f64::NEG_INFINITY, true)),
        _ => s.parse::<f64>().map(|f| (f, inclusive)).map_err(|_| ()),
    }
}

/// Parse a lex range like "[a", "(a", "+", "-".
fn parse_lex_range(s: &str) -> Result<(String, bool), ()> {
    if s.is_empty() {
        return Err(());
    }
    if s == "+" || s == "-" {
        return Ok((s.to_string(), false));
    }
    match s.as_bytes()[0] {
        b'(' => Ok((s[1..].to_string(), false)),
        b'[' => Ok((s[1..].to_string(), true)),
        _ => Err(()),
    }
}

/// Filter elements by score range.
fn with_ss_range(
    members: Vec<SSElem>,
    min: f64,
    min_incl: bool,
    max: f64,
    max_incl: bool,
) -> Vec<SSElem> {
    members
        .into_iter()
        .filter(|e| {
            let above_min = if min_incl {
                e.score >= min
            } else {
                e.score > min
            };
            let below_max = if max_incl {
                e.score <= max
            } else {
                e.score < max
            };
            above_min && below_max
        })
        .collect()
}

/// Filter member names by lex range.
fn with_lex_range(
    members: Vec<String>,
    min: &str,
    min_incl: bool,
    max: &str,
    max_incl: bool,
) -> Vec<String> {
    if max == "-" || min == "+" {
        return Vec::new();
    }
    members
        .into_iter()
        .filter(|m| {
            let above_min = if min == "-" {
                true
            } else if min_incl {
                m.as_str() >= min
            } else {
                m.as_str() > min
            };
            let below_max = if max == "+" {
                true
            } else if max_incl {
                m.as_str() <= max
            } else {
                m.as_str() < max
            };
            above_min && below_max
        })
        .collect()
}

/// Normalize Redis-style range indices for sorted sets.
fn redis_range(len: usize, start: i64, stop: i64) -> (usize, usize) {
    let len = len as i64;
    let mut s = if start < 0 { len + start } else { start };
    let mut e = if stop < 0 { len + stop } else { stop };
    if s < 0 {
        s = 0;
    }
    if e >= len {
        e = len - 1;
    }
    if s > e || s >= len {
        return (0, 0);
    }
    (s as usize, (e + 1) as usize)
}

// ── Commands ─────────────────────────────────────────────────────────

/// ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
fn cmd_zadd(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut i = 1;
    let mut nx = false;
    let mut xx = false;
    let mut gt = false;
    let mut lt = false;
    let mut ch = false;
    let mut incr = false;

    // Parse flags
    while i < args.len() {
        let flag = String::from_utf8_lossy(&args[i]).to_uppercase();
        match flag.as_str() {
            "NX" => {
                nx = true;
                i += 1;
            }
            "XX" => {
                xx = true;
                i += 1;
            }
            "GT" => {
                gt = true;
                i += 1;
            }
            "LT" => {
                lt = true;
                i += 1;
            }
            "CH" => {
                ch = true;
                i += 1;
            }
            "INCR" => {
                incr = true;
                i += 1;
            }
            _ => break,
        }
    }

    // Remaining args should be score-member pairs
    let remaining = &args[i..];
    if remaining.is_empty() || !remaining.len().is_multiple_of(2) {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    // Parse score-member pairs
    let mut elems: Vec<(String, f64)> = Vec::new();
    let mut j = 0;
    while j < remaining.len() {
        let score = match parse_float(&remaining[j]) {
            Some(f) => f,
            None => return Frame::error(MSG_INVALID_FLOAT),
        };
        let member = String::from_utf8_lossy(&remaining[j + 1]).into_owned();
        elems.push((member, score));
        j += 2;
    }

    // Validation
    if xx && nx {
        return Frame::error(MSG_XX_AND_NX);
    }
    if (gt || lt) && (gt && lt || nx) {
        return Frame::error("ERR GT, LT, and/or NX options at the same time are not compatible");
    }
    if incr && elems.len() > 1 {
        return Frame::error(MSG_SINGLE_ELEMENT_PAIR);
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // INCR mode
    if incr {
        let (member, delta) = &elems[0];
        if nx && db.sset_exists(&key, member) {
            return Frame::Null;
        }
        if xx && !db.sset_exists(&key, member) {
            return Frame::Null;
        }
        let new_score = db.sset_incrby(&key, member, *delta, now);
        return if ctx.resp3 {
            Frame::Double(new_score)
        } else {
            Frame::Bulk(write_float(new_score).into())
        };
    }

    let mut count = 0i64;
    for (member, score) in &elems {
        let exists = db.sset_exists(&key, member);
        if nx && exists {
            continue;
        }
        if xx && !exists {
            continue;
        }
        if gt
            && exists
            && let Some(old) = db.sset_score(&key, member)
            && *score <= old
        {
            continue;
        }
        if lt
            && exists
            && let Some(old) = db.sset_score(&key, member)
            && *score >= old
        {
            continue;
        }
        let old_score = db.sset_score(&key, member);
        let is_new = db.sset_add(&key, *score, member, now);
        if is_new || (ch && old_score != Some(*score)) {
            count += 1;
        }
    }

    Frame::Integer(count)
}

/// ZCARD key
fn cmd_zcard(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    Frame::Integer(db.sset_card(&key) as i64)
}

/// ZCOUNT key min max
fn cmd_zcount(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let min_s = String::from_utf8_lossy(&args[1]);
    let max_s = String::from_utf8_lossy(&args[2]);

    let (min, min_incl) = match parse_float_range(&min_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_MIN_MAX),
    };
    let (max, max_incl) = match parse_float_range(&max_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_MIN_MAX),
    };

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key.as_ref()) {
        Some(ss) => ss,
        None => return Frame::Integer(0),
    };
    let elems = ss.by_score(Direction::Asc);
    let filtered = with_ss_range(elems, min, min_incl, max, max_incl);
    Frame::Integer(filtered.len() as i64)
}

/// ZINCRBY key increment member
fn cmd_zincrby(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let delta = match parse_float(&args[1]) {
        Some(f) => f,
        None => return Frame::error(MSG_INVALID_FLOAT),
    };
    let member = String::from_utf8_lossy(&args[2]).into_owned();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let new_score = db.sset_incrby(&key, &member, delta, now);
    Frame::Bulk(write_float(new_score).into())
}

/// ZSCORE key member
fn cmd_zscore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let member = String::from_utf8_lossy(&args[1]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Null;
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.sset_score(&key, &member) {
        Some(score) => {
            if ctx.resp3 {
                Frame::Double(score)
            } else {
                Frame::Bulk(write_float(score).into())
            }
        }
        None => Frame::Null,
    }
}

/// ZMSCORE key member [member ...]
fn cmd_zmscore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let results: Vec<Frame> = args[1..]
        .iter()
        .map(|a| {
            let member = String::from_utf8_lossy(a);
            match db.sset_score(&key, &member) {
                Some(score) => Frame::Bulk(write_float(score).into()),
                None => Frame::Null,
            }
        })
        .collect();

    Frame::Array(results)
}

/// ZRANK key member
fn cmd_zrank(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zrank_impl(state, ctx, args, Direction::Asc, "zrank")
}

/// ZREVRANK key member
fn cmd_zrevrank(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zrank_impl(state, ctx, args, Direction::Desc, "zrevrank")
}

fn zrank_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    dir: Direction,
    cmd: &str,
) -> Frame {
    if args.len() > 3 {
        return Frame::error(err_wrong_number(cmd));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let member = String::from_utf8_lossy(&args[1]);

    let with_score =
        args.len() == 3 && String::from_utf8_lossy(&args[2]).to_uppercase() == "WITHSCORE";

    if args.len() == 3 && !with_score {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return if with_score {
            Frame::NullArray
        } else {
            Frame::Null
        };
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key.as_ref()) {
        Some(ss) => ss,
        None => {
            return if with_score {
                Frame::NullArray
            } else {
                Frame::Null
            };
        }
    };

    match ss.rank(&member, dir) {
        Some(rank) => {
            if with_score {
                let score = ss.get(&member).unwrap_or(0.0);
                Frame::Array(vec![
                    Frame::Integer(rank as i64),
                    Frame::Bulk(write_float(score).into()),
                ])
            } else {
                Frame::Integer(rank as i64)
            }
        }
        None => {
            if with_score {
                Frame::NullArray
            } else {
                Frame::Null
            }
        }
    }
}

/// ZREM key member [member ...]
fn cmd_zrem(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut deleted = 0i64;
    for a in &args[1..] {
        let member = String::from_utf8_lossy(a);
        if db.sset_rem(&key, &member, now) {
            deleted += 1;
        }
    }
    Frame::Integer(deleted)
}

// ── Range commands ───────────────────────────────────────────────────

/// ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
fn cmd_zrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let min_s = String::from_utf8_lossy(&args[1]).into_owned();
    let max_s = String::from_utf8_lossy(&args[2]).into_owned();

    let mut with_scores = false;
    let mut by_score = false;
    let mut by_lex = false;
    let mut reverse = false;
    let mut with_limit = false;
    let mut offset_s = String::new();
    let mut count_s = String::new();

    let mut i = 3;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "BYSCORE" => {
                by_score = true;
                i += 1;
            }
            "BYLEX" => {
                by_lex = true;
                i += 1;
            }
            "REV" => {
                reverse = true;
                i += 1;
            }
            "LIMIT" => {
                with_limit = true;
                i += 1;
                if i + 1 >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                offset_s = String::from_utf8_lossy(&args[i]).into_owned();
                count_s = String::from_utf8_lossy(&args[i + 1]).into_owned();
                i += 2;
            }
            "WITHSCORES" => {
                with_scores = true;
                i += 1;
            }
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    }

    if by_score && by_lex {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if by_score {
        run_range_by_score(
            db,
            &key,
            &min_s,
            &max_s,
            reverse,
            with_limit,
            &offset_s,
            &count_s,
            with_scores,
        )
    } else if by_lex {
        run_range_by_lex(
            db, &key, &min_s, &max_s, reverse, with_limit, &offset_s, &count_s,
        )
    } else {
        if with_limit {
            return Frame::error(
                "ERR syntax error, LIMIT is only supported in combination with either BYSCORE or BYLEX",
            );
        }
        run_range_by_rank(db, &key, &min_s, &max_s, reverse, with_scores)
    }
}

/// ZREVRANGE key start stop [WITHSCORES]
fn cmd_zrevrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let min_s = String::from_utf8_lossy(&args[1]).into_owned();
    let max_s = String::from_utf8_lossy(&args[2]).into_owned();

    let mut with_scores = false;
    if args.len() > 3 {
        let opt = String::from_utf8_lossy(&args[3]).to_uppercase();
        if opt == "WITHSCORES" {
            with_scores = true;
        } else {
            return Frame::error(MSG_SYNTAX_ERROR);
        }
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    run_range_by_rank(db, &key, &min_s, &max_s, true, with_scores)
}

/// ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
fn cmd_zrangebyscore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zrangebyscore_impl(state, ctx, args, false)
}

/// ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
fn cmd_zrevrangebyscore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zrangebyscore_impl(state, ctx, args, true)
}

fn zrangebyscore_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    reverse: bool,
) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let min_s = String::from_utf8_lossy(&args[1]).into_owned();
    let max_s = String::from_utf8_lossy(&args[2]).into_owned();

    let mut with_scores = false;
    let mut with_limit = false;
    let mut offset_s = String::new();
    let mut count_s = String::new();

    let mut i = 3;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "WITHSCORES" => {
                with_scores = true;
                i += 1;
            }
            "LIMIT" => {
                with_limit = true;
                i += 1;
                if i + 1 >= args.len() {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                offset_s = String::from_utf8_lossy(&args[i]).into_owned();
                count_s = String::from_utf8_lossy(&args[i + 1]).into_owned();
                i += 2;
            }
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    run_range_by_score(
        db,
        &key,
        &min_s,
        &max_s,
        reverse,
        with_limit,
        &offset_s,
        &count_s,
        with_scores,
    )
}

/// ZRANGEBYLEX key min max [LIMIT offset count]
fn cmd_zrangebylex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zrangebylex_impl(state, ctx, args, false)
}

/// ZREVRANGEBYLEX key max min [LIMIT offset count]
fn cmd_zrevrangebylex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zrangebylex_impl(state, ctx, args, true)
}

fn zrangebylex_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    reverse: bool,
) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let min_s = String::from_utf8_lossy(&args[1]).into_owned();
    let max_s = String::from_utf8_lossy(&args[2]).into_owned();

    let mut with_limit = false;
    let mut offset_s = String::new();
    let mut count_s = String::new();

    let mut i = 3;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        if opt == "LIMIT" {
            with_limit = true;
            i += 1;
            if i + 1 >= args.len() {
                return Frame::error(MSG_SYNTAX_ERROR);
            }
            offset_s = String::from_utf8_lossy(&args[i]).into_owned();
            count_s = String::from_utf8_lossy(&args[i + 1]).into_owned();
            i += 2;
        } else {
            return Frame::error(MSG_SYNTAX_ERROR);
        }
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    run_range_by_lex(
        db, &key, &min_s, &max_s, reverse, with_limit, &offset_s, &count_s,
    )
}

/// ZLEXCOUNT key min max
fn cmd_zlexcount(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let min_s = String::from_utf8_lossy(&args[1]);
    let max_s = String::from_utf8_lossy(&args[2]);

    let (min, min_incl) = match parse_lex_range(&min_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_RANGE_ITEM),
    };
    let (max, max_incl) = match parse_lex_range(&max_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_RANGE_ITEM),
    };

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key.as_ref()) {
        Some(ss) => ss,
        None => return Frame::Integer(0),
    };
    let mut members = ss.members_sorted();
    members.sort();
    let filtered = with_lex_range(members, &min, min_incl, &max, max_incl);
    Frame::Integer(filtered.len() as i64)
}

// ── Remove range commands ────────────────────────────────────────────

/// ZREMRANGEBYRANK key start stop
fn cmd_zremrangebyrank(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let start = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };
    let stop = match parse_int(&args[2]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(&key) {
        Some(ss) => ss,
        None => return Frame::Integer(0),
    };
    let members = ss.members_sorted();
    let (rs, re) = redis_range(members.len(), start, stop);
    let to_remove: Vec<String> = members[rs..re].to_vec();

    for m in &to_remove {
        db.sset_rem(&key, m, now);
    }
    Frame::Integer(to_remove.len() as i64)
}

/// ZREMRANGEBYSCORE key min max
fn cmd_zremrangebyscore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let min_s = String::from_utf8_lossy(&args[1]);
    let max_s = String::from_utf8_lossy(&args[2]);

    let (min, min_incl) = match parse_float_range(&min_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_MIN_MAX),
    };
    let (max, max_incl) = match parse_float_range(&max_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_MIN_MAX),
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(&key) {
        Some(ss) => ss,
        None => return Frame::Integer(0),
    };
    let elems = ss.by_score(Direction::Asc);
    let filtered = with_ss_range(elems, min, min_incl, max, max_incl);
    let to_remove: Vec<String> = filtered.into_iter().map(|e| e.member).collect();

    for m in &to_remove {
        db.sset_rem(&key, m, now);
    }
    Frame::Integer(to_remove.len() as i64)
}

/// ZREMRANGEBYLEX key min max
fn cmd_zremrangebylex(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let min_s = String::from_utf8_lossy(&args[1]);
    let max_s = String::from_utf8_lossy(&args[2]);

    let (min, min_incl) = match parse_lex_range(&min_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_RANGE_ITEM),
    };
    let (max, max_incl) = match parse_lex_range(&max_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_RANGE_ITEM),
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(&key) {
        Some(ss) => ss,
        None => return Frame::Integer(0),
    };
    let mut members = ss.members_sorted();
    members.sort();
    let filtered = with_lex_range(members, &min, min_incl, &max, max_incl);

    for m in &filtered {
        db.sset_rem(&key, m, now);
    }
    Frame::Integer(filtered.len() as i64)
}

// ── Set operations (ZUNIONSTORE, ZINTERSTORE) ────────────────────────

/// ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS w...] [AGGREGATE SUM|MIN|MAX]
fn cmd_zunionstore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zstore_impl(state, ctx, args, false)
}

/// ZINTERSTORE destination numkeys key [key ...] [WEIGHTS w...] [AGGREGATE SUM|MIN|MAX]
fn cmd_zinterstore(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zstore_impl(state, ctx, args, true)
}

fn zstore_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    intersect: bool,
) -> Frame {
    let dest = String::from_utf8_lossy(&args[0]).into_owned();
    let num_keys = match parse_int(&args[1]) {
        Some(n) if n > 0 => n as usize,
        Some(_) => {
            return Frame::error("ERR at least 1 input key is needed for ZUNIONSTORE/ZINTERSTORE");
        }
        _ => return Frame::error(MSG_INVALID_INT),
    };

    if args.len() < 2 + num_keys {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let keys: Vec<String> = args[2..2 + num_keys]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();
    let mut rest = &args[2 + num_keys..];

    let mut weights: Vec<f64> = Vec::new();
    let mut with_weights = false;
    let mut aggregate = "sum".to_string();

    while !rest.is_empty() {
        let opt = String::from_utf8_lossy(&rest[0]).to_lowercase();
        match opt.as_str() {
            "weights" => {
                if rest.len() < num_keys + 1 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                for i in 0..num_keys {
                    match parse_float(&rest[i + 1]) {
                        Some(f) => weights.push(f),
                        None => return Frame::error("ERR weight value is not a float"),
                    }
                }
                with_weights = true;
                rest = &rest[num_keys + 1..];
            }
            "aggregate" => {
                if rest.len() < 2 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                aggregate = String::from_utf8_lossy(&rest[1]).to_lowercase();
                match aggregate.as_str() {
                    "sum" | "min" | "max" => {}
                    _ => return Frame::error(MSG_SYNTAX_ERROR),
                }
                rest = &rest[2..];
            }
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    // Collect all scores
    let mut sset: std::collections::HashMap<String, f64> = std::collections::HashMap::new();
    let mut counts: std::collections::HashMap<String, usize> = std::collections::HashMap::new();

    for (i, key) in keys.iter().enumerate() {
        if !db.keys.contains_key(key) {
            continue;
        }
        let key_type = db.key_type(key);

        let set: std::collections::HashMap<String, f64> = match key_type {
            Some(crate::types::KeyType::Set) => db
                .set_keys
                .get(key)
                .map(|s| s.iter().map(|m| (m.clone(), 1.0)).collect())
                .unwrap_or_default(),
            Some(crate::types::KeyType::SortedSet) => db
                .sorted_set_keys
                .get(key)
                .map(|ss| ss.scores.clone())
                .unwrap_or_default(),
            _ => return Frame::error(MSG_WRONG_TYPE),
        };

        for (member, mut score) in set {
            if with_weights {
                score *= weights[i];
            }
            *counts.entry(member.clone()).or_insert(0) += 1;
            let entry = sset.entry(member);
            match entry {
                std::collections::hash_map::Entry::Vacant(e) => {
                    e.insert(score);
                }
                std::collections::hash_map::Entry::Occupied(mut e) => {
                    let old = *e.get();
                    match aggregate.as_str() {
                        "sum" => *e.get_mut() += score,
                        "min" if score < old => {
                            *e.get_mut() = score;
                        }
                        "max" if score > old => {
                            *e.get_mut() = score;
                        }
                        _ => {}
                    }
                }
            }
        }
    }

    // For ZINTERSTORE: only keep members present in ALL keys
    if intersect {
        sset.retain(|member, _| counts.get(member).copied().unwrap_or(0) == keys.len());
    }

    // Store result
    db.del(&dest);
    if !sset.is_empty() {
        let mut new_ss = SortedSet::new();
        for (member, score) in &sset {
            new_ss.set(*score, member);
        }
        db.sset_set(&dest, new_ss, now);
    }

    Frame::Integer(sset.len() as i64)
}

// ── ZPOPMIN/ZPOPMAX ─────────────────────────────────────────────────

/// ZPOPMIN key [count]
fn cmd_zpopmin(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zpop_impl(state, ctx, args, false)
}

/// ZPOPMAX key [count]
fn cmd_zpopmax(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zpop_impl(state, ctx, args, true)
}

fn zpop_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    reverse: bool,
) -> Frame {
    if args.len() > 2 {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let count = if args.len() > 1 {
        match parse_int(&args[1]) {
            Some(n) if n >= 0 => n as usize,
            _ => return Frame::error(MSG_INVALID_INT),
        }
    } else {
        1
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(&key) {
        return Frame::Array(vec![]);
    }
    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(&key) {
        Some(ss) => ss,
        None => return Frame::Array(vec![]),
    };

    let dir = if reverse {
        Direction::Desc
    } else {
        Direction::Asc
    };
    let elems = ss.by_score(dir);
    let take = count.min(elems.len());
    let to_pop: Vec<SSElem> = elems[..take].to_vec();

    let mut result = Vec::new();
    for e in &to_pop {
        result.push(Frame::Bulk(e.member.clone().into()));
        result.push(Frame::Bulk(write_float(e.score).into()));
        db.sset_rem(&key, &e.member, now);
    }

    Frame::Array(result)
}

// ── ZSCAN ────────────────────────────────────────────────────────────

/// ZSCAN key cursor [MATCH pattern] [COUNT count]
fn cmd_zscan(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let _cursor = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_CURSOR),
    };

    let opts = match super::parse_scan_opts(&args[2..], false) {
        Ok(o) => o,
        Err(e) => return e,
    };

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key.as_ref()) {
        Some(ss) => ss,
        None => {
            return Frame::Array(vec![Frame::Bulk("0".into()), Frame::Array(vec![])]);
        }
    };

    let mut members = ss.members_sorted();
    members.sort();

    // Apply MATCH filter
    if let Some(ref pat) = opts.pattern {
        members = crate::keys::match_keys_vec(&members, pat);
    }

    // Return all members with cursor=0 (no real cursor pagination)
    let mut result = Vec::new();
    for m in &members {
        result.push(Frame::Bulk(m.clone().into()));
        let score = ss.get(m).unwrap_or(0.0);
        result.push(Frame::Bulk(write_float(score).into()));
    }

    Frame::Array(vec![Frame::Bulk("0".into()), Frame::Array(result)])
}

// ── Range helper functions ───────────────────────────────────────────

fn run_range_by_rank(
    db: &crate::db::RedisDB,
    key: &str,
    min_s: &str,
    max_s: &str,
    reverse: bool,
    with_scores: bool,
) -> Frame {
    let min: i64 = match min_s.parse() {
        Ok(n) => n,
        Err(_) => return Frame::error(MSG_INVALID_INT),
    };
    let max: i64 = match max_s.parse() {
        Ok(n) => n,
        Err(_) => return Frame::error(MSG_INVALID_INT),
    };

    if !db.keys.contains_key(key) {
        return Frame::Array(vec![]);
    }
    if let Some(t) = db.key_type(key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key) {
        Some(ss) => ss,
        None => return Frame::Array(vec![]),
    };

    let dir = if reverse {
        Direction::Desc
    } else {
        Direction::Asc
    };
    let elems = ss.by_score(dir);
    let (rs, re) = redis_range(elems.len(), min, max);

    let mut result = Vec::new();
    for e in &elems[rs..re] {
        result.push(Frame::Bulk(e.member.clone().into()));
        if with_scores {
            result.push(Frame::Bulk(write_float(e.score).into()));
        }
    }
    Frame::Array(result)
}

#[allow(clippy::too_many_arguments)]
fn run_range_by_score(
    db: &crate::db::RedisDB,
    key: &str,
    min_s: &str,
    max_s: &str,
    reverse: bool,
    with_limit: bool,
    offset_s: &str,
    count_s: &str,
    with_scores: bool,
) -> Frame {
    let mut limit_offset = 0i64;
    let mut limit_count = -1i64;

    if with_limit {
        limit_offset = match offset_s.parse() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_INT),
        };
        limit_count = match count_s.parse() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_INT),
        };
    }

    let (min, min_incl) = match parse_float_range(min_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_MIN_MAX),
    };
    let (max, max_incl) = match parse_float_range(max_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_MIN_MAX),
    };

    if !db.keys.contains_key(key) {
        return Frame::Array(vec![]);
    }
    if let Some(t) = db.key_type(key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key) {
        Some(ss) => ss,
        None => return Frame::Array(vec![]),
    };

    let elems = ss.by_score(Direction::Asc);

    // For reverse, swap min/max and their inclusiveness
    let (fmin, fmin_incl, fmax, fmax_incl) = if reverse {
        (max, max_incl, min, min_incl)
    } else {
        (min, min_incl, max, max_incl)
    };

    let mut filtered = with_ss_range(elems, fmin, fmin_incl, fmax, fmax_incl);
    if reverse {
        filtered.reverse();
    }

    // Apply LIMIT
    if with_limit {
        if limit_offset < 0 {
            filtered = Vec::new();
        } else {
            let offset = limit_offset as usize;
            if offset < filtered.len() {
                filtered = filtered[offset..].to_vec();
            } else {
                filtered = Vec::new();
            }
            if limit_count >= 0 {
                let count = limit_count as usize;
                if filtered.len() > count {
                    filtered.truncate(count);
                }
            }
        }
    }

    let mut result = Vec::new();
    for e in &filtered {
        result.push(Frame::Bulk(e.member.clone().into()));
        if with_scores {
            result.push(Frame::Bulk(write_float(e.score).into()));
        }
    }
    Frame::Array(result)
}

#[allow(clippy::too_many_arguments)]
fn run_range_by_lex(
    db: &crate::db::RedisDB,
    key: &str,
    min_s: &str,
    max_s: &str,
    reverse: bool,
    with_limit: bool,
    offset_s: &str,
    count_s: &str,
) -> Frame {
    let mut limit_offset = 0i64;
    let mut limit_count = -1i64;

    if with_limit {
        limit_offset = match offset_s.parse() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_INT),
        };
        limit_count = match count_s.parse() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_INT),
        };
    }

    let (min, min_incl) = match parse_lex_range(min_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_RANGE_ITEM),
    };
    let (max, max_incl) = match parse_lex_range(max_s) {
        Ok(v) => v,
        Err(_) => return Frame::error(MSG_INVALID_RANGE_ITEM),
    };

    if !db.keys.contains_key(key) {
        return Frame::Array(vec![]);
    }
    if let Some(t) = db.key_type(key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key) {
        Some(ss) => ss,
        None => return Frame::Array(vec![]),
    };

    let mut members = ss.members_sorted();
    members.sort();

    // For reverse, swap min/max
    let (fmin, fmin_incl, fmax, fmax_incl) = if reverse {
        (max.clone(), max_incl, min.clone(), min_incl)
    } else {
        (min, min_incl, max, max_incl)
    };

    let mut filtered = with_lex_range(members, &fmin, fmin_incl, &fmax, fmax_incl);
    if reverse {
        filtered.reverse();
    }

    // Apply LIMIT
    if with_limit {
        if limit_offset < 0 {
            filtered = Vec::new();
        } else {
            let offset = limit_offset as usize;
            if offset < filtered.len() {
                filtered = filtered[offset..].to_vec();
            } else {
                filtered = Vec::new();
            }
            if limit_count >= 0 {
                let count = limit_count as usize;
                if filtered.len() > count {
                    filtered.truncate(count);
                }
            }
        }
    }

    let result: Vec<Frame> = filtered
        .into_iter()
        .map(|m| Frame::Bulk(m.into()))
        .collect();
    Frame::Array(result)
}

// ── ZINTER / ZUNION (without STORE) ──────────────────────────────────

/// Parse args for ZINTER/ZUNION: numkeys key [...] [WEIGHTS ...] [AGGREGATE ...] [WITHSCORES]
fn zop_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    intersect: bool,
) -> Frame {
    let num_keys = match parse_int(&args[0]) {
        Some(n) if n > 0 => n as usize,
        Some(_) => {
            return Frame::error(
                "ERR at least 1 input key is needed for 'zinter'/'zunion' command",
            );
        }
        _ => return Frame::error(MSG_INVALID_INT),
    };

    if args.len() < 1 + num_keys {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let keys: Vec<String> = args[1..1 + num_keys]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();
    let mut rest = &args[1 + num_keys..];

    let mut weights: Vec<f64> = Vec::new();
    let mut with_weights = false;
    let mut aggregate = "sum".to_string();
    let mut with_scores = false;

    while !rest.is_empty() {
        let opt = String::from_utf8_lossy(&rest[0]).to_lowercase();
        match opt.as_str() {
            "weights" => {
                if rest.len() < num_keys + 1 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                for i in 0..num_keys {
                    match parse_float(&rest[i + 1]) {
                        Some(f) => weights.push(f),
                        None => return Frame::error("ERR weight value is not a float"),
                    }
                }
                with_weights = true;
                rest = &rest[num_keys + 1..];
            }
            "aggregate" => {
                if rest.len() < 2 {
                    return Frame::error(MSG_SYNTAX_ERROR);
                }
                aggregate = String::from_utf8_lossy(&rest[1]).to_lowercase();
                match aggregate.as_str() {
                    "sum" | "min" | "max" => {}
                    _ => return Frame::error(MSG_SYNTAX_ERROR),
                }
                rest = &rest[2..];
            }
            "withscores" => {
                with_scores = true;
                rest = &rest[1..];
            }
            _ => return Frame::error(MSG_SYNTAX_ERROR),
        }
    }

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    let mut sset: std::collections::HashMap<String, f64> = std::collections::HashMap::new();
    let mut counts: std::collections::HashMap<String, usize> = std::collections::HashMap::new();

    for (i, key) in keys.iter().enumerate() {
        if !db.keys.contains_key(key) {
            continue;
        }
        let key_type = db.key_type(key);
        let set: std::collections::HashMap<String, f64> = match key_type {
            Some(crate::types::KeyType::Set) => db
                .set_keys
                .get(key)
                .map(|s| s.iter().map(|m| (m.clone(), 1.0)).collect())
                .unwrap_or_default(),
            Some(crate::types::KeyType::SortedSet) => db
                .sorted_set_keys
                .get(key)
                .map(|ss| ss.scores.clone())
                .unwrap_or_default(),
            _ => return Frame::error(MSG_WRONG_TYPE),
        };

        for (member, mut score) in set {
            if with_weights {
                score *= weights[i];
            }
            *counts.entry(member.clone()).or_insert(0) += 1;
            let entry = sset.entry(member);
            match entry {
                std::collections::hash_map::Entry::Vacant(e) => {
                    e.insert(score);
                }
                std::collections::hash_map::Entry::Occupied(mut e) => {
                    let old = *e.get();
                    match aggregate.as_str() {
                        "sum" => *e.get_mut() += score,
                        "min" if score < old => {
                            *e.get_mut() = score;
                        }
                        "max" if score > old => {
                            *e.get_mut() = score;
                        }
                        _ => {}
                    }
                }
            }
        }
    }

    if intersect {
        sset.retain(|member, _| counts.get(member).copied().unwrap_or(0) == keys.len());
    }

    // Sort by score, then by member
    let mut elems: Vec<(String, f64)> = sset.into_iter().collect();
    elems.sort_by(|a, b| {
        a.1.partial_cmp(&b.1)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| a.0.cmp(&b.0))
    });

    let mut result = Vec::new();
    for (member, score) in &elems {
        result.push(Frame::Bulk(member.clone().into()));
        if with_scores {
            result.push(Frame::Bulk(write_float(*score).into()));
        }
    }
    Frame::Array(result)
}

/// ZINTER numkeys key [...] [WEIGHTS ...] [AGGREGATE ...] [WITHSCORES]
fn cmd_zinter(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zop_impl(state, ctx, args, true)
}

/// ZUNION numkeys key [...] [WEIGHTS ...] [AGGREGATE ...] [WITHSCORES]
fn cmd_zunion(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    zop_impl(state, ctx, args, false)
}

/// ZRANDMEMBER key [count [WITHSCORES]]
fn cmd_zrandmember(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() > 3 {
        return Frame::error(err_wrong_number("zrandmember"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut count: i64 = 0;
    let mut with_count = false;
    let mut with_scores = false;

    if args.len() >= 2 {
        match parse_int(&args[1]) {
            Some(n) => {
                count = n;
                with_count = true;
            }
            None => return Frame::error(MSG_INVALID_INT),
        }
    }
    if args.len() == 3 {
        let opt = String::from_utf8_lossy(&args[2]).to_uppercase();
        if opt == "WITHSCORES" {
            with_scores = true;
        } else {
            return Frame::error(MSG_SYNTAX_ERROR);
        }
    }

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if !db.keys.contains_key(key.as_ref()) {
        return if with_count {
            Frame::Array(vec![])
        } else {
            Frame::Null
        };
    }

    if let Some(t) = db.key_type(&key)
        && t != crate::types::KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let ss = match db.sorted_set_keys.get(key.as_ref()) {
        Some(ss) => ss,
        None => {
            return if with_count {
                Frame::Array(vec![])
            } else {
                Frame::Null
            };
        }
    };

    let mut members = ss.members_sorted();
    // Collect scores before shuffling (avoids borrow issues with inner.rng)
    let scores: std::collections::HashMap<String, f64> = members
        .iter()
        .map(|m| (m.clone(), ss.get(m).unwrap_or(0.0)))
        .collect();

    if count < 0 {
        // Negative count: allow duplicates
        let abs_count = (-count) as usize;
        let mut result = Vec::new();
        for _ in 0..abs_count {
            let idx = inner.rng.random_range(0..members.len());
            result.push(Frame::Bulk(members[idx].clone().into()));
            if with_scores {
                let score = scores.get(&members[idx]).copied().unwrap_or(0.0);
                result.push(Frame::Bulk(write_float(score).into()));
            }
        }
        return Frame::Array(result);
    }

    // Positive count: unique, shuffle
    members.shuffle(&mut inner.rng);
    let take = (count as usize).min(members.len());

    if !with_count {
        return Frame::Bulk(members[0].clone().into());
    }

    let mut result = Vec::new();
    for m in &members[..take] {
        result.push(Frame::Bulk(m.clone().into()));
        if with_scores {
            let score = scores.get(m).copied().unwrap_or(0.0);
            result.push(Frame::Bulk(write_float(score).into()));
        }
    }
    Frame::Array(result)
}
