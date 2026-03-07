use std::sync::Arc;
use std::sync::atomic::Ordering;
use std::time::Duration;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{CommandTable, MSG_INVALID_INT, MSG_SYNTAX_ERROR, err_wrong_number};
use crate::frame::Frame;
use crate::types::KeyType;

/// Raw RESP blob for COMMAND response, captured from Redis 5.0.7.
static COMMAND_RESP: &[u8] = include_bytes!("command_resp.bin");

pub fn register(table: &mut CommandTable) {
    table.add("DBSIZE", cmd_dbsize, true, 1);
    table.add("FLUSHDB", cmd_flushdb, false, -1);
    table.add("FLUSHALL", cmd_flushall, false, -1);
    table.add("COMMAND", cmd_command, true, -1);
    table.add("TIME", cmd_time, true, 1);
    table.add("INFO", cmd_info, true, -1);
    table.add("SWAPDB", cmd_swapdb, false, 3);
    table.add("MEMORY", cmd_memory, true, -2);
    table.add("MINIREDIS.FASTFORWARD", cmd_fastforward, false, 2);
}

/// DBSIZE
fn cmd_dbsize(state: &Arc<SharedState>, ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    let inner = state.lock();
    let db = inner.db(ctx.selected_db);
    Frame::Integer(db.keys.len() as i64)
}

/// FLUSHDB [ASYNC|SYNC]
fn cmd_flushdb(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() > 1 {
        return Frame::error(MSG_SYNTAX_ERROR);
    }
    if args.len() == 1 {
        let opt = String::from_utf8_lossy(&args[0]).to_uppercase();
        if opt != "ASYNC" && opt != "SYNC" {
            return Frame::error(MSG_SYNTAX_ERROR);
        }
    }
    let mut inner = state.lock();
    inner.db_mut(ctx.selected_db).flush();
    Frame::ok()
}

/// FLUSHALL [ASYNC|SYNC]
fn cmd_flushall(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() > 1 {
        return Frame::error(MSG_SYNTAX_ERROR);
    }
    if args.len() == 1 {
        let opt = String::from_utf8_lossy(&args[0]).to_uppercase();
        if opt != "ASYNC" && opt != "SYNC" {
            return Frame::error(MSG_SYNTAX_ERROR);
        }
    }
    let mut inner = state.lock();
    for i in 0..16 {
        inner.db_mut(i).flush();
    }
    Frame::ok()
}

/// COMMAND — returns static command metadata (captured from Redis 5.0.7).
fn cmd_command(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    use std::sync::OnceLock;
    static CACHED: OnceLock<Frame> = OnceLock::new();
    CACHED
        .get_or_init(|| {
            let mut pos = 0;
            parse_resp(COMMAND_RESP, &mut pos)
        })
        .clone()
}

// ── Minimal RESP parser (for the static COMMAND blob) ────────────────

fn parse_resp(data: &[u8], pos: &mut usize) -> Frame {
    match data[*pos] {
        b'*' => {
            *pos += 1;
            let n = parse_resp_int(data, pos);
            let mut items = Vec::with_capacity(n as usize);
            for _ in 0..n {
                items.push(parse_resp(data, pos));
            }
            Frame::Array(items)
        }
        b'$' => {
            *pos += 1;
            let n = parse_resp_int(data, pos) as usize;
            let val = data[*pos..*pos + n].to_vec();
            *pos += n + 2; // skip data + \r\n
            Frame::Bulk(val.into())
        }
        b':' => {
            *pos += 1;
            Frame::Integer(parse_resp_int(data, pos))
        }
        b'+' => {
            *pos += 1;
            let start = *pos;
            while data[*pos] != b'\r' {
                *pos += 1;
            }
            let val = String::from_utf8_lossy(&data[start..*pos]).to_string();
            *pos += 2; // skip \r\n
            Frame::Simple(val)
        }
        _ => {
            *pos += 1;
            Frame::Null
        }
    }
}

fn parse_resp_int(data: &[u8], pos: &mut usize) -> i64 {
    let neg = data[*pos] == b'-';
    if neg {
        *pos += 1;
    }
    let mut val: i64 = 0;
    while data[*pos] != b'\r' {
        val = val * 10 + (data[*pos] - b'0') as i64;
        *pos += 1;
    }
    *pos += 2; // skip \r\n
    if neg { -val } else { val }
}

/// TIME — returns [seconds, microseconds] of server time.
fn cmd_time(state: &Arc<SharedState>, _ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    let inner = state.lock();
    let now = inner.effective_now();
    let since_epoch = now
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default();
    let secs = since_epoch.as_secs();
    let micros = since_epoch.subsec_micros();

    Frame::Array(vec![
        Frame::Bulk(secs.to_string().into()),
        Frame::Bulk(micros.to_string().into()),
    ])
}

