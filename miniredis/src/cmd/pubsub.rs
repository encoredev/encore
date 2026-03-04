use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{CommandTable, err_wrong_number};
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("PUBLISH", cmd_publish, false);
    table.add("PUBSUB", cmd_pubsub, true);
}

/// PUBLISH channel message
fn cmd_publish(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 2 {
        return Frame::error(err_wrong_number("publish"));
    }

    let channel = String::from_utf8_lossy(&args[0]).to_string();
    let message = String::from_utf8_lossy(&args[1]).to_string();

    let registry = state.pubsub.lock().unwrap();
    let count = registry.publish(&channel, &message);
    Frame::Integer(count)
}

/// PUBSUB CHANNELS/NUMSUB/NUMPAT
fn cmd_pubsub(state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("pubsub"));
    }

    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "CHANNELS" => {
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
            let registry = state.pubsub.lock().unwrap();
            Frame::Integer(registry.numpat())
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try PUBSUB HELP.",
            subcmd.to_lowercase()
        )),
    }
}
