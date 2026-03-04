use std::sync::Arc;

use rand::Rng;
use rand::seq::SliceRandom;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_GT_AND_LT, MSG_INT_OVERFLOW, MSG_INVALID_CURSOR, MSG_INVALID_FLOAT,
    MSG_INVALID_INT, MSG_NUM_FIELDS_INVALID, MSG_NUM_FIELDS_PARAMETER, MSG_NX_AND_XX_GT_LT,
    MSG_SYNTAX_ERROR, MSG_WRONG_TYPE, err_wrong_number,
};
use crate::frame::Frame;
use crate::types::KeyType;

use super::parse_int;

pub fn register(table: &mut CommandTable) {
    table.add("HSET", cmd_hset, false);
    table.add("HSETNX", cmd_hsetnx, false);
    table.add("HMSET", cmd_hmset, false);
    table.add("HGET", cmd_hget, true);
    table.add("HMGET", cmd_hmget, true);
    table.add("HDEL", cmd_hdel, false);
    table.add("HEXISTS", cmd_hexists, true);
    table.add("HGETALL", cmd_hgetall, true);
    table.add("HKEYS", cmd_hkeys, true);
    table.add("HVALS", cmd_hvals, true);
    table.add("HLEN", cmd_hlen, true);
    table.add("HINCRBY", cmd_hincrby, false);
    table.add("HINCRBYFLOAT", cmd_hincrbyfloat, false);
    table.add("HSTRLEN", cmd_hstrlen, true);
    table.add("HSCAN", cmd_hscan, true);
    table.add("HRANDFIELD", cmd_hrandfield, true);
    table.add("HEXPIRE", cmd_hexpire, false);
}

