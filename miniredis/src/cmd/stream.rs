use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{CommandTable, MSG_WRONG_TYPE, err_wrong_number};
use crate::frame::Frame;
use crate::types::{KeyType, Stream, format_stream_range_bound};

pub fn register(table: &mut CommandTable) {
    table.add("XADD", cmd_xadd, false, -5);
    table.add("XLEN", cmd_xlen, true, 2);
    table.add("XRANGE", cmd_xrange, true, -4);
    table.add("XREVRANGE", cmd_xrevrange, true, -4);
    table.add("XREAD", cmd_xread, true, -4);
    table.add("XINFO", cmd_xinfo, true, -2);
    table.add("XDEL", cmd_xdel, false, -3);
    table.add("XTRIM", cmd_xtrim, false, -4);
    table.add("XGROUP", cmd_xgroup, false, -2);
    table.add("XREADGROUP", cmd_xreadgroup, false, -7);
    table.add("XACK", cmd_xack, false, -4);
    table.add("XPENDING", cmd_xpending, true, -3);
    table.add("XCLAIM", cmd_xclaim, false, -6);
    table.add("XAUTOCLAIM", cmd_xautoclaim, false, -6);
}

/// XADD key [NOMKSTREAM] [MAXLEN|MINID [=|~] threshold] id field value [field value ...]
fn cmd_xadd(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let mut i = 1;
    let mut nomkstream = false;
    let mut maxlen: Option<usize> = None;
    let mut minid: Option<String> = None;

    // Parse options
    while i < args.len() {
        let arg = String::from_utf8_lossy(&args[i]).to_uppercase();
        match arg.as_str() {
            "NOMKSTREAM" => {
                nomkstream = true;
                i += 1;
            }
            "MAXLEN" => {
                i += 1;
                if i < args.len() {
                    let next = String::from_utf8_lossy(&args[i]).to_string();
                    if next == "~" || next == "=" {
                        i += 1;
                    }
                }
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(n) if n >= 0 => {
                        maxlen = Some(n as usize);
                        i += 1;
                    }
                    Ok(_) => {
                        return Frame::error("ERR The MAXLEN argument must be >= 0.");
                    }
                    Err(_) => {
                        return Frame::error("ERR value is not an integer or out of range");
                    }
                }
            }
            "MINID" => {
                i += 1;
                if i < args.len() {
                    let next = String::from_utf8_lossy(&args[i]).to_string();
                    if next == "~" || next == "=" {
                        i += 1;
                    }
                }
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                minid = Some(String::from_utf8_lossy(&args[i]).to_string());
                i += 1;
            }
            _ => break,
        }
    }

    if i >= args.len() {
        return Frame::error(err_wrong_number("xadd"));
    }

    let id = String::from_utf8_lossy(&args[i]).to_string();
    i += 1;

    // Remaining args are field-value pairs
    let remaining = &args[i..];
    if !remaining.len().is_multiple_of(2) {
        return Frame::error(err_wrong_number("xadd"));
    }

    let values: Vec<String> = remaining
        .iter()
        .map(|a| String::from_utf8_lossy(a).to_string())
        .collect();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let ms = now
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64;
    let db = inner.db_mut(ctx.selected_db);

    // Type check
    if let Some(kt) = db.keys.get(&key) {
        if *kt != KeyType::Stream {
            return Frame::error(MSG_WRONG_TYPE);
        }
    } else if nomkstream {
        return Frame::Null;
    }

    db.keys.entry(key.clone()).or_insert(KeyType::Stream);
    let stream = db.stream_keys.entry(key.clone()).or_default();

    match stream.add(&id, values, ms) {
        Ok(final_id) => {
            if let Some(ml) = maxlen {
                stream.trim_maxlen(ml);
            }
            if let Some(mi) = minid {
                let normalized = Stream::normalize_id(&mi);
                stream.trim_minid(&normalized);
            }
            db.incr_version(&key, now);
            Frame::Bulk(final_id.into())
        }
        Err(e) => Frame::error(e),
    }
}

/// XLEN key
fn cmd_xlen(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if let Some(kt) = db.keys.get(key.as_ref())
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.stream_keys.get(key.as_ref()) {
        Some(stream) => Frame::Integer(stream.entries.len() as i64),
        None => Frame::Integer(0),
    }
}

/// XRANGE key start end [COUNT count]
fn cmd_xrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xrange_impl(state, ctx, args, false)
}

/// XREVRANGE key end start [COUNT count]
fn cmd_xrevrange(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_xrange_impl(state, ctx, args, true)
}

