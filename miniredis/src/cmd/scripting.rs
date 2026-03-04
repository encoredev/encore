use std::sync::Arc;

use mlua::prelude::*;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INVALID_INT, MSG_INVALID_KEYS_NUMBER, MSG_NEGATIVE_KEYS_NUMBER,
    MSG_NO_SCRIPT_FOUND, err_wrong_number,
};
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("EVAL", cmd_eval, false);
    table.add("EVAL_RO", cmd_eval_ro, true);
    table.add("EVALSHA", cmd_evalsha, false);
    table.add("EVALSHA_RO", cmd_evalsha_ro, true);
    table.add("SCRIPT", cmd_script, false);
}

fn sha1_hex(s: &str) -> String {
    use sha1_smol::Sha1;
    let mut hasher = Sha1::new();
    hasher.update(s.as_bytes());
    let digest = hasher.digest();
    // Convert digest bytes to hex string
    digest
        .bytes()
        .iter()
        .map(|b| format!("{:02x}", b))
        .collect()
}

fn msg_not_from_scripts(sha: &str) -> String {
    format!(
        "This Redis command is not allowed from script script: {}, &c",
        sha
    )
}

fn err_lua_parse_error(err: &str) -> String {
    format!("ERR Error compiling script (new function): {}", err)
}

// ── Frame <-> Lua value conversion ──────────────────────────────────

/// Convert a Frame (Redis response) to a Lua value.
fn frame_to_lua(lua: &Lua, frame: &Frame) -> LuaResult<LuaValue> {
    match frame {
        Frame::Null => Ok(LuaValue::Boolean(false)),
        Frame::Integer(n) => Ok(LuaValue::Integer(*n)),
        Frame::Simple(s) => {
            // Status reply → table with "ok" field
            let tbl = lua.create_table()?;
            tbl.set("ok", s.as_str())?;
            Ok(LuaValue::Table(tbl))
        }
        Frame::Bulk(b) => {
            let s = lua.create_string(b.as_ref())?;
            Ok(LuaValue::String(s))
        }
        Frame::Error(msg) => {
            // Error → table with "err" field
            let tbl = lua.create_table()?;
            tbl.set("err", msg.as_str())?;
            Ok(LuaValue::Table(tbl))
        }
        Frame::Array(arr) | Frame::Set(arr) | Frame::Push(arr) => {
            let tbl = lua.create_table()?;
            for (i, item) in arr.iter().enumerate() {
                let val = frame_to_lua(lua, item)?;
                tbl.set(i + 1, val)?;
            }
            Ok(LuaValue::Table(tbl))
        }
        Frame::Map(pairs) => {
            let tbl = lua.create_table()?;
            for (i, (k, v)) in pairs.iter().enumerate() {
                let kv_tbl = lua.create_table()?;
                kv_tbl.set(1, frame_to_lua(lua, k)?)?;
                kv_tbl.set(2, frame_to_lua(lua, v)?)?;
                tbl.set(i + 1, kv_tbl)?;
            }
            Ok(LuaValue::Table(tbl))
        }
        Frame::Double(f) => Ok(LuaValue::Number(*f)),
    }
}

/// Convert a Lua value to a Frame (Redis response).
fn lua_to_frame(value: LuaValue) -> Frame {
    match value {
        LuaValue::Nil => Frame::Null,
        LuaValue::Boolean(b) => {
            if b {
                Frame::Integer(1)
            } else {
                Frame::Null
            }
        }
        LuaValue::Integer(n) => Frame::Integer(n),
        LuaValue::Number(n) => Frame::Integer(n as i64),
        LuaValue::String(s) => Frame::Bulk(s.as_bytes().to_vec().into()),
        LuaValue::Table(tbl) => {
            // Check for special "err" field
            if let Ok(err_val) = tbl.get::<LuaValue>("err")
                && let LuaValue::String(s) = err_val
            {
                let msg = String::from_utf8_lossy(&s.as_bytes()).to_string();
                return Frame::Error(msg);
            }
            // Check for special "ok" field
            if let Ok(ok_val) = tbl.get::<LuaValue>("ok")
                && let LuaValue::String(s) = ok_val
            {
                let msg = String::from_utf8_lossy(&s.as_bytes()).to_string();
                return Frame::Simple(msg);
            }
            // Numeric array
            let mut result = Vec::new();
            for i in 1.. {
                match tbl.get::<LuaValue>(i) {
                    Ok(LuaValue::Nil) => break,
                    Ok(val) => result.push(lua_to_frame(val)),
                    Err(_) => break,
                }
            }
            Frame::Array(result)
        }
        _ => Frame::Null,
    }
}

