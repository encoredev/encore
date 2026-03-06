use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::CommandTable;
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("MULTI", cmd_multi, false, 1);
    table.add("DISCARD", cmd_discard, false, 1);
    table.add("WATCH", cmd_watch, true, -2);
    table.add("UNWATCH", cmd_unwatch, false, 1);
    // EXEC is handled directly in dispatch.rs (needs command table access).
}

/// MULTI
fn cmd_multi(_state: &Arc<SharedState>, ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    if ctx.in_tx() {
        return Frame::error("ERR MULTI calls can not be nested");
    }

    ctx.transaction = Some(Vec::new());
    ctx.dirty_transaction = false;
    Frame::ok()
}

/// DISCARD
fn cmd_discard(_state: &Arc<SharedState>, ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    if !ctx.in_tx() {
        return Frame::error("ERR DISCARD without MULTI");
    }

    ctx.transaction = None;
    ctx.watch.clear();
    Frame::ok()
}

/// WATCH key [key ...]
fn cmd_watch(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if ctx.in_tx() {
        return Frame::error("ERR WATCH inside MULTI is not allowed");
    }

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    for arg in args {
        let key = String::from_utf8_lossy(arg).to_string();
        let version = db.key_version.get(&key).copied().unwrap_or(0);
        ctx.watch.insert((ctx.selected_db, key), version);
    }

    Frame::ok()
}

/// UNWATCH
fn cmd_unwatch(_state: &Arc<SharedState>, ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    ctx.watch.clear();
    Frame::ok()
}
