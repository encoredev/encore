use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{CommandTable, err_wrong_number};
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("PUBLISH", cmd_publish, false, 3);
    table.add("PUBSUB", cmd_pubsub, true, -2);
    // SUBSCRIBE/PSUBSCRIBE/UNSUBSCRIBE/PUNSUBSCRIBE are normally handled in
    // server.rs (outside dispatch). These are registered so they can be queued
    // inside MULTI/EXEC.
    table.add("SUBSCRIBE", cmd_subscribe, false, -2);
    table.add("PSUBSCRIBE", cmd_psubscribe, false, -2);
    table.add("UNSUBSCRIBE", cmd_unsubscribe, false, -1);
    table.add("PUNSUBSCRIBE", cmd_punsubscribe, false, -1);
}

/// SUBSCRIBE channel [channel ...] — handler for MULTI/EXEC path.
/// Normal (non-MULTI) SUBSCRIBE is handled directly in server.rs.
fn cmd_subscribe(_state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let mut confirmations = Vec::new();
    for arg in args {
        let channel = String::from_utf8_lossy(arg).to_string();
        ctx.pending_subscribe.push(channel.clone());
        let count = ctx.pending_subscribe.len() + ctx.pending_psubscribe.len();
        confirmations.push(Frame::Array(vec![
            Frame::Bulk("subscribe".into()),
            Frame::Bulk(channel.into()),
            Frame::Integer(count as i64),
        ]));
    }

    if confirmations.len() == 1 {
        confirmations.pop().unwrap()
    } else {
        Frame::Array(confirmations)
    }
}

/// PSUBSCRIBE pattern [pattern ...] — handler for MULTI/EXEC path.
fn cmd_psubscribe(_state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let mut confirmations = Vec::new();
    for arg in args {
        let pattern = String::from_utf8_lossy(arg).to_string();
        ctx.pending_psubscribe.push(pattern.clone());
        let count = ctx.pending_subscribe.len() + ctx.pending_psubscribe.len();
        confirmations.push(Frame::Array(vec![
            Frame::Bulk("psubscribe".into()),
            Frame::Bulk(pattern.into()),
            Frame::Integer(count as i64),
        ]));
    }

    if confirmations.len() == 1 {
        confirmations.pop().unwrap()
    } else {
        Frame::Array(confirmations)
    }
}

/// UNSUBSCRIBE [channel ...] — handler for MULTI/EXEC path.
fn cmd_unsubscribe(_state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        // Unsubscribe from all pending channels
        ctx.pending_subscribe.clear();
        let count = ctx.pending_subscribe.len() + ctx.pending_psubscribe.len();
        return Frame::Array(vec![
            Frame::Bulk("unsubscribe".into()),
            Frame::Null,
            Frame::Integer(count as i64),
        ]);
    }

    let mut confirmations = Vec::new();
    for arg in args {
        let channel = String::from_utf8_lossy(arg).to_string();
        ctx.pending_subscribe.retain(|ch| *ch != channel);
        let count = ctx.pending_subscribe.len() + ctx.pending_psubscribe.len();
        confirmations.push(Frame::Array(vec![
            Frame::Bulk("unsubscribe".into()),
            Frame::Bulk(channel.into()),
            Frame::Integer(count as i64),
        ]));
    }

    if confirmations.len() == 1 {
        confirmations.pop().unwrap()
    } else {
        Frame::Array(confirmations)
    }
}

/// PUNSUBSCRIBE [pattern ...] — handler for MULTI/EXEC path.
fn cmd_punsubscribe(_state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        // Unsubscribe from all pending patterns
        ctx.pending_psubscribe.clear();
        let count = ctx.pending_subscribe.len() + ctx.pending_psubscribe.len();
        return Frame::Array(vec![
            Frame::Bulk("punsubscribe".into()),
            Frame::Null,
            Frame::Integer(count as i64),
        ]);
    }

    let mut confirmations = Vec::new();
    for arg in args {
        let pattern = String::from_utf8_lossy(arg).to_string();
        ctx.pending_psubscribe.retain(|p| *p != pattern);
        let count = ctx.pending_subscribe.len() + ctx.pending_psubscribe.len();
        confirmations.push(Frame::Array(vec![
            Frame::Bulk("punsubscribe".into()),
            Frame::Bulk(pattern.into()),
            Frame::Integer(count as i64),
        ]));
    }

    if confirmations.len() == 1 {
        confirmations.pop().unwrap()
    } else {
        Frame::Array(confirmations)
    }
}

/// PUBLISH channel message
fn cmd_publish(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let channel = String::from_utf8_lossy(&args[0]).to_string();
    let message = String::from_utf8_lossy(&args[1]).to_string();

    let registry = state.pubsub.lock().unwrap();
    let count = registry.publish(&channel, &message);
    Frame::Integer(count)
}

/// PUBSUB CHANNELS/NUMSUB/NUMPAT
fn cmd_pubsub(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "CHANNELS" => {
            if args.len() > 2 {
                return Frame::error(err_wrong_number("pubsub|channels"));
            }
            let pattern = if args.len() > 1 {
                Some(String::from_utf8_lossy(&args[1]).to_string())
            } else {
                None
            };
            let registry = state.pubsub.lock().unwrap();
            let channels = registry.active_channels(pattern.as_deref());
            Frame::Array(
                channels
                    .into_iter()
                    .map(|ch| Frame::Bulk(ch.into()))
                    .collect(),
            )
        }
        "NUMSUB" => {
            let registry = state.pubsub.lock().unwrap();
            let mut result = Vec::new();
            for arg in &args[1..] {
                let channel = String::from_utf8_lossy(arg).to_string();
                let count = registry.numsub(&channel);
                result.push(Frame::Bulk(channel.into()));
                result.push(Frame::Integer(count));
            }
            Frame::Array(result)
        }
        "NUMPAT" => {
            if args.len() > 1 {
                return Frame::error(err_wrong_number("pubsub|numpat"));
            }
            let registry = state.pubsub.lock().unwrap();
            Frame::Integer(registry.numpat())
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try PUBSUB HELP.",
            subcmd.to_lowercase()
        )),
    }
}