/// HSET key field value [field value ...]
fn cmd_hset(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 3 || args.len().is_multiple_of(2) {
        return Frame::error(err_wrong_number("hset"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let pairs: Vec<(String, Vec<u8>)> = args[1..]
        .chunks_exact(2)
        .map(|c| (String::from_utf8_lossy(&c[0]).into_owned(), c[1].clone()))
        .collect();

    let added = db.hash_set(&key, &pairs, now);
    Frame::Integer(added)
}

/// HSETNX key field value
fn cmd_hsetnx(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("hsetnx"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let field = String::from_utf8_lossy(&args[1]).into_owned();
    let value = args[2].clone();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    // Only set if field doesn't exist
    if let Some(hash) = db.hash_keys.get(&key)
        && hash.contains_key(&field)
    {
        return Frame::Integer(0);
    }

    db.hash_set(&key, &[(field, value)], now);
    Frame::Integer(1)
}

/// HMSET key field value [field value ...]
fn cmd_hmset(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 3 || args.len().is_multiple_of(2) {
        return Frame::error(err_wrong_number("hmset"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let pairs: Vec<(String, Vec<u8>)> = args[1..]
        .chunks_exact(2)
        .map(|c| (String::from_utf8_lossy(&c[0]).into_owned(), c[1].clone()))
        .collect();

    db.hash_set(&key, &pairs, now);
    Frame::ok()
}

/// HGET key field
fn cmd_hget(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("hget"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let field = String::from_utf8_lossy(&args[1]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.hash_get(&key, &field) {
        Some(val) => Frame::Bulk(val.clone().into()),
        None => Frame::Null,
    }
}

/// HMGET key field [field ...]
fn cmd_hmget(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("hmget"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut results = Vec::with_capacity(args.len() - 1);
    for arg in &args[1..] {
        let field = String::from_utf8_lossy(arg);
        match db.hash_get(&key, &field) {
            Some(val) => results.push(Frame::Bulk(val.clone().into())),
            None => results.push(Frame::Null),
        }
    }

    Frame::Array(results)
}

/// HDEL key field [field ...]
fn cmd_hdel(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("hdel"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    if !db.keys.contains_key(&key) {
        return Frame::Integer(0);
    }

    let fields: Vec<String> = args[1..]
        .iter()
        .map(|a| String::from_utf8_lossy(a).into_owned())
        .collect();

    let count = db.hash_del(&key, &fields, now);
    Frame::Integer(count)
}

/// HEXISTS key field
fn cmd_hexists(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("hexists"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let field = String::from_utf8_lossy(&args[1]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.hash_get(&key, &field) {
        Some(_) => Frame::Integer(1),
        None => Frame::Integer(0),
    }
}

/// HGETALL key
fn cmd_hgetall(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("hgetall"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let fields = db.hash_fields(&key);

    if ctx.resp3 {
        let mut pairs = Vec::with_capacity(fields.len());
        for field in &fields {
            let val = db.hash_get(&key, field).cloned().unwrap_or_default();
            pairs.push((Frame::Bulk(field.clone().into()), Frame::Bulk(val.into())));
        }
        Frame::Map(pairs)
    } else {
        let mut result = Vec::with_capacity(fields.len() * 2);
        for field in &fields {
            result.push(Frame::Bulk(field.clone().into()));
            if let Some(val) = db.hash_get(&key, field) {
                result.push(Frame::Bulk(val.clone().into()));
            }
        }
        Frame::Array(result)
    }
}

/// HKEYS key
fn cmd_hkeys(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("hkeys"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let fields = db.hash_fields(&key);
    Frame::Array(fields.into_iter().map(|f| Frame::Bulk(f.into())).collect())
}

/// HVALS key
fn cmd_hvals(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("hvals"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let values = db.hash_values(&key);
    Frame::Array(values.into_iter().map(|v| Frame::Bulk(v.into())).collect())
}

/// HLEN key
fn cmd_hlen(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("hlen"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let len = db.hash_keys.get(key.as_ref()).map(|h| h.len()).unwrap_or(0);
    Frame::Integer(len as i64)
}

/// HINCRBY key field increment
fn cmd_hincrby(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("hincrby"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let field = String::from_utf8_lossy(&args[1]).into_owned();
    let delta: i64 = match String::from_utf8_lossy(&args[2]).parse() {
        Ok(n) => n,
        Err(_) => return Frame::error(MSG_INVALID_INT),
    };

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let current: i64 = match db.hash_get(&key, &field) {
        Some(v) => match String::from_utf8_lossy(v).parse::<i64>() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_INT),
        },
        None => 0,
    };

    let new_val = match current.checked_add(delta) {
        Some(n) => n,
        None => {
            return Frame::error(MSG_INT_OVERFLOW);
        }
    };

    db.hash_set(&key, &[(field, new_val.to_string().into_bytes())], now);
    Frame::Integer(new_val)
}

/// HINCRBYFLOAT key field increment
fn cmd_hincrbyfloat(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 3 {
        return Frame::error(err_wrong_number("hincrbyfloat"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let field = String::from_utf8_lossy(&args[1]).into_owned();
    let delta: f64 = match String::from_utf8_lossy(&args[2]).parse() {
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
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let current: f64 = match db.hash_get(&key, &field) {
        Some(v) => match String::from_utf8_lossy(v).parse::<f64>() {
            Ok(n) => n,
            Err(_) => return Frame::error(MSG_INVALID_FLOAT),
        },
        None => 0.0,
    };

    let new_val = current + delta;
    let formatted = crate::cmd::string::format_float(new_val);
    db.hash_set(&key, &[(field, formatted.as_bytes().to_vec())], now);
    Frame::Bulk(formatted.into_bytes().into())
}

/// HSTRLEN key field
fn cmd_hstrlen(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("hstrlen"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let field = String::from_utf8_lossy(&args[1]);

    let mut inner = state.lock();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    match db.hash_get(&key, &field) {
        Some(val) => Frame::Integer(val.len() as i64),
        None => Frame::Integer(0),
    }
}

/// HSCAN key cursor [MATCH pattern] [COUNT count]
fn cmd_hscan(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("hscan"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let _cursor: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_CURSOR),
    };

    let opts = match super::parse_scan_opts(&args[2..], false) {
        Ok(o) => o,
        Err(e) => return e,
    };

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut fields = db.hash_fields(&key);

    if let Some(ref pat) = opts.pattern {
        fields = crate::keys::match_keys_vec(&fields, pat);
    }

    let mut result = Vec::new();
    for field in &fields {
        result.push(Frame::Bulk(field.clone().into()));
        let val = db.hash_get(&key, field).cloned().unwrap_or_default();
        result.push(Frame::Bulk(val.into()));
    }

    Frame::Array(vec![Frame::Bulk("0".into()), Frame::Array(result)])
}

/// HRANDFIELD key [count [WITHVALUES]]
fn cmd_hrandfield(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() || args.len() > 3 {
        return Frame::error(err_wrong_number("hrandfield"));
    }

    let key = String::from_utf8_lossy(&args[0]);
    let mut count: i64 = 0;
    let mut with_count = false;
    let mut with_values = false;

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
        if opt == "WITHVALUES" {
            with_values = true;
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
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut fields = db.hash_fields(&key);
    if fields.is_empty() {
        return if with_count {
            Frame::Array(vec![])
        } else {
            Frame::Null
        };
    }

    // Collect values before shuffling (avoids borrow issues with inner.rng)
    let field_values: std::collections::HashMap<String, Vec<u8>> = fields
        .iter()
        .map(|f| (f.clone(), db.hash_get(&key, f).cloned().unwrap_or_default()))
        .collect();

    if count < 0 {
        let abs_count = (-count) as usize;
        let mut result = Vec::new();
        for _ in 0..abs_count {
            let idx = inner.rng.random_range(0..fields.len());
            result.push(Frame::Bulk(fields[idx].clone().into()));
            if with_values {
                let val = field_values.get(&fields[idx]).cloned().unwrap_or_default();
                result.push(Frame::Bulk(val.into()));
            }
        }
        return Frame::Array(result);
    }

    fields.shuffle(&mut inner.rng);
    let take = (count as usize).min(fields.len());

    if !with_count {
        return Frame::Bulk(fields[0].clone().into());
    }

    let mut result = Vec::new();
    for f in &fields[..take] {
        result.push(Frame::Bulk(f.clone().into()));
        if with_values {
            let val = field_values.get(f).cloned().unwrap_or_default();
            result.push(Frame::Bulk(val.into()));
        }
    }
    Frame::Array(result)
}

/// HEXPIRE key seconds [NX|XX|GT|LT] FIELDS numfields field [field ...]
fn cmd_hexpire(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 4 {
        return Frame::error(err_wrong_number("hexpire"));
    }

    let key = String::from_utf8_lossy(&args[0]).into_owned();
    let ttl_secs: i64 = match parse_int(&args[1]) {
        Some(n) => n,
        None => return Frame::error(MSG_INVALID_INT),
    };

    let mut nx = false;
    let mut xx = false;
    let mut gt = false;
    let mut lt = false;
    let mut fields: Vec<String> = Vec::new();

    let mut i = 2;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
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
            "FIELDS" => {
                i += 1;
                if i >= args.len() {
                    return Frame::error(MSG_NUM_FIELDS_INVALID);
                }
                let num_fields: i64 = match parse_int(&args[i]) {
                    Some(n) => n,
                    None => return Frame::error(MSG_NUM_FIELDS_INVALID),
                };
                if num_fields <= 0 {
                    return Frame::error(MSG_NUM_FIELDS_INVALID);
                }
                i += 1;
                let num_fields = num_fields as usize;
                if i + num_fields > args.len() {
                    return Frame::error(MSG_NUM_FIELDS_PARAMETER);
                }
                for j in 0..num_fields {
                    fields.push(String::from_utf8_lossy(&args[i + j]).into_owned());
                }
                i += num_fields;
            }
            _ => {
                return Frame::error(
                    "ERR Mandatory argument FIELDS is missing or not at the right position",
                );
            }
        }
    }

    if gt && lt {
        return Frame::error(MSG_GT_AND_LT);
    }
    if nx && (xx || gt || lt) {
        return Frame::error(MSG_NX_AND_XX_GT_LT);
    }

    if fields.is_empty() {
        return Frame::error(
            "ERR Mandatory argument FIELDS is missing or not at the right position",
        );
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);
    db.check_ttl(&key);

    // Key doesn't exist: return -2 for all fields
    if !db.keys.contains_key(&key) {
        return Frame::Array(fields.iter().map(|_| Frame::Integer(-2)).collect());
    }

    if let Some(t) = db.key_type(&key)
        && t != KeyType::Hash
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let new_ttl = std::time::Duration::from_secs(ttl_secs as u64);

    let field_ttls = db.hash_field_ttls.entry(key.clone()).or_default();

    let mut results = Vec::with_capacity(fields.len());
    for field in &fields {
        // Check field exists in hash
        let field_exists = db
            .hash_keys
            .get(&key)
            .is_some_and(|h| h.contains_key(field));
        if !field_exists {
            results.push(Frame::Integer(-2));
            continue;
        }

        let current_ttl = field_ttls.get(field).copied();
        let has_ttl = current_ttl.is_some();

        // NX: set only when field has no expiration
        if nx && has_ttl {
            results.push(Frame::Integer(0));
            continue;
        }

        // XX: set only when field has existing expiration
        if xx && !has_ttl {
            results.push(Frame::Integer(0));
            continue;
        }

        // GT: set only when new TTL > current TTL
        if gt && (!has_ttl || new_ttl <= current_ttl.unwrap()) {
            results.push(Frame::Integer(0));
            continue;
        }

        // LT: set only when new TTL < current TTL (and field has expiration)
        if lt && has_ttl && new_ttl >= current_ttl.unwrap() {
            results.push(Frame::Integer(0));
            continue;
        }

        field_ttls.insert(field.clone(), new_ttl);
        results.push(Frame::Integer(1));
    }

    db.incr_version(&key, now);
    Frame::Array(results)
}