fn cmd_xrange_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    reverse: bool,
) -> Frame {
    let key = String::from_utf8_lossy(&args[0]);
    let arg_start = String::from_utf8_lossy(&args[1]).to_string();
    let arg_end = String::from_utf8_lossy(&args[2]).to_string();

    let mut count: Option<usize> = None;
    if args.len() > 3 {
        if args.len() != 5 {
            return Frame::error("ERR syntax error");
        }
        let opt = String::from_utf8_lossy(&args[3]).to_uppercase();
        if opt != "COUNT" {
            return Frame::error("ERR syntax error");
        }
        match String::from_utf8_lossy(&args[4]).parse::<usize>() {
            Ok(n) => count = Some(n),
            Err(_) => {
                return Frame::error("ERR value is not an integer or out of range");
            }
        }
    }

    let (start, end) = if reverse {
        let s = match format_stream_range_bound(&arg_end, true) {
            Ok(s) => s,
            Err(e) => return Frame::error(e),
        };
        let e = match format_stream_range_bound(&arg_start, false) {
            Ok(e) => e,
            Err(e) => return Frame::error(e),
        };
        (s, e)
    } else {
        let s = match format_stream_range_bound(&arg_start, true) {
            Ok(s) => s,
            Err(e) => return Frame::error(e),
        };
        let e = match format_stream_range_bound(&arg_end, false) {
            Ok(e) => e,
            Err(e) => return Frame::error(e),
        };
        (s, e)
    };

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if let Some(kt) = db.keys.get(key.as_ref())
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let stream = match db.stream_keys.get(key.as_ref()) {
        Some(s) => s,
        None => return Frame::Array(vec![]),
    };

    let entries = if reverse {
        stream.rev_range(&start, &end, count)
    } else {
        stream.range(&start, &end, count)
    };

    Frame::Array(
        entries
            .into_iter()
            .map(|e| {
                let vals: Vec<Frame> = e
                    .values
                    .iter()
                    .map(|v| Frame::Bulk(v.clone().into()))
                    .collect();
                Frame::Array(vec![Frame::Bulk(e.id.clone().into()), Frame::Array(vals)])
            })
            .collect(),
    )
}

/// XREAD [COUNT count] [BLOCK ms] STREAMS key [key ...] id [id ...]
fn cmd_xread(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let mut i = 0;
    let mut count: Option<usize> = None;

    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "COUNT" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<usize>() {
                    Ok(n) => count = Some(n),
                    Err(_) => {
                        return Frame::error("ERR value is not an integer or out of range");
                    }
                }
                i += 1;
            }
            "BLOCK" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(n) if n < 0 => {
                        return Frame::error("ERR timeout is negative");
                    }
                    Ok(_) => {} // Accept but don't actually block
                    Err(_) => {
                        return Frame::error("ERR timeout is not an integer or out of range");
                    }
                }
                i += 1;
            }
            "STREAMS" => {
                i += 1;
                break;
            }
            _ => {
                return Frame::error("ERR syntax error");
            }
        }
    }

    let remaining = &args[i..];
    if remaining.is_empty() || !remaining.len().is_multiple_of(2) {
        return Frame::error(
            "ERR Unbalanced 'xread' list of streams: for each stream key an ID or '$' must be specified.",
        );
    }

    let half = remaining.len() / 2;
    let keys: Vec<String> = remaining[..half]
        .iter()
        .map(|a| String::from_utf8_lossy(a).to_string())
        .collect();

    let inner = state.lock();

    let mut ids = Vec::with_capacity(half);
    for (idx, a) in remaining[half..].iter().enumerate() {
        let s = String::from_utf8_lossy(a).to_string();
        if s == "$" {
            // Get current last ID for this stream
            let db = inner.db(ctx.selected_db);
            ids.push(
                db.stream_keys
                    .get(&keys[idx])
                    .map(|stream| stream.last_id().to_string())
                    .unwrap_or_else(|| "0-0".to_string()),
            );
        } else {
            let normalized = Stream::normalize_id(&s);
            if Stream::parse_id(&normalized).is_err() {
                return Frame::error("ERR Invalid stream ID specified as stream command argument");
            }
            ids.push(normalized);
        }
    }

    let db = inner.db(ctx.selected_db);
    let mut results = Vec::new();
    let mut has_data = false;

    for (idx, key) in keys.iter().enumerate() {
        if let Some(kt) = db.keys.get(key)
            && *kt != KeyType::Stream
        {
            return Frame::error(MSG_WRONG_TYPE);
        }

        let entries = match db.stream_keys.get(key) {
            Some(stream) => {
                let mut entries = stream.after(&ids[idx]);
                if let Some(c) = count {
                    entries.truncate(c);
                }
                entries
            }
            None => vec![],
        };

        if entries.is_empty() {
            continue;
        }

        has_data = true;

        let entry_frames: Vec<Frame> = entries
            .into_iter()
            .map(|e| {
                let vals: Vec<Frame> = e
                    .values
                    .iter()
                    .map(|v| Frame::Bulk(v.clone().into()))
                    .collect();
                Frame::Array(vec![Frame::Bulk(e.id.clone().into()), Frame::Array(vals)])
            })
            .collect();

        results.push(Frame::Array(vec![
            Frame::Bulk(key.clone().into()),
            Frame::Array(entry_frames),
        ]));
    }

    if !has_data {
        return Frame::NullArray;
    }

    Frame::Array(results)
}

/// XDEL key id [id ...]
fn cmd_xdel(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let ids: Vec<String> = args[1..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).to_string())
        .collect();
    let id_refs: Vec<&str> = ids.iter().map(|s| s.as_str()).collect();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.stream_keys.get_mut(&key) {
        Some(stream) => {
            // Validate all IDs before deleting
            for id in &id_refs {
                let normalized = Stream::normalize_id(id);
                if Stream::parse_id(&normalized).is_err() {
                    return Frame::error(
                        "ERR Invalid stream ID specified as stream command argument",
                    );
                }
            }
            let count = stream.del(&id_refs);
            db.incr_version(&key, now);
            Frame::Integer(count)
        }
        None => {
            // Non-existing key: return 0 even for invalid IDs
            Frame::Integer(0)
        }
    }
}