// ── Lua script execution ────────────────────────────────────────────

/// Run a Lua script. Returns the response Frame.
fn run_lua_script(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    sha: &str,
    script: &str,
    read_only: bool,
    args: &[Vec<u8>],
) -> Result<Frame, String> {
    // Parse numkeys and split args into KEYS/ARGV
    if args.is_empty() {
        return Err(MSG_INVALID_INT.to_string());
    }
    let numkeys_str = String::from_utf8_lossy(&args[0]);
    let numkeys: i64 = numkeys_str
        .parse()
        .map_err(|_| MSG_INVALID_INT.to_string())?;
    if numkeys < 0 {
        return Err(MSG_NEGATIVE_KEYS_NUMBER.to_string());
    }
    let numkeys = numkeys as usize;
    let remaining = &args[1..];
    if numkeys > remaining.len() {
        return Err(MSG_INVALID_KEYS_NUMBER.to_string());
    }

    let keys = &remaining[..numkeys];
    let argv = &remaining[numkeys..];

    // Create Lua state
    let lua = Lua::new();

    // Set KEYS global
    let keys_table = lua.create_table().map_err(|e| e.to_string())?;
    for (i, k) in keys.iter().enumerate() {
        let s = lua.create_string(k.as_slice()).map_err(|e| e.to_string())?;
        keys_table.set(i + 1, s).map_err(|e| e.to_string())?;
    }
    lua.globals()
        .set("KEYS", keys_table)
        .map_err(|e| e.to_string())?;

    // Set ARGV global
    let argv_table = lua.create_table().map_err(|e| e.to_string())?;
    for (i, a) in argv.iter().enumerate() {
        let s = lua.create_string(a.as_slice()).map_err(|e| e.to_string())?;
        argv_table.set(i + 1, s).map_err(|e| e.to_string())?;
    }
    lua.globals()
        .set("ARGV", argv_table)
        .map_err(|e| e.to_string())?;

    // Create redis module
    let redis_table = lua.create_table().map_err(|e| e.to_string())?;

    // redis.call() and redis.pcall()
    let state_call = Arc::clone(state);
    let selected_db = ctx.selected_db;
    let authenticated = ctx.authenticated;
    let sha_str = sha.to_string();

    let call_fn = lua
        .create_function(move |lua_ctx, args: LuaMultiValue| {
            redis_call_impl(
                lua_ctx,
                &state_call,
                selected_db,
                authenticated,
                &sha_str,
                true,
                false,
                args,
            )
        })
        .map_err(|e| e.to_string())?;
    redis_table
        .set("call", call_fn)
        .map_err(|e| e.to_string())?;

    let state_pcall = Arc::clone(state);
    let sha_str2 = sha.to_string();
    let pcall_fn = lua
        .create_function(move |lua_ctx, args: LuaMultiValue| {
            redis_call_impl(
                lua_ctx,
                &state_pcall,
                selected_db,
                authenticated,
                &sha_str2,
                false,
                false,
                args,
            )
        })
        .map_err(|e| e.to_string())?;
    redis_table
        .set("pcall", pcall_fn)
        .map_err(|e| e.to_string())?;

    // Read-only variants for redis.call in read-only scripts
    if read_only {
        // Wrap call/pcall to check read-only
        let state_ro_call = Arc::clone(state);
        let sha_ro = sha.to_string();
        let ro_call_fn = lua
            .create_function(move |lua_ctx, args: LuaMultiValue| {
                redis_call_impl(
                    lua_ctx,
                    &state_ro_call,
                    selected_db,
                    authenticated,
                    &sha_ro,
                    true,
                    true,
                    args,
                )
            })
            .map_err(|e| e.to_string())?;
        redis_table
            .set("call", ro_call_fn)
            .map_err(|e| e.to_string())?;

        let state_ro_pcall = Arc::clone(state);
        let sha_ro2 = sha.to_string();
        let ro_pcall_fn = lua
            .create_function(move |lua_ctx, args: LuaMultiValue| {
                redis_call_impl(
                    lua_ctx,
                    &state_ro_pcall,
                    selected_db,
                    authenticated,
                    &sha_ro2,
                    false,
                    true,
                    args,
                )
            })
            .map_err(|e| e.to_string())?;
        redis_table
            .set("pcall", ro_pcall_fn)
            .map_err(|e| e.to_string())?;
    }

    // redis.error_reply()
    let error_reply_fn = lua
        .create_function(|lua_ctx, msg: LuaString| {
            let s = String::from_utf8_lossy(&msg.as_bytes()).to_string();
            let parts: Vec<&str> = s.splitn(2, ' ').collect();
            let final_msg = if parts.len() == 2 {
                let prefix = parts[0].strip_prefix('-').unwrap_or(parts[0]);
                format!("{} {}", prefix, parts[1])
            } else {
                let prefix = parts[0].strip_prefix('-').unwrap_or(parts[0]);
                format!("ERR {}", prefix)
            };
            let tbl = lua_ctx.create_table()?;
            tbl.set("err", final_msg)?;
            Ok(LuaValue::Table(tbl))
        })
        .map_err(|e| e.to_string())?;
    redis_table
        .set("error_reply", error_reply_fn)
        .map_err(|e| e.to_string())?;

    // redis.status_reply()
    let status_reply_fn = lua
        .create_function(|lua_ctx, msg: LuaString| {
            let tbl = lua_ctx.create_table()?;
            tbl.set("ok", msg)?;
            Ok(LuaValue::Table(tbl))
        })
        .map_err(|e| e.to_string())?;
    redis_table
        .set("status_reply", status_reply_fn)
        .map_err(|e| e.to_string())?;

    // redis.log() - no-op
    let log_fn = lua
        .create_function(|_, (_level, _msg): (i32, LuaString)| Ok(()))
        .map_err(|e| e.to_string())?;
    redis_table.set("log", log_fn).map_err(|e| e.to_string())?;

    // redis.sha1hex()
    let sha1hex_fn = lua
        .create_function(|lua_ctx, msg: LuaString| {
            let s = String::from_utf8_lossy(&msg.as_bytes()).to_string();
            let hash = sha1_hex(&s);
            let result = lua_ctx.create_string(hash.as_bytes())?;
            Ok(LuaValue::String(result))
        })
        .map_err(|e| e.to_string())?;
    redis_table
        .set("sha1hex", sha1hex_fn)
        .map_err(|e| e.to_string())?;

    // redis.replicate_commands() - always true
    let replicate_fn = lua
        .create_function(|_, ()| Ok(true))
        .map_err(|e| e.to_string())?;
    redis_table
        .set("replicate_commands", replicate_fn)
        .map_err(|e| e.to_string())?;

    // redis.set_repl() - no-op
    let set_repl_fn = lua
        .create_function(|_, _: i32| Ok(()))
        .map_err(|e| e.to_string())?;
    redis_table
        .set("set_repl", set_repl_fn)
        .map_err(|e| e.to_string())?;

    // redis.setresp() - no-op in our implementation
    let setresp_fn = lua
        .create_function(|_, _: i32| Ok(()))
        .map_err(|e| e.to_string())?;
    redis_table
        .set("setresp", setresp_fn)
        .map_err(|e| e.to_string())?;

    // Redis constants
    redis_table.set("LOG_DEBUG", 0).map_err(|e| e.to_string())?;
    redis_table
        .set("LOG_VERBOSE", 1)
        .map_err(|e| e.to_string())?;
    redis_table
        .set("LOG_NOTICE", 2)
        .map_err(|e| e.to_string())?;
    redis_table
        .set("LOG_WARNING", 3)
        .map_err(|e| e.to_string())?;

    lua.globals()
        .set("redis", redis_table)
        .map_err(|e| e.to_string())?;

    // Execute the script
    let result: LuaValue = lua
        .load(script)
        .eval()
        .map_err(|e| err_lua_parse_error(&e.to_string()))?;

    Ok(lua_to_frame(result))
}

