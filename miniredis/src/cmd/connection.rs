use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_DB_INDEX_OUT_OF_RANGE, MSG_SYNTAX_ERROR, err_wrong_number,
};
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("PING", cmd_ping, true);
    table.add("ECHO", cmd_echo, true);
    table.add("QUIT", cmd_quit, true);
    table.add("SELECT", cmd_select, true);
    table.add("AUTH", cmd_auth, false);
    table.add("HELLO", cmd_hello, false);
}

/// PING [message]
fn cmd_ping(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    match args.len() {
        0 => Frame::Simple("PONG".into()),
        1 => Frame::Bulk(args[0].clone().into()),
        _ => Frame::error(err_wrong_number("ping")),
    }
}

/// ECHO message
fn cmd_echo(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("echo"));
    }
    Frame::Bulk(args[0].clone().into())
}

/// QUIT
fn cmd_quit(_state: &Arc<SharedState>, _ctx: &mut ConnCtx, _args: &[Vec<u8>]) -> Frame {
    Frame::ok()
}

/// SELECT db
fn cmd_select(_state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.len() != 1 {
        return Frame::error(err_wrong_number("select"));
    }

    let db_str = match std::str::from_utf8(&args[0]) {
        Ok(s) => s,
        Err(_) => return Frame::error(MSG_DB_INDEX_OUT_OF_RANGE),
    };

    let db: usize = match db_str.parse() {
        Ok(n) => n,
        Err(_) => return Frame::error(MSG_DB_INDEX_OUT_OF_RANGE),
    };

    if db > 15 {
        return Frame::error(MSG_DB_INDEX_OUT_OF_RANGE);
    }

    ctx.selected_db = db;
    Frame::ok()
}

/// AUTH [username] password
fn cmd_auth(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() || args.len() > 2 {
        if args.is_empty() {
            return Frame::error(err_wrong_number("auth"));
        }
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let (username, password) = if args.len() == 2 {
        (
            String::from_utf8_lossy(&args[0]).to_string(),
            String::from_utf8_lossy(&args[1]).to_string(),
        )
    } else {
        (
            "default".to_string(),
            String::from_utf8_lossy(&args[0]).to_string(),
        )
    };

    let inner = state.lock();

    if inner.passwords.is_empty() && username == "default" {
        return Frame::error(
            "ERR AUTH <password> called without any password configured for the default user. Are you sure your configuration is correct?",
        );
    }

    match inner.passwords.get(&username) {
        Some(pw) if pw == &password => {
            ctx.authenticated = true;
            Frame::ok()
        }
        _ => Frame::error("WRONGPASS invalid username-password pair"),
    }
}

/// HELLO protover [AUTH username password] [SETNAME clientname]
fn cmd_hello(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("hello"));
    }

    // Parse protocol version
    let version: i64 = match std::str::from_utf8(&args[0])
        .ok()
        .and_then(|s| s.parse().ok())
    {
        Some(v) => v,
        None => {
            return Frame::error("ERR Protocol version is not an integer or out of range");
        }
    };

    if version != 2 && version != 3 {
        return Frame::error("NOPROTO unsupported protocol version");
    }

    // Parse optional AUTH and SETNAME
    let mut check_auth = false;
    let mut username = "default".to_string();
    let mut password = String::new();
    let mut i = 1;

    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "AUTH" => {
                if i + 2 >= args.len() {
                    return Frame::error(format!(
                        "ERR Syntax error in HELLO option '{}'",
                        String::from_utf8_lossy(&args[i])
                    ));
                }
                username = String::from_utf8_lossy(&args[i + 1]).to_string();
                password = String::from_utf8_lossy(&args[i + 2]).to_string();
                check_auth = true;
                i += 3;
            }
            "SETNAME" => {
                if i + 1 >= args.len() {
                    return Frame::error(format!(
                        "ERR Syntax error in HELLO option '{}'",
                        String::from_utf8_lossy(&args[i])
                    ));
                }
                ctx.client_name = Some(String::from_utf8_lossy(&args[i + 1]).to_string());
                i += 2;
            }
            _ => {
                return Frame::error(format!(
                    "ERR Syntax error in HELLO option '{}'",
                    String::from_utf8_lossy(&args[i])
                ));
            }
        }
    }

    // Check authentication if AUTH was provided
    let inner = state.lock();
    if inner.passwords.is_empty() && username == "default" {
        check_auth = false;
    }
    if check_auth {
        match inner.passwords.get(&username) {
            Some(pw) if pw == &password => {
                ctx.authenticated = true;
            }
            _ => {
                return Frame::error("WRONGPASS invalid username-password pair");
            }
        }
    }

    // Set RESP3 mode if version is 3
    ctx.resp3 = version == 3;

    // Return server info as a map
    Frame::Map(vec![
        (
            Frame::bulk_string("server"),
            Frame::bulk_string("miniredis"),
        ),
        (Frame::bulk_string("version"), Frame::bulk_string("8.4.0")),
        (Frame::bulk_string("proto"), Frame::Integer(version)),
        (Frame::bulk_string("id"), Frame::Integer(42)),
        (Frame::bulk_string("mode"), Frame::bulk_string("standalone")),
        (Frame::bulk_string("role"), Frame::bulk_string("master")),
        (Frame::bulk_string("modules"), Frame::Array(vec![])),
    ])
}
