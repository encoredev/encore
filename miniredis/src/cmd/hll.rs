use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{CommandTable, MSG_NOT_VALID_HLL_VALUE, err_wrong_number};
use crate::frame::Frame;
use crate::types::KeyType;

pub fn register(table: &mut CommandTable) {
    table.add("PFADD", cmd_pfadd, false);
    table.add("PFCOUNT", cmd_pfcount, true);
    table.add("PFMERGE", cmd_pfmerge, false);
}

/// PFADD key element [element ...]
fn cmd_pfadd(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("pfadd"));
    }

    let key = String::from_utf8_lossy(&args[0]).to_string();
    let items: Vec<&str> = args[1..]
        .iter()
        .map(|a| std::str::from_utf8(a).unwrap_or(""))
        .collect();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    // Check type if key already exists
    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::HyperLogLog
    {
        return Frame::error(MSG_NOT_VALID_HLL_VALUE);
    }

    let altered = db.hll_add(&key, &items, now);
    Frame::Integer(altered)
}

/// PFCOUNT key [key ...]
fn cmd_pfcount(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("pfcount"));
    }

    let keys: Vec<&str> = args
        .iter()
        .map(|a| std::str::from_utf8(a).unwrap_or(""))
        .collect();

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    match db.hll_count(&keys) {
        Ok(count) => Frame::Integer(count),
        Err(msg) => Frame::error(msg),
    }
}

/// PFMERGE destkey [sourcekey ...]
fn cmd_pfmerge(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("pfmerge"));
    }

    let keys: Vec<&str> = args
        .iter()
        .map(|a| std::str::from_utf8(a).unwrap_or(""))
        .collect();

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    match db.hll_merge(&keys, now) {
        Ok(()) => Frame::ok(),
        Err(msg) => Frame::error(msg),
    }
}
