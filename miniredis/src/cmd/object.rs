use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::CommandTable;
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("OBJECT", cmd_object, true);
}

fn cmd_object(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error("ERR wrong number of arguments for 'object' command");
    }

    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "ENCODING" => {
            if args.len() != 2 {
                return Frame::error("ERR wrong number of arguments for 'object|encoding' command");
            }
            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            match db.keys.get(key.as_ref()) {
                None => Frame::Null,
                Some(kt) => {
                    let encoding = match kt {
                        crate::types::KeyType::String => "raw",
                        crate::types::KeyType::Hash => "hashtable",
                        crate::types::KeyType::List => "linkedlist",
                        crate::types::KeyType::Set => "hashtable",
                        crate::types::KeyType::SortedSet => "skiplist",
                        crate::types::KeyType::Stream => "stream",
                        crate::types::KeyType::HyperLogLog => "raw",
                    };
                    Frame::Bulk(encoding.into())
                }
            }
        }
        "REFCOUNT" => {
            if args.len() != 2 {
                return Frame::error("ERR wrong number of arguments for 'object|refcount' command");
            }
            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            if !db.keys.contains_key(key.as_ref()) {
                return Frame::Null;
            }
            // Always return 1 (simplified)
            Frame::Integer(1)
        }
        "FREQ" => {
            if args.len() != 2 {
                return Frame::error("ERR wrong number of arguments for 'object|freq' command");
            }
            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            if !db.keys.contains_key(key.as_ref()) {
                return Frame::Null;
            }
            // Always return 0 (simplified)
            Frame::Integer(0)
        }
        "IDLETIME" => {
            if args.len() != 2 {
                return Frame::error("ERR wrong number of arguments for 'object|idletime' command");
            }
            let key = String::from_utf8_lossy(&args[1]);
            let inner = state.lock();
            let db = inner.db(ctx.selected_db);

            if !db.keys.contains_key(key.as_ref()) {
                return Frame::Null;
            }

            match db.lru.get(key.as_ref()) {
                Some(last_access) => {
                    let now = inner.effective_now();
                    let idle = now.duration_since(*last_access).unwrap_or_default();
                    Frame::Integer(idle.as_secs() as i64)
                }
                None => Frame::Integer(0),
            }
        }
        "HELP" => Frame::Array(vec![Frame::Bulk(
            "OBJECT IDLETIME <key> - return idle time of key".into(),
        )]),
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try OBJECT HELP.",
            subcmd.to_lowercase()
        )),
    }
}