/// XTRIM key MAXLEN|MINID [=|~] threshold [LIMIT count]
fn cmd_xtrim(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let strategy = String::from_utf8_lossy(&args[1]).to_uppercase();

    if strategy != "MAXLEN" && strategy != "MINID" {
        return Frame::error(err_wrong_number("xtrim"));
    }

    let mut i = 2;
    let mut approx = false;
    if i < args.len() {
        let next = String::from_utf8_lossy(&args[i]).to_string();
        if next == "~" {
            approx = true;
            i += 1;
        } else if next == "=" {
            i += 1;
        }
    }

    if i >= args.len() {
        return Frame::error(err_wrong_number("xtrim"));
    }

    let threshold = String::from_utf8_lossy(&args[i]).to_string();
    i += 1;

    // Parse optional LIMIT
    if i < args.len() {
        let next = String::from_utf8_lossy(&args[i]).to_uppercase();
        if next == "LIMIT" {
            if !approx {
                return Frame::error(
                    "ERR syntax error, LIMIT cannot be used without the special ~ flag",
                );
            }
            i += 1;
            if i >= args.len() {
                return Frame::error("ERR syntax error");
            }
            // Parse the limit value (we accept it but don't use it for exact behavior)
            match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                Ok(_) => {
                    i += 1;
                }
                Err(_) => {
                    return Frame::error("ERR value is not an integer or out of range");
                }
            }
        }
    }

    if i < args.len() {
        return Frame::error("ERR syntax error");
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let stream = match db.stream_keys.get_mut(&key) {
        Some(s) => s,
        None => return Frame::Integer(0),
    };

    let count = match strategy.as_str() {
        "MAXLEN" => match threshold.parse::<i64>() {
            Ok(n) if n >= 0 => stream.trim_maxlen(n as usize),
            _ => {
                return Frame::error("ERR value is not an integer or out of range");
            }
        },
        "MINID" => {
            let normalized = Stream::normalize_id(&threshold);
            stream.trim_minid(&normalized)
        }
        _ => {
            return Frame::error("ERR syntax error");
        }
    };

    db.incr_version(&key, now);
    Frame::Integer(count)
}

/// XGROUP CREATE/DESTROY/CREATECONSUMER/DELCONSUMER
fn cmd_xgroup(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "CREATE" => {
            if args.len() < 3 {
                return Frame::error(err_wrong_number("xgroup|create"));
            }
            let key = String::from_utf8_lossy(&args[1]).to_string();
            let group = String::from_utf8_lossy(&args[2]).to_string();
            let id = if args.len() > 3 {
                String::from_utf8_lossy(&args[3]).to_string()
            } else {
                "$".to_string()
            };

            let mkstream =
                args.len() > 4 && String::from_utf8_lossy(&args[4]).to_uppercase() == "MKSTREAM";

            let mut inner = state.lock();
            let now = inner.effective_now();
            let db = inner.db_mut(ctx.selected_db);

            if let Some(kt) = db.keys.get(&key) {
                if *kt != KeyType::Stream {
                    return Frame::error(MSG_WRONG_TYPE);
                }
            } else if !mkstream {
                return Frame::error(
                    "ERR The XGROUP subcommand requires the key to exist. Note that for CREATE you may want to use the MKSTREAM option to create an empty stream automatically.",
                );
            } else {
                db.keys.insert(key.clone(), KeyType::Stream);
                db.stream_keys.insert(key.clone(), Stream::new());
            }

            let stream = db.stream_keys.get_mut(&key).unwrap();
            match stream.create_group(&group, &id) {
                Ok(()) => {
                    db.incr_version(&key, now);
                    Frame::ok()
                }
                Err(e) => Frame::error(e),
            }
        }
        "DESTROY" => {
            if args.len() < 3 {
                return Frame::error(err_wrong_number("xgroup|destroy"));
            }
            let key = String::from_utf8_lossy(&args[1]).to_string();
            let group = String::from_utf8_lossy(&args[2]).to_string();

            let mut inner = state.lock();
            let now = inner.effective_now();
            let db = inner.db_mut(ctx.selected_db);

            if let Some(kt) = db.keys.get(&key)
                && *kt != KeyType::Stream
            {
                return Frame::error(MSG_WRONG_TYPE);
            }

            let stream = match db.stream_keys.get_mut(&key) {
                Some(s) => s,
                None => {
                    return Frame::error(
                        "ERR The XGROUP subcommand requires the key to exist. Note that for CREATE you may want to use the MKSTREAM option to create an empty stream automatically.",
                    );
                }
            };

            if stream.groups.remove(&group).is_some() {
                db.incr_version(&key, now);
                Frame::Integer(1)
            } else {
                Frame::Integer(0)
            }
        }
        "CREATECONSUMER" => {
            if args.len() < 4 {
                return Frame::error(err_wrong_number("xgroup|createconsumer"));
            }
            let key = String::from_utf8_lossy(&args[1]).to_string();
            let group_name = String::from_utf8_lossy(&args[2]).to_string();
            let consumer_name = String::from_utf8_lossy(&args[3]).to_string();

            let mut inner = state.lock();
            let now = inner.effective_now();
            let db = inner.db_mut(ctx.selected_db);

            if let Some(kt) = db.keys.get(&key)
                && *kt != KeyType::Stream
            {
                return Frame::error(MSG_WRONG_TYPE);
            }

            let stream = match db.stream_keys.get_mut(&key) {
                Some(s) => s,
                None => {
                    return Frame::error("ERR The XGROUP subcommand requires the key to exist.");
                }
            };

            let group = match stream.groups.get_mut(&group_name) {
                Some(g) => g,
                None => {
                    return Frame::error(format!(
                        "NOGROUP No such consumer group '{}' for key name '{}'",
                        group_name, key
                    ));
                }
            };

            if let std::collections::hash_map::Entry::Vacant(e) =
                group.consumers.entry(consumer_name)
            {
                e.insert(crate::types::StreamConsumer {
                    num_pending: 0,
                    last_seen: now,
                    last_success: now,
                });
                Frame::Integer(1)
            } else {
                Frame::Integer(0)
            }
        }
        "DELCONSUMER" => {
            if args.len() < 4 {
                return Frame::error(err_wrong_number("xgroup|delconsumer"));
            }
            let key = String::from_utf8_lossy(&args[1]).to_string();
            let group_name = String::from_utf8_lossy(&args[2]).to_string();
            let consumer_name = String::from_utf8_lossy(&args[3]).to_string();

            let mut inner = state.lock();
            let now = inner.effective_now();
            let db = inner.db_mut(ctx.selected_db);

            if let Some(kt) = db.keys.get(&key)
                && *kt != KeyType::Stream
            {
                return Frame::error(MSG_WRONG_TYPE);
            }

            let stream = match db.stream_keys.get_mut(&key) {
                Some(s) => s,
                None => {
                    return Frame::error("ERR The XGROUP subcommand requires the key to exist.");
                }
            };

            let group = match stream.groups.get_mut(&group_name) {
                Some(g) => g,
                None => {
                    return Frame::error(format!(
                        "NOGROUP No such consumer group '{}' for key name '{}'",
                        group_name, key
                    ));
                }
            };

            let pending_count = group
                .pending
                .iter()
                .filter(|pe| pe.consumer == consumer_name)
                .count() as i64;
            group.pending.retain(|pe| pe.consumer != consumer_name);
            group.consumers.remove(&consumer_name);
            db.incr_version(&key, now);
            Frame::Integer(pending_count)
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try XGROUP HELP.",
            String::from_utf8_lossy(&args[0])
        )),
    }
}

