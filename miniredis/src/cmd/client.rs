use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::CommandTable;
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("CLIENT", cmd_client, false, -2);
}

fn cmd_client(_state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    match subcmd.as_str() {
        "SETNAME" => {
            if args.len() != 2 {
                return Frame::error("ERR wrong number of arguments for 'client|setname' command");
            }
            let name = String::from_utf8_lossy(&args[1]).to_string();
            if name.contains(' ') || name.contains('\n') {
                return Frame::error(
                    "ERR Client names cannot contain spaces, newlines or special characters.",
                );
            }
            ctx.client_name = if name.is_empty() { None } else { Some(name) };
            Frame::ok()
        }
        "GETNAME" => {
            if args.len() != 1 {
                return Frame::error("ERR wrong number of arguments for 'client|getname' command");
            }
            match &ctx.client_name {
                Some(name) => Frame::Bulk(name.clone().into()),
                None => Frame::Null,
            }
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try CLIENT HELP.",
            subcmd.to_lowercase()
        )),
    }
}