/// Implementation of redis.call() / redis.pcall() within Lua.
#[allow(clippy::too_many_arguments)]
fn redis_call_impl(
    lua: &Lua,
    state: &Arc<SharedState>,
    selected_db: usize,
    authenticated: bool,
    sha: &str,
    fail_fast: bool,
    read_only: bool,
    args: LuaMultiValue,
) -> LuaResult<LuaValue> {
    if args.is_empty() {
        return Err(LuaError::RuntimeError(format!(
            "Please specify at least one argument for this redis lib call script: {}, &c.",
            sha
        )));
    }

    // Convert Lua args to string args
    let mut cmd_args: Vec<Vec<u8>> = Vec::new();
    for arg in args {
        match arg {
            LuaValue::String(s) => cmd_args.push(s.as_bytes().to_vec()),
            LuaValue::Integer(n) => cmd_args.push(n.to_string().into_bytes()),
            LuaValue::Number(n) => cmd_args.push((n as i64).to_string().into_bytes()),
            _ => {
                return Err(LuaError::RuntimeError(format!(
                    "Lua redis lib command arguments must be strings or integers script: {}, &c.",
                    sha
                )));
            }
        }
    }

    if cmd_args.is_empty() {
        return Err(LuaError::RuntimeError(msg_not_from_scripts(sha)));
    }

    let cmd = String::from_utf8_lossy(&cmd_args[0]).to_uppercase();
    let cmd_args_rest = &cmd_args[1..];

    // Get the command table
    let table = state
        .command_table
        .get()
        .expect("command table not initialized");

    // Look up the command
    let meta = match table.get(&cmd) {
        Some(m) => m,
        None => {
            let msg = if fail_fast {
                format!(
                    "Unknown Redis command called from script script: {}, &c.",
                    sha
                )
            } else {
                "ERR Unknown Redis command called from script".to_string()
            };
            if fail_fast {
                return Err(LuaError::RuntimeError(msg));
            }
            let tbl = lua.create_table()?;
            tbl.set("err", msg)?;
            return Ok(LuaValue::Table(tbl));
        }
    };

    // Check read-only mode
    if read_only && !meta.read_only {
        let msg = "Write commands are not allowed in read-only scripts";
        if fail_fast {
            return Err(LuaError::RuntimeError(msg.to_string()));
        }
        let tbl = lua.create_table()?;
        tbl.set("err", msg)?;
        return Ok(LuaValue::Table(tbl));
    }

    // Create a nested ConnCtx for this call
    let mut nested_ctx = ConnCtx::new();
    nested_ctx.selected_db = selected_db;
    nested_ctx.authenticated = authenticated;
    nested_ctx.nested = true;
    nested_ctx.nested_sha = Some(sha.to_string());

    // Execute the command
    let frame = (meta.handler)(state, &mut nested_ctx, cmd_args_rest);

    // Convert result to Lua
    match &frame {
        Frame::Error(msg) => {
            if fail_fast {
                return Err(LuaError::RuntimeError(msg.clone()));
            }
            let tbl = lua.create_table()?;
            tbl.set("err", msg.as_str())?;
            Ok(LuaValue::Table(tbl))
        }
        _ => frame_to_lua(lua, &frame),
    }
}