/// XREADGROUP GROUP group consumer [COUNT count] [BLOCK ms] [NOACK] STREAMS key [key ...] id [id ...]
fn cmd_xreadgroup(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let mut i = 0;
    let group_kw = String::from_utf8_lossy(&args[i]).to_uppercase();
    if group_kw != "GROUP" {
        return Frame::error("ERR syntax error");
    }
    i += 1;

    let group_name = String::from_utf8_lossy(&args[i]).to_string();
    i += 1;
    let consumer_name = String::from_utf8_lossy(&args[i]).to_string();
    i += 1;

    let mut count: Option<usize> = None;
    let mut noack = false;

    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "COUNT" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(n) if n > 0 => count = Some(n as usize),
                    Ok(_) => {
                        // Negative or zero COUNT: treat as unlimited (no count limit)
                        count = None;
                    }
                    Err(_) => {
                        return Frame::error("ERR value is not an integer or out of range");
                    }
                }
                i += 1;
            }
            "BLOCK" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(n) if n < 0 => {
                        return Frame::error("ERR timeout is negative");
                    }
                    Ok(_) => {} // Accept but don't actually block
                    Err(_) => {
                        return Frame::error("ERR timeout is not an integer or out of range");
                    }
                }
                i += 1;
            }
            "NOACK" => {
                noack = true;
                i += 1;
            }
            "STREAMS" => {
                i += 1;
                break;
            }
            _ => {
                return Frame::error("ERR syntax error");
            }
        }
    }

    let remaining = &args[i..];
    if remaining.is_empty() || !remaining.len().is_multiple_of(2) {
        return Frame::error(
            "ERR Unbalanced XREADGROUP list of streams: for each stream key an ID or '$' must be specified.",
        );
    }

    let half = remaining.len() / 2;
    let keys: Vec<String> = remaining[..half]
        .iter()
        .map(|a| String::from_utf8_lossy(a).to_string())
        .collect();

    // Collect IDs (validation deferred to per-stream loop, after group check)
    let mut ids = Vec::with_capacity(half);
    for a in &remaining[half..] {
        ids.push(String::from_utf8_lossy(a).to_string());
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    let mut results = Vec::new();
    let mut has_data = false;

    for (idx, key) in keys.iter().enumerate() {
        if let Some(kt) = db.keys.get(key)
            && *kt != KeyType::Stream
        {
            return Frame::error(MSG_WRONG_TYPE);
        }

        let stream = match db.stream_keys.get_mut(key) {
            Some(s) => s,
            None => {
                return Frame::error(format!(
                    "NOGROUP No such consumer group '{}' for key name '{}'",
                    group_name, key
                ));
            }
        };

        // Check group exists before ID validation
        if !stream.groups.contains_key(&group_name) {
            return Frame::error(format!(
                "NOGROUP No such consumer group '{}' for key name '{}'",
                group_name, key
            ));
        }

        // Validate non-">" IDs after confirming the group exists
        if ids[idx] != ">" && ids[idx] != "$" {
            let normalized = Stream::normalize_id(&ids[idx]);
            if Stream::parse_id(&normalized).is_err() {
                return Frame::error("ERR Invalid stream ID specified as stream command argument");
            }
        }

        let entries =
            match stream.read_group(&group_name, &consumer_name, &ids[idx], count, noack, now) {
                Ok(entries) => entries,
                Err(e) => return Frame::error(e),
            };

        // For ">" IDs, omit streams with no new entries from results
        if entries.is_empty() && ids[idx] == ">" {
            continue;
        }

        if !entries.is_empty() {
            has_data = true;
        }

        let entry_frames: Vec<Frame> = entries
            .into_iter()
            .map(|e| {
                let vals: Vec<Frame> = e
                    .values
                    .iter()
                    .map(|v| Frame::Bulk(v.clone().into()))
                    .collect();
                Frame::Array(vec![Frame::Bulk(e.id.into()), Frame::Array(vals)])
            })
            .collect();

        results.push(Frame::Array(vec![
            Frame::Bulk(key.clone().into()),
            Frame::Array(entry_frames),
        ]));
    }

    if !has_data && ids.iter().all(|id| id == ">") {
        return Frame::NullArray;
    }

    Frame::Array(results)
}

