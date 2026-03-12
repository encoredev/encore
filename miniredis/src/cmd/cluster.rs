use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::CommandTable;
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("CLUSTER", cmd_cluster, true, -2);
}

fn cmd_cluster(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "SLOTS" => {
            // Single-node cluster: one slot range 0-16383
            Frame::Array(vec![Frame::Array(vec![
                Frame::Integer(0),
                Frame::Integer(16383),
                Frame::Array(vec![
                    Frame::Bulk("127.0.0.1".into()),
                    Frame::Integer(6379),
                    Frame::Bulk(
                        "09dbe9720cda62f7865eabc5fd8857c5d2678366".into(),
                    ),
                ]),
            ])])
        }
        "KEYSLOT" => {
            if args.len() != 2 {
                return Frame::error(
                    "ERR wrong number of arguments for 'cluster|keyslot' command",
                );
            }
            // Simplified: always return 163
            Frame::Integer(163)
        }
        "NODES" => {
            Frame::Bulk(
                "e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca 127.0.0.1:6379@6379 myself,master - 0 0 1 connected 0-16383\n"
                    .into(),
            )
        }
        "SHARDS" => {
            // Simplified shard info as a flat array
            Frame::Array(vec![Frame::Array(vec![
                Frame::Bulk("slots".into()),
                Frame::Array(vec![Frame::Integer(0), Frame::Integer(16383)]),
                Frame::Bulk("nodes".into()),
                Frame::Array(vec![Frame::Array(vec![
                    Frame::Bulk("id".into()),
                    Frame::Bulk(
                        "13f84e686106847b76671957dd348fde540a77bb".into(),
                    ),
                    Frame::Bulk("ip".into()),
                    Frame::Bulk("127.0.0.1".into()),
                    Frame::Bulk("port".into()),
                    Frame::Integer(6379),
                    Frame::Bulk("role".into()),
                    Frame::Bulk("master".into()),
                    Frame::Bulk("replication-offset".into()),
                    Frame::Integer(0),
                    Frame::Bulk("health".into()),
                    Frame::Bulk("online".into()),
                ])]),
            ])])
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try CLUSTER HELP.",
            subcmd.to_lowercase()
        )),
    }
}