// ── Command handlers ────────────────────────────────────────────────

fn cmd_eval(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_eval_shared(state, ctx, args, false)
}

fn cmd_eval_ro(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_eval_shared(state, ctx, args, true)
}

fn cmd_eval_shared(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    read_only: bool,
) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("eval"));
    }

    if ctx.nested {
        return Frame::error(msg_not_from_scripts(
            ctx.nested_sha.as_deref().unwrap_or(""),
        ));
    }

    let script = String::from_utf8_lossy(&args[0]).to_string();
    let sha = sha1_hex(&script);
    let remaining = &args[1..];

    match run_lua_script(state, ctx, &sha, &script, read_only, remaining) {
        Ok(frame) => {
            // Cache script on success
            let mut inner = state.lock();
            inner.scripts.insert(sha, script);
            frame
        }
        Err(msg) => Frame::error(msg),
    }
}

fn cmd_evalsha(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_evalsha_shared(state, ctx, args, false)
}

fn cmd_evalsha_ro(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_evalsha_shared(state, ctx, args, true)
}

fn cmd_evalsha_shared(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    read_only: bool,
) -> Frame {
    if args.len() < 2 {
        return Frame::error(err_wrong_number("evalsha"));
    }

    if ctx.nested {
        return Frame::error(msg_not_from_scripts(
            ctx.nested_sha.as_deref().unwrap_or(""),
        ));
    }

    let sha = String::from_utf8_lossy(&args[0]).to_string();
    let remaining = &args[1..];

    // Look up the script
    let script = {
        let inner = state.lock();
        inner.scripts.get(&sha).cloned()
    };

    match script {
        Some(script) => match run_lua_script(state, ctx, &sha, &script, read_only, remaining) {
            Ok(frame) => frame,
            Err(msg) => Frame::error(msg),
        },
        None => Frame::error(MSG_NO_SCRIPT_FOUND),
    }
}