/// XACK key group id [id ...]
fn cmd_xack(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let group_name = String::from_utf8_lossy(&args[1]).to_string();
    let ids: Vec<String> = args[2..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).to_string())
        .collect();

    // Validate all IDs
    for id in &ids {
        let normalized = Stream::normalize_id(id);
        if Stream::parse_id(&normalized).is_err() {
            return Frame::error("ERR Invalid stream ID specified as stream command argument");
        }
    }

    let id_refs: Vec<&str> = ids.iter().map(|s| s.as_str()).collect();

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let stream = match db.stream_keys.get_mut(&key) {
        Some(s) => s,
        None => return Frame::Integer(0),
    };

    match stream.ack(&group_name, &id_refs) {
        Ok(count) => Frame::Integer(count),
        Err(e) => Frame::error(e),
    }
}

/// XPENDING key group [[IDLE ms] start end count [consumer]]
fn cmd_xpending(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let group_name = String::from_utf8_lossy(&args[1]).to_string();

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let stream = match db.stream_keys.get(&key) {
        Some(s) => s,
        None => {
            return Frame::error(format!(
                "NOGROUP No such consumer group '{}' for key name '{}'",
                group_name, key
            ));
        }
    };

    let group = match stream.groups.get(&group_name) {
        Some(g) => g,
        None => {
            return Frame::error(format!(
                "NOGROUP No such consumer group '{}' for key name '{}'",
                group_name, key
            ));
        }
    };

    if args.len() == 2 {
        // Summary mode
        let active: Vec<&crate::types::PendingEntry> = group
            .pending
            .iter()
            .filter(|pe| stream.entries.iter().any(|e| e.id == pe.id))
            .collect();

        if active.is_empty() {
            return Frame::Array(vec![
                Frame::Integer(0),
                Frame::Null,
                Frame::Null,
                Frame::NullArray,
            ]);
        }

        let min_id = active
            .iter()
            .map(|pe| &pe.id)
            .min_by(|a, b| Stream::cmp_ids(a, b))
            .unwrap();
        let max_id = active
            .iter()
            .map(|pe| &pe.id)
            .max_by(|a, b| Stream::cmp_ids(a, b))
            .unwrap();

        // Count per consumer
        let mut consumer_counts: std::collections::HashMap<&str, i64> =
            std::collections::HashMap::new();
        for pe in &active {
            *consumer_counts.entry(&pe.consumer).or_insert(0) += 1;
        }
        let mut consumers: Vec<Frame> = consumer_counts
            .iter()
            .map(|(name, count)| {
                Frame::Array(vec![
                    Frame::Bulk(name.to_string().into()),
                    Frame::Bulk(count.to_string().into()),
                ])
            })
            .collect();
        consumers.sort_by(|a, b| {
            if let (Frame::Array(a), Frame::Array(b)) = (a, b)
                && let (Frame::Bulk(a), Frame::Bulk(b)) = (&a[0], &b[0])
            {
                return a.cmp(b);
            }
            std::cmp::Ordering::Equal
        });

        return Frame::Array(vec![
            Frame::Integer(active.len() as i64),
            Frame::Bulk(min_id.clone().into()),
            Frame::Bulk(max_id.clone().into()),
            Frame::Array(consumers),
        ]);
    }

    // Detail mode: XPENDING key group [IDLE ms] start end count [consumer]
    let mut i = 2;
    let mut idle_filter: Option<u64> = None;

    if i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        if opt == "IDLE" {
            i += 1;
            if i >= args.len() {
                return Frame::error("ERR syntax error");
            }
            match String::from_utf8_lossy(&args[i]).parse::<u64>() {
                Ok(n) => idle_filter = Some(n),
                Err(_) => {
                    return Frame::error("ERR value is not an integer or out of range");
                }
            }
            i += 1;
        }
    }

    if i + 3 > args.len() {
        return Frame::error("ERR syntax error");
    }

    let start = match format_stream_range_bound(&String::from_utf8_lossy(&args[i]), true) {
        Ok(s) => s,
        Err(e) => return Frame::error(e),
    };
    let end = match format_stream_range_bound(&String::from_utf8_lossy(&args[i + 1]), false) {
        Ok(e) => e,
        Err(e) => return Frame::error(e),
    };
    let count_val = match String::from_utf8_lossy(&args[i + 2]).parse::<i64>() {
        Ok(n) => n,
        Err(_) => {
            return Frame::error("ERR value is not an integer or out of range");
        }
    };

    let consumer_filter = if i + 3 < args.len() {
        Some(String::from_utf8_lossy(&args[i + 3]).to_string())
    } else {
        None
    };

    if count_val <= 0 {
        return Frame::Array(vec![]);
    }

    let now = inner.effective_now();
    let mut result = Vec::new();

    for pe in &group.pending {
        if !stream.entries.iter().any(|e| e.id == pe.id) {
            continue;
        }
        if Stream::cmp_ids(&pe.id, &start) == std::cmp::Ordering::Less {
            continue;
        }
        if Stream::cmp_ids(&pe.id, &end) == std::cmp::Ordering::Greater {
            continue;
        }
        if let Some(consumer) = &consumer_filter
            && pe.consumer != *consumer
        {
            continue;
        }
        let idle_ms = now
            .duration_since(pe.last_delivery)
            .unwrap_or_default()
            .as_millis() as u64;
        if let Some(min_idle) = idle_filter
            && idle_ms < min_idle
        {
            continue;
        }

        result.push(Frame::Array(vec![
            Frame::Bulk(pe.id.clone().into()),
            Frame::Bulk(pe.consumer.clone().into()),
            Frame::Integer(idle_ms as i64),
            Frame::Integer(pe.delivery_count),
        ]));

        if result.len() >= count_val as usize {
            break;
        }
    }

    Frame::Array(result)
}