/// INFO [section]
fn cmd_info(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() > 1 {
        return Frame::error(err_wrong_number("info"));
    }

    let connected = state.connected_clients.load(Ordering::Relaxed);
    let total_conn = state.total_connections_received.load(Ordering::Relaxed);
    let total_cmds = state.total_commands_processed.load(Ordering::Relaxed);

    let section = if args.len() == 1 {
        String::from_utf8_lossy(&args[0]).to_lowercase()
    } else {
        String::new()
    };

    let want_all = section.is_empty();

    if !want_all && section != "clients" && section != "stats" {
        return Frame::error(format!("ERR section ({}) is not supported", section));
    }

    let mut result = String::new();

    if want_all || section == "clients" {
        result.push_str(&format!("# Clients\r\nconnected_clients:{}\r\n", connected));
    }

    if want_all || section == "stats" {
        result.push_str(&format!(
            "# Stats\r\ntotal_connections_received:{}\r\ntotal_commands_processed:{}\r\n",
            total_conn, total_cmds
        ));
    }

    Frame::Bulk(result.into())
}

/// SWAPDB db1 db2
fn cmd_swapdb(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let db1 = match String::from_utf8_lossy(&args[0]).parse::<i64>() {
        Ok(n) => n,
        Err(_) => return Frame::error("ERR invalid first DB index"),
    };
    let db2 = match String::from_utf8_lossy(&args[1]).parse::<i64>() {
        Ok(n) => n,
        Err(_) => return Frame::error("ERR invalid second DB index"),
    };

    if !(0..16).contains(&db1) {
        return Frame::error("ERR DB index is out of range");
    }
    if !(0..16).contains(&db2) {
        return Frame::error("ERR DB index is out of range");
    }

    let db1 = db1 as usize;
    let db2 = db2 as usize;

    if db1 != db2 {
        let mut inner = state.lock();
        inner.dbs.swap(db1, db2);
    }

    Frame::ok()
}

/// MEMORY USAGE key
fn cmd_memory(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "USAGE" => {
            if args.len() < 2 {
                return Frame::error("ERR wrong number of arguments for 'memory|usage' command");
            }
            if args.len() > 2 {
                return Frame::error(crate::dispatch::MSG_SYNTAX_ERROR);
            }

            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            match db.keys.get(key.as_ref()) {
                None => Frame::Null,
                Some(kt) => {
                    let size = estimate_key_size(db, &key, *kt);
                    Frame::Integer(size as i64)
                }
            }
        }
        "HELP" => Frame::Array(vec![Frame::Bulk(
            "MEMORY USAGE <key> - estimate memory usage of key".into(),
        )]),
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try MEMORY HELP.",
            subcmd.to_lowercase()
        )),
    }
}

/// Estimate the memory usage of a key in bytes (simplified).
fn estimate_key_size(db: &crate::db::RedisDB, key: &str, kt: KeyType) -> usize {
    let key_overhead = 16 + key.len(); // pointer + key string
    let value_size = match kt {
        KeyType::String => db.string_keys.get(key).map(|v| v.len() + 3).unwrap_or(0),
        KeyType::Hash => db
            .hash_keys
            .get(key)
            .map(|h| h.iter().map(|(f, v)| f.len() + v.len() + 16).sum::<usize>() + 16)
            .unwrap_or(0),
        KeyType::List => db
            .list_keys
            .get(key)
            .map(|l| l.iter().map(|v| v.len() + 16).sum::<usize>() + 16)
            .unwrap_or(0),
        KeyType::Set => db
            .set_keys
            .get(key)
            .map(|s| s.iter().map(|m| m.len() + 16).sum::<usize>() + 16)
            .unwrap_or(0),
        KeyType::SortedSet => db
            .sorted_set_keys
            .get(key)
            .map(|ss| ss.card() * 32 + 16)
            .unwrap_or(0),
        KeyType::Stream => 64,
        KeyType::HyperLogLog => {
            // 16384 registers + overhead
            16384 + 24
        }
    };
    key_overhead + value_size
}

/// MINIREDIS.FASTFORWARD <ms>
///
/// Advance mock time by the given number of milliseconds, expiring any keys
/// whose TTL falls to zero. Used by the integration test suite.
fn cmd_fastforward(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let ms: u64 = match String::from_utf8_lossy(&args[0]).parse() {
        Ok(n) => n,
        Err(_) => return Frame::error(MSG_INVALID_INT),
    };

    let mut inner = state.lock();
    inner.fast_forward(Duration::from_millis(ms));
    Frame::ok()
}