fn cmd_script(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    if args.is_empty() {
        return Frame::error(err_wrong_number("script"));
    }

    if ctx.nested {
        return Frame::error(msg_not_from_scripts(
            ctx.nested_sha.as_deref().unwrap_or(""),
        ));
    }

    let subcmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    let sub_args = &args[1..];

    match subcmd.as_str() {
        "LOAD" => {
            if sub_args.len() != 1 {
                return Frame::error(format!(
                    "ERR unknown subcommand or wrong number of arguments for '{}'. Try SCRIPT HELP.",
                    "LOAD"
                ));
            }
            let script = String::from_utf8_lossy(&sub_args[0]).to_string();

            // Validate syntax by attempting to load in a Lua state
            let lua = Lua::new();
            if let Err(e) = lua.load(&script).into_function() {
                return Frame::error(err_lua_parse_error(&e.to_string()));
            }

            let sha = sha1_hex(&script);
            let mut inner = state.lock();
            inner.scripts.insert(sha.clone(), script);
            Frame::Bulk(sha.into())
        }
        "EXISTS" => {
            if sub_args.is_empty() {
                return Frame::error(err_wrong_number("script|exists"));
            }
            let inner = state.lock();
            let mut results = Vec::with_capacity(sub_args.len());
            for arg in sub_args {
                let sha = String::from_utf8_lossy(arg);
                if inner.scripts.contains_key(sha.as_ref()) {
                    results.push(Frame::Integer(1));
                } else {
                    results.push(Frame::Integer(0));
                }
            }
            Frame::Array(results)
        }
        "FLUSH" => {
            // Accept optional SYNC/ASYNC arg
            if sub_args.len() > 1 {
                return Frame::error("ERR SCRIPT FLUSH only support SYNC|ASYNC option");
            }
            if sub_args.len() == 1 {
                let opt = String::from_utf8_lossy(&sub_args[0]).to_uppercase();
                if opt != "SYNC" && opt != "ASYNC" {
                    return Frame::error("ERR SCRIPT FLUSH only support SYNC|ASYNC option");
                }
            }
            let mut inner = state.lock();
            inner.scripts.clear();
            Frame::ok()
        }
        _ => Frame::error(format!(
            "ERR unknown subcommand '{}'. Try SCRIPT HELP.",
            subcmd
        )),
    }
}