/// XCLAIM key group consumer min-idle-ms id [id ...] [IDLE ms] [TIME ms] [RETRYCOUNT count] [FORCE] [JUSTID]
fn cmd_xclaim(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let group_name = String::from_utf8_lossy(&args[1]).to_string();
    let consumer_name = String::from_utf8_lossy(&args[2]).to_string();
    let _min_idle_ms = match String::from_utf8_lossy(&args[3]).parse::<u64>() {
        Ok(n) => n,
        Err(_) => {
            return Frame::error("ERR Invalid min-idle-time argument for XCLAIM");
        }
    };

    let mut ids = Vec::new();
    let mut justid = false;
    let mut force = false;
    let mut in_options = false;
    let mut i = 4;

    while i < args.len() {
        let arg = String::from_utf8_lossy(&args[i]).to_uppercase();
        match arg.as_str() {
            "JUSTID" => {
                in_options = true;
                justid = true;
                i += 1;
            }
            "FORCE" => {
                in_options = true;
                force = true;
                i += 1;
            }
            "IDLE" => {
                in_options = true;
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(_) => {}
                    Err(_) => {
                        return Frame::error("ERR Invalid IDLE option argument for XCLAIM");
                    }
                }
                i += 1;
            }
            "TIME" => {
                in_options = true;
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(_) => {}
                    Err(_) => {
                        return Frame::error("ERR Invalid TIME option argument for XCLAIM");
                    }
                }
                i += 1;
            }
            "RETRYCOUNT" => {
                in_options = true;
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                    Ok(_) => {}
                    Err(_) => {
                        return Frame::error("ERR Invalid RETRYCOUNT option argument for XCLAIM");
                    }
                }
                i += 1;
            }
            _ => {
                if in_options {
                    return Frame::error(format!(
                        "ERR Unrecognized XCLAIM option '{}'",
                        String::from_utf8_lossy(&args[i])
                    ));
                }
                ids.push(String::from_utf8_lossy(&args[i]).to_string());
                i += 1;
            }
        }
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let stream = match db.stream_keys.get_mut(&key) {
        Some(s) => s,
        None => {
            return Frame::error(format!(
                "NOGROUP No such key '{}' or consumer group '{}' in XCLAIM for key name '{}'",
                key, group_name, key
            ));
        }
    };

    let group = match stream.groups.get_mut(&group_name) {
        Some(g) => g,
        None => {
            return Frame::error(format!(
                "NOGROUP No such key '{}' or consumer group '{}' in XCLAIM for key name '{}'",
                key, group_name, key
            ));
        }
    };

    // Ensure consumer exists
    group
        .consumers
        .entry(consumer_name.clone())
        .or_insert(crate::types::StreamConsumer {
            num_pending: 0,
            last_seen: now,
            last_success: now,
        });

    let mut claimed = Vec::new();
    for id in &ids {
        let entry_exists = stream.entries.iter().any(|e| e.id == *id);
        let in_pel = group.pending.iter().any(|pe| pe.id == *id);

        if !entry_exists && !force && !in_pel {
            // Entry doesn't exist, not forced, and not in PEL: skip
            continue;
        }

        if !entry_exists && in_pel && !force {
            // Entry was deleted but is still in PEL: remove from PEL
            let consumer_name_of_pe = group
                .pending
                .iter()
                .find(|pe| pe.id == *id)
                .map(|pe| pe.consumer.clone());
            group.pending.retain(|pe| pe.id != *id);
            if let Some(cname) = consumer_name_of_pe
                && let Some(c) = group.consumers.get_mut(&cname)
            {
                c.num_pending -= 1;
            }
            continue;
        }

        if !entry_exists && !force {
            continue;
        }

        // Find in pending or create if force
        let found = group.pending.iter_mut().find(|pe| pe.id == *id);
        match found {
            Some(pe) => {
                // Transfer to new consumer
                let old_consumer = pe.consumer.clone();
                pe.consumer = consumer_name.clone();
                pe.delivery_count += 1;
                pe.last_delivery = now;

                // Update consumer pending counts
                if let Some(c) = group.consumers.get_mut(&old_consumer) {
                    c.num_pending -= 1;
                }
                if let Some(c) = group.consumers.get_mut(&consumer_name) {
                    c.num_pending += 1;
                }
            }
            None => {
                if force {
                    group.pending.push(crate::types::PendingEntry {
                        id: id.clone(),
                        consumer: consumer_name.clone(),
                        delivery_count: 1,
                        last_delivery: now,
                    });
                    if let Some(c) = group.consumers.get_mut(&consumer_name) {
                        c.num_pending += 1;
                    }
                } else {
                    continue;
                }
            }
        }

        if justid {
            claimed.push(Frame::Bulk(id.clone().into()));
        } else if let Some(entry) = stream.entries.iter().find(|e| e.id == *id) {
            let vals: Vec<Frame> = entry
                .values
                .iter()
                .map(|v| Frame::Bulk(v.clone().into()))
                .collect();
            claimed.push(Frame::Array(vec![
                Frame::Bulk(entry.id.clone().into()),
                Frame::Array(vals),
            ]));
        }
    }

    Frame::Array(claimed)
}

/// XAUTOCLAIM key group consumer min-idle-ms start [COUNT count] [JUSTID]
fn cmd_xautoclaim(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = String::from_utf8_lossy(&args[0]).to_string();
    let group_name = String::from_utf8_lossy(&args[1]).to_string();
    let consumer_name = String::from_utf8_lossy(&args[2]).to_string();
    let min_idle_ms = match String::from_utf8_lossy(&args[3]).parse::<u64>() {
        Ok(n) => n,
        Err(_) => {
            return Frame::error("ERR Invalid min-idle-time argument for XAUTOCLAIM");
        }
    };
    let start = String::from_utf8_lossy(&args[4]).to_string();
    let start_id = Stream::normalize_id(&start);
    // Validate the start ID
    if Stream::parse_id(&start_id).is_err() {
        return Frame::error("ERR Invalid stream ID specified as stream command argument");
    }

    let mut count: usize = 100;
    let mut justid = false;

    let mut i = 5;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "COUNT" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error("ERR syntax error");
                }
                match String::from_utf8_lossy(&args[i]).parse::<usize>() {
                    Ok(n) => count = n,
                    Err(_) => {
                        return Frame::error("ERR value is not an integer or out of range");
                    }
                }
                i += 1;
            }
            "JUSTID" => {
                justid = true;
                i += 1;
            }
            _ => {
                return Frame::error("ERR syntax error");
            }
        }
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::Stream
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let stream = match db.stream_keys.get_mut(&key) {
        Some(s) => s,
        None => {
            return Frame::error(format!(
                "NOGROUP No such key '{}' or consumer group '{}' in XAUTOCLAIM for key name '{}'",
                key, group_name, key
            ));
        }
    };

    let group = match stream.groups.get_mut(&group_name) {
        Some(g) => g,
        None => {
            return Frame::error(format!(
                "NOGROUP No such key '{}' or consumer group '{}' in XAUTOCLAIM for key name '{}'",
                key, group_name, key
            ));
        }
    };

    // Ensure consumer exists
    group
        .consumers
        .entry(consumer_name.clone())
        .or_insert(crate::types::StreamConsumer {
            num_pending: 0,
            last_seen: now,
            last_success: now,
        });

    let mut claimed = Vec::new();
    let mut last_claimed_id: Option<String> = None;
    let mut hit_count_limit = false;

    for pe in group.pending.iter_mut() {
        if Stream::cmp_ids(&pe.id, &start_id) == std::cmp::Ordering::Less {
            continue;
        }

        let idle_ms = now
            .duration_since(pe.last_delivery)
            .unwrap_or_default()
            .as_millis() as u64;
        if idle_ms < min_idle_ms {
            continue;
        }

        if !stream.entries.iter().any(|e| e.id == pe.id) {
            continue;
        }

        // Claim this entry
        let old_consumer = pe.consumer.clone();
        pe.consumer = consumer_name.clone();
        pe.delivery_count += 1;
        pe.last_delivery = now;

        if let Some(c) = group.consumers.get_mut(&old_consumer) {
            c.num_pending -= 1;
        }
        if let Some(c) = group.consumers.get_mut(&consumer_name) {
            c.num_pending += 1;
        }

        if justid {
            claimed.push(Frame::Bulk(pe.id.clone().into()));
        } else if let Some(entry) = stream.entries.iter().find(|e| e.id == pe.id) {
            let vals: Vec<Frame> = entry
                .values
                .iter()
                .map(|v| Frame::Bulk(v.clone().into()))
                .collect();
            claimed.push(Frame::Array(vec![
                Frame::Bulk(entry.id.clone().into()),
                Frame::Array(vals),
            ]));
        }

        last_claimed_id = Some(pe.id.clone());

        if claimed.len() >= count {
            hit_count_limit = true;
            break;
        }
    }

    // Compute next_id: only return a non-zero cursor if we stopped early due to COUNT limit.
    // If we scanned all eligible entries, return "0-0".
    let next_id = if hit_count_limit {
        match last_claimed_id {
            Some(id) => {
                if let Ok((ms, seq)) = Stream::parse_id(&id) {
                    Stream::format_id(ms, seq + 1)
                } else {
                    "0-0".to_string()
                }
            }
            None => "0-0".to_string(),
        }
    } else {
        "0-0".to_string()
    };

    Frame::Array(vec![
        Frame::Bulk(next_id.into()),
        Frame::Array(claimed),
        Frame::Array(vec![]), // deleted entries (not implemented)
    ])
}

/// XINFO STREAM/GROUPS/CONSUMERS
fn cmd_xinfo(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "STREAM" => {
            if args.len() < 2 {
                return Frame::error(err_wrong_number("xinfo|stream"));
            }
            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            if let Some(kt) = db.keys.get(key.as_ref())
                && *kt != KeyType::Stream
            {
                return Frame::error(MSG_WRONG_TYPE);
            }

            let stream = match db.stream_keys.get(key.as_ref()) {
                Some(s) => s,
                None => return Frame::error("ERR no such key"),
            };

            Frame::Array(vec![
                Frame::Bulk("length".into()),
                Frame::Integer(stream.entries.len() as i64),
                Frame::Bulk("groups".into()),
                Frame::Integer(stream.groups.len() as i64),
                Frame::Bulk("last-generated-id".into()),
                Frame::Bulk(stream.last_id().to_string().into()),
            ])
        }
        "GROUPS" => {
            if args.len() < 2 {
                return Frame::error(err_wrong_number("xinfo|groups"));
            }
            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            if let Some(kt) = db.keys.get(key.as_ref())
                && *kt != KeyType::Stream
            {
                return Frame::error(MSG_WRONG_TYPE);
            }

            let stream = match db.stream_keys.get(key.as_ref()) {
                Some(s) => s,
                None => return Frame::error("ERR no such key"),
            };

            let mut groups: Vec<Frame> = stream
                .groups
                .iter()
                .map(|(name, group)| {
                    // Compute entries-read and lag
                    let (entries_read, lag) = compute_entries_read_lag(stream, group);
                    Frame::Array(vec![
                        Frame::Bulk("name".into()),
                        Frame::Bulk(name.clone().into()),
                        Frame::Bulk("consumers".into()),
                        Frame::Integer(group.consumers.len() as i64),
                        Frame::Bulk("pending".into()),
                        Frame::Integer(group.pending.len() as i64),
                        Frame::Bulk("last-delivered-id".into()),
                        Frame::Bulk(group.last_id.clone().into()),
                        Frame::Bulk("entries-read".into()),
                        entries_read,
                        Frame::Bulk("lag".into()),
                        lag,
                    ])
                })
                .collect();
            groups.sort_by(|a, b| {
                if let (Frame::Array(a), Frame::Array(b)) = (a, b)
                    && let (Frame::Bulk(a), Frame::Bulk(b)) = (&a[1], &b[1])
                {
                    return a.cmp(b);
                }
                std::cmp::Ordering::Equal
            });

            Frame::Array(groups)
        }
        "CONSUMERS" => {
            if args.len() < 3 {
                return Frame::error(err_wrong_number("xinfo|consumers"));
            }
            let key = String::from_utf8_lossy(&args[1]);
            let group_name = String::from_utf8_lossy(&args[2]);
            let inner = state.lock();
            let now = inner.effective_now();
            let db = inner.db(ctx.selected_db);

            if let Some(kt) = db.keys.get(key.as_ref())
                && *kt != KeyType::Stream
            {
                return Frame::error(MSG_WRONG_TYPE);
            }

            let stream = match db.stream_keys.get(key.as_ref()) {
                Some(s) => s,
                None => return Frame::error("ERR no such key"),
            };

            let group = match stream.groups.get(group_name.as_ref()) {
                Some(g) => g,
                None => {
                    return Frame::error(format!(
                        "NOGROUP No such consumer group '{}' for key name '{}'",
                        group_name, key
                    ));
                }
            };

            let consumers: Vec<Frame> = group
                .consumers
                .iter()
                .map(|(name, consumer)| {
                    let idle = now
                        .duration_since(consumer.last_seen)
                        .unwrap_or_default()
                        .as_millis() as i64;
                    let inactive = now
                        .duration_since(consumer.last_success)
                        .unwrap_or_default()
                        .as_millis() as i64;
                    Frame::Array(vec![
                        Frame::Bulk("name".into()),
                        Frame::Bulk(name.clone().into()),
                        Frame::Bulk("pending".into()),
                        Frame::Integer(consumer.num_pending),
                        Frame::Bulk("idle".into()),
                        Frame::Integer(idle),
                        Frame::Bulk("inactive".into()),
                        Frame::Integer(inactive),
                    ])
                })
                .collect();

            Frame::Array(consumers)
        }
        _ => Frame::error(
            "ERR unknown subcommand or wrong number of arguments for 'XINFO' command".to_string(),
        ),
    }
}

/// Compute `entries-read` and `lag` for XINFO GROUPS output.
fn compute_entries_read_lag(stream: &Stream, group: &crate::types::StreamGroup) -> (Frame, Frame) {
    // If last_id is "0-0", the group has never delivered anything.
    if group.last_id == "0-0" {
        return (Frame::Null, Frame::Integer(stream.entries.len() as i64));
    }

    // If entries_read_known is false (group was created with $ or a specific ID
    // but never actually delivered entries), return nil for entries-read.
    if !group.entries_read_known {
        // We still know the lag: number of entries after the group's last_id.
        let entries_after = stream
            .entries
            .iter()
            .filter(|e| Stream::cmp_ids(&e.id, &group.last_id) == std::cmp::Ordering::Greater)
            .count() as i64;
        return (Frame::Null, Frame::Integer(entries_after));
    }

    // entries-read: number of entries with id <= group.last_id.
    // Find the position of the first entry after last_id.
    let pos = stream
        .entries
        .iter()
        .position(|e| Stream::cmp_ids(&e.id, &group.last_id) == std::cmp::Ordering::Greater);

    let entries_read = match pos {
        Some(p) => p as i64,
        None => stream.entries.len() as i64, // last_id >= all entries
    };

    let lag = stream.entries.len() as i64 - entries_read;

    (Frame::Integer(entries_read), Frame::Integer(lag))
}
