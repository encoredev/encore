use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};

use mlua::prelude::*;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INVALID_INT, MSG_INVALID_KEYS_NUMBER, MSG_NEGATIVE_KEYS_NUMBER,
    MSG_NO_SCRIPT_FOUND, err_wrong_number,
};
use crate::frame::Frame;

pub fn register(table: &mut CommandTable) {
    table.add("EVAL", cmd_eval, false, -3);
    table.add("EVAL_RO", cmd_eval_ro, true, -3);
    table.add("EVALSHA", cmd_evalsha, false, -3);
    table.add("EVALSHA_RO", cmd_evalsha_ro, true, -3);
    table.add("SCRIPT", cmd_script, false, -2);
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
    format!(
        "ERR Error compiling script (new function): {}",
        sanitize_lua_error(err)
    )
}

/// Sanitize a Lua error message for RESP:
/// 1. Strip everything after the first `\n` (stack traces)
/// 2. Strip mlua "runtime error: " prefix
/// 3. Strip Lua source location prefix like `[string "..."]:N: `
fn sanitize_lua_error(err: &str) -> String {
    // Take only the first line
    let first_line = err.split('\n').next().unwrap_or(err);
    // Strip mlua "runtime error: " prefix
    let stripped = first_line
        .strip_prefix("runtime error: ")
        .unwrap_or(first_line);
    // Strip Lua source location prefix like `[string "user_script"]:N: `
    let stripped = if let Some(pos) = stripped.find("]: ") {
        let after = &stripped[pos + 3..];
        // Strip the line number prefix "N: " that may remain
        if let Some(colon_pos) = after.find(": ") {
            if after[..colon_pos].chars().all(|c| c.is_ascii_digit()) {
                &after[colon_pos + 2..]
            } else {
                after
            }
        } else {
            after
        }
    } else {
        stripped
    };
    stripped.to_string()
}

/// Commands that are not allowed inside Lua scripts.
const DISALLOWED_IN_SCRIPTS: &[&str] = &[
    "MULTI",
    "EXEC",
    "DISCARD",
    "EVAL",
    "EVAL_RO",
    "EVALSHA",
    "EVALSHA_RO",
    "SCRIPT",
    "AUTH",
    "WATCH",
    "UNWATCH",
    "SUBSCRIBE",
    "UNSUBSCRIBE",
    "PSUBSCRIBE",
    "PUNSUBSCRIBE",
];

// ── Frame <-> Lua value conversion ──────────────────────────────────

/// Convert a Frame (Redis response) to a Lua value.
fn frame_to_lua(lua: &Lua, frame: &Frame) -> LuaResult<LuaValue> {
    match frame {
        Frame::Null | Frame::NullArray => Ok(LuaValue::Boolean(false)),
        Frame::Integer(n) => Ok(LuaValue::Integer(*n)),
        Frame::Simple(s) => {
            // Status reply -> table with "ok" field
            let tbl = lua.create_table()?;
            tbl.set("ok", s.as_str())?;
            Ok(LuaValue::Table(tbl))
        }
        Frame::Bulk(b) => {
            let s = lua.create_string(b.as_ref())?;
            Ok(LuaValue::String(s))
        }
        Frame::Error(msg) => {
            // Error -> table with "err" field
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

// ── cjson helpers ───────────────────────────────────────────────────

/// Convert a Lua value to a JSON string.
fn lua_to_json_string(value: &LuaValue) -> Result<String, String> {
    match value {
        LuaValue::Nil => Ok("null".to_string()),
        LuaValue::Boolean(b) => Ok(if *b { "true" } else { "false" }.to_string()),
        LuaValue::Integer(n) => Ok(n.to_string()),
        LuaValue::Number(n) => {
            if n.fract() == 0.0 && n.abs() < i64::MAX as f64 {
                Ok(format!("{}", *n as i64))
            } else {
                Ok(n.to_string())
            }
        }
        LuaValue::String(s) => {
            let raw = String::from_utf8_lossy(&s.as_bytes()).to_string();
            Ok(json_escape_string(&raw))
        }
        LuaValue::Table(tbl) => {
            // Check if it's an array (sequential integer keys starting at 1)
            let is_array = {
                let mut has_seq = false;
                if let Ok(v) = tbl.get::<LuaValue>(1)
                    && !matches!(v, LuaValue::Nil)
                {
                    has_seq = true;
                }
                has_seq
            };

            if is_array {
                let mut items = Vec::new();
                for i in 1.. {
                    match tbl.get::<LuaValue>(i) {
                        Ok(LuaValue::Nil) => break,
                        Ok(val) => items.push(lua_to_json_string(&val)?),
                        Err(_) => break,
                    }
                }
                Ok(format!("[{}]", items.join(",")))
            } else {
                // Object - iterate pairs
                let mut pairs = Vec::new();
                for (k, v) in tbl.clone().pairs::<LuaValue, LuaValue>().flatten() {
                    let key_str = match &k {
                        LuaValue::String(s) => String::from_utf8_lossy(&s.as_bytes()).to_string(),
                        LuaValue::Integer(n) => n.to_string(),
                        LuaValue::Number(n) => n.to_string(),
                        _ => continue,
                    };
                    pairs.push(format!(
                        "{}:{}",
                        json_escape_string(&key_str),
                        lua_to_json_string(&v)?
                    ));
                }
                Ok(format!("{{{}}}", pairs.join(",")))
            }
        }
        _ => Err("Cannot encode non-supported Lua type".to_string()),
    }
}

/// JSON-escape a string.
fn json_escape_string(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for ch in s.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => {
                out.push_str(&format!("\\u{:04x}", c as u32));
            }
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

/// Parse a JSON string into a Lua value.
fn json_to_lua(lua: &Lua, json: &str) -> LuaResult<LuaValue> {
    let json = json.trim();
    if json.is_empty() {
        return Err(LuaError::RuntimeError(
            "Expected value but found EOF".to_string(),
        ));
    }

    let (val, _) = json_parse_value(lua, json, 0)?;
    Ok(val)
}

fn json_parse_value(lua: &Lua, json: &str, pos: usize) -> LuaResult<(LuaValue, usize)> {
    let pos = json_skip_whitespace(json, pos);
    if pos >= json.len() {
        return Err(LuaError::RuntimeError("Unexpected end of JSON".to_string()));
    }

    let ch = json.as_bytes()[pos];
    match ch {
        b'"' => json_parse_string(lua, json, pos),
        b'{' => json_parse_object(lua, json, pos),
        b'[' => json_parse_array(lua, json, pos),
        b't' => {
            if json[pos..].starts_with("true") {
                Ok((LuaValue::Boolean(true), pos + 4))
            } else {
                Err(LuaError::RuntimeError("Invalid JSON value".to_string()))
            }
        }
        b'f' => {
            if json[pos..].starts_with("false") {
                Ok((LuaValue::Boolean(false), pos + 5))
            } else {
                Err(LuaError::RuntimeError("Invalid JSON value".to_string()))
            }
        }
        b'n' => {
            if json[pos..].starts_with("null") {
                Ok((LuaValue::Nil, pos + 4))
            } else {
                Err(LuaError::RuntimeError("Invalid JSON value".to_string()))
            }
        }
        b'-' | b'0'..=b'9' => json_parse_number(lua, json, pos),
        _ => Err(LuaError::RuntimeError(format!(
            "Unexpected character '{}' at position {}",
            ch as char, pos
        ))),
    }
}

fn json_skip_whitespace(json: &str, mut pos: usize) -> usize {
    let bytes = json.as_bytes();
    while pos < bytes.len() && matches!(bytes[pos], b' ' | b'\t' | b'\n' | b'\r') {
        pos += 1;
    }
    pos
}

fn json_parse_string(lua: &Lua, json: &str, pos: usize) -> LuaResult<(LuaValue, usize)> {
    // pos points to opening quote
    let mut i = pos + 1;
    let bytes = json.as_bytes();
    let mut result = String::new();

    while i < bytes.len() {
        match bytes[i] {
            b'"' => {
                let s = lua.create_string(result.as_bytes())?;
                return Ok((LuaValue::String(s), i + 1));
            }
            b'\\' => {
                i += 1;
                if i >= bytes.len() {
                    return Err(LuaError::RuntimeError(
                        "Unterminated string escape".to_string(),
                    ));
                }
                match bytes[i] {
                    b'"' => result.push('"'),
                    b'\\' => result.push('\\'),
                    b'/' => result.push('/'),
                    b'n' => result.push('\n'),
                    b'r' => result.push('\r'),
                    b't' => result.push('\t'),
                    b'b' => result.push('\u{0008}'),
                    b'f' => result.push('\u{000C}'),
                    b'u' => {
                        if i + 4 >= bytes.len() {
                            return Err(LuaError::RuntimeError(
                                "Invalid unicode escape".to_string(),
                            ));
                        }
                        let hex = &json[i + 1..i + 5];
                        let code = u32::from_str_radix(hex, 16).map_err(|_| {
                            LuaError::RuntimeError("Invalid unicode escape".to_string())
                        })?;
                        if let Some(c) = char::from_u32(code) {
                            result.push(c);
                        }
                        i += 4;
                    }
                    _ => {
                        result.push('\\');
                        result.push(bytes[i] as char);
                    }
                }
            }
            _ => result.push(bytes[i] as char),
        }
        i += 1;
    }
    Err(LuaError::RuntimeError("Unterminated string".to_string()))
}

fn json_parse_number(_lua: &Lua, json: &str, pos: usize) -> LuaResult<(LuaValue, usize)> {
    let bytes = json.as_bytes();
    let mut i = pos;
    let mut is_float = false;

    if i < bytes.len() && bytes[i] == b'-' {
        i += 1;
    }
    while i < bytes.len() && bytes[i].is_ascii_digit() {
        i += 1;
    }
    if i < bytes.len() && bytes[i] == b'.' {
        is_float = true;
        i += 1;
        while i < bytes.len() && bytes[i].is_ascii_digit() {
            i += 1;
        }
    }
    if i < bytes.len() && (bytes[i] == b'e' || bytes[i] == b'E') {
        is_float = true;
        i += 1;
        if i < bytes.len() && (bytes[i] == b'+' || bytes[i] == b'-') {
            i += 1;
        }
        while i < bytes.len() && bytes[i].is_ascii_digit() {
            i += 1;
        }
    }

    let num_str = &json[pos..i];
    if is_float {
        let n: f64 = num_str
            .parse()
            .map_err(|_| LuaError::RuntimeError("Invalid number".to_string()))?;
        Ok((LuaValue::Number(n), i))
    } else {
        match num_str.parse::<i64>() {
            Ok(n) => Ok((LuaValue::Integer(n), i)),
            Err(_) => {
                let n: f64 = num_str
                    .parse()
                    .map_err(|_| LuaError::RuntimeError("Invalid number".to_string()))?;
                Ok((LuaValue::Number(n), i))
            }
        }
    }
}

fn json_parse_array(lua: &Lua, json: &str, pos: usize) -> LuaResult<(LuaValue, usize)> {
    let tbl = lua.create_table()?;
    let mut i = pos + 1; // skip '['
    let mut idx = 1;

    i = json_skip_whitespace(json, i);
    if i < json.len() && json.as_bytes()[i] == b']' {
        return Ok((LuaValue::Table(tbl), i + 1));
    }

    loop {
        let (val, new_pos) = json_parse_value(lua, json, i)?;
        tbl.set(idx, val)?;
        idx += 1;
        i = json_skip_whitespace(json, new_pos);
        if i >= json.len() {
            return Err(LuaError::RuntimeError("Unterminated array".to_string()));
        }
        if json.as_bytes()[i] == b']' {
            return Ok((LuaValue::Table(tbl), i + 1));
        }
        if json.as_bytes()[i] != b',' {
            return Err(LuaError::RuntimeError("Expected ',' in array".to_string()));
        }
        i += 1;
    }
}

fn json_parse_object(lua: &Lua, json: &str, pos: usize) -> LuaResult<(LuaValue, usize)> {
    let tbl = lua.create_table()?;
    let mut i = pos + 1; // skip '{'

    i = json_skip_whitespace(json, i);
    if i < json.len() && json.as_bytes()[i] == b'}' {
        return Ok((LuaValue::Table(tbl), i + 1));
    }

    loop {
        i = json_skip_whitespace(json, i);
        // Parse key (must be a string)
        let (key_val, new_pos) = json_parse_string(lua, json, i)?;
        i = json_skip_whitespace(json, new_pos);
        if i >= json.len() || json.as_bytes()[i] != b':' {
            return Err(LuaError::RuntimeError("Expected ':' in object".to_string()));
        }
        i += 1;
        // Parse value
        let (val, new_pos) = json_parse_value(lua, json, i)?;
        tbl.set(key_val, val)?;
        i = json_skip_whitespace(json, new_pos);
        if i >= json.len() {
            return Err(LuaError::RuntimeError("Unterminated object".to_string()));
        }
        if json.as_bytes()[i] == b'}' {
            return Ok((LuaValue::Table(tbl), i + 1));
        }
        if json.as_bytes()[i] != b',' {
            return Err(LuaError::RuntimeError("Expected ',' in object".to_string()));
        }
        i += 1;
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

    // Set up global protection metatable to catch accesses to nonexistent globals.
    // This mimics Redis's behavior of erroring on undefined global variables.
    lua.load(r#"
        -- Sandbox the Lua environment like Redis does.
        -- Remove dangerous os functions, keep only os.clock.
        if os then
            local clock = os.clock
            os = { clock = clock }
        end
        -- Remove other dangerous functions
        loadfile = nil
        dofile = nil

        local _orig_globals = {}
        for k, v in pairs(_G) do
            _orig_globals[k] = true
        end
        -- Also allow KEYS, ARGV, redis, cjson which will be set after this
        _orig_globals["KEYS"] = true
        _orig_globals["ARGV"] = true
        _orig_globals["redis"] = true
        _orig_globals["cjson"] = true
        setmetatable(_G, {
            __index = function(t, name)
                if _orig_globals[name] then
                    return rawget(t, name)
                end
                error("Script attempted to access nonexistent global variable '" .. tostring(name) .. "'")
            end,
            __newindex = function(t, name, value)
                rawset(t, name, value)
            end
        })
    "#)
    .exec()
    .map_err(|e| e.to_string())?;

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

    // Create cjson module
    let cjson_table = lua.create_table().map_err(|e| e.to_string())?;

    // cjson.decode()
    let decode_fn = lua
        .create_function(|lua_ctx, args: LuaMultiValue| {
            if args.len() != 1 {
                return Err(LuaError::RuntimeError(
                    "bad argument #1 to 'decode' (string expected, got no value)".to_string(),
                ));
            }
            let arg = args.into_iter().next().unwrap();
            let json_str = match arg {
                LuaValue::String(s) => String::from_utf8_lossy(&s.as_bytes()).to_string(),
                _ => {
                    return Err(LuaError::RuntimeError(
                        "bad argument #1 to 'decode' (string expected)".to_string(),
                    ));
                }
            };
            json_to_lua(lua_ctx, &json_str)
        })
        .map_err(|e| e.to_string())?;
    cjson_table
        .set("decode", decode_fn)
        .map_err(|e| e.to_string())?;

    // cjson.encode()
    let encode_fn = lua
        .create_function(|_, args: LuaMultiValue| {
            if args.len() != 1 {
                return Err(LuaError::RuntimeError(
                    "bad argument #1 to 'encode' (value expected)".to_string(),
                ));
            }
            let arg = args.into_iter().next().unwrap();
            lua_to_json_string(&arg).map_err(LuaError::RuntimeError)
        })
        .map_err(|e| e.to_string())?;
    cjson_table
        .set("encode", encode_fn)
        .map_err(|e| e.to_string())?;

    lua.globals()
        .set("cjson", cjson_table)
        .map_err(|e| e.to_string())?;

    // Create redis module
    let redis_table = lua.create_table().map_err(|e| e.to_string())?;

    // Use an AtomicUsize to share selected_db between closures so that SELECT inside
    // a script persists for subsequent redis.call() invocations within the same script.
    let shared_selected_db = Arc::new(AtomicUsize::new(ctx.selected_db));

    // redis.call() and redis.pcall()
    let authenticated = ctx.authenticated;
    {
        let state_call = Arc::clone(state);
        let db_cell_call = Arc::clone(&shared_selected_db);
        let sha_str = sha.to_string();
        let call_fn = lua
            .create_function(move |lua_ctx, args: LuaMultiValue| {
                redis_call_impl(
                    lua_ctx,
                    &state_call,
                    &db_cell_call,
                    authenticated,
                    &sha_str,
                    true,
                    read_only,
                    args,
                )
            })
            .map_err(|e| e.to_string())?;
        redis_table
            .set("call", call_fn)
            .map_err(|e| e.to_string())?;
    }
    {
        let state_pcall = Arc::clone(state);
        let db_cell_pcall = Arc::clone(&shared_selected_db);
        let sha_str2 = sha.to_string();
        let pcall_fn = lua
            .create_function(move |lua_ctx, args: LuaMultiValue| {
                redis_call_impl(
                    lua_ctx,
                    &state_pcall,
                    &db_cell_pcall,
                    authenticated,
                    &sha_str2,
                    false,
                    read_only,
                    args,
                )
            })
            .map_err(|e| e.to_string())?;
        redis_table
            .set("pcall", pcall_fn)
            .map_err(|e| e.to_string())?;
    }

    // redis.error_reply() - must receive exactly one string argument
    let error_reply_fn = lua
        .create_function(|lua_ctx, args: LuaMultiValue| {
            if args.len() != 1 {
                return Err(LuaError::RuntimeError(
                    "wrong number or type of arguments".to_string(),
                ));
            }
            let arg = args.into_iter().next().unwrap();
            let s = match arg {
                LuaValue::String(s) => String::from_utf8_lossy(&s.as_bytes()).to_string(),
                _ => {
                    return Err(LuaError::RuntimeError(
                        "wrong number or type of arguments".to_string(),
                    ));
                }
            };
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

    // redis.status_reply() - must receive exactly one string argument
    let status_reply_fn = lua
        .create_function(|lua_ctx, args: LuaMultiValue| {
            if args.len() != 1 {
                return Err(LuaError::RuntimeError(
                    "wrong number or type of arguments".to_string(),
                ));
            }
            let arg = args.into_iter().next().unwrap();
            let msg = match arg {
                LuaValue::String(s) => s,
                _ => {
                    return Err(LuaError::RuntimeError(
                        "wrong number or type of arguments".to_string(),
                    ));
                }
            };
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

    // redis.sha1hex() - handle nil/non-string args (treat as empty string)
    let sha1hex_fn = lua
        .create_function(|lua_ctx, args: LuaMultiValue| {
            if args.len() != 1 {
                return Err(LuaError::RuntimeError(
                    "wrong number of arguments".to_string(),
                ));
            }
            let arg = args.into_iter().next().unwrap();
            let s = match arg {
                LuaValue::String(s) => String::from_utf8_lossy(&s.as_bytes()).to_string(),
                LuaValue::Integer(n) => n.to_string(),
                LuaValue::Number(n) => n.to_string(),
                LuaValue::Nil => String::new(),
                _ => String::new(), // tables, booleans etc treated as empty string
            };
            let hash = sha1_hex(&s);
            let result = lua_ctx.create_string(hash.as_bytes())?;
            Ok(LuaValue::String(result))
        })
        .map_err(|e| e.to_string())?;
    redis_table
        .set("sha1hex", sha1hex_fn)
        .map_err(|e| e.to_string())?;

    // redis.replicate_commands() - no-op, returns true (always succeeds since Redis 7.0)
    let replicate_fn = lua
        .create_function(|_, ()| Ok(LuaValue::Boolean(true)))
        .map_err(|e| e.to_string())?;
    redis_table
        .set("replicate_commands", replicate_fn)
        .map_err(|e| e.to_string())?;

    // redis.set_repl() - no-op, accepts any value
    let set_repl_fn = lua
        .create_function(|_, _: LuaMultiValue| Ok(()))
        .map_err(|e| e.to_string())?;
    redis_table
        .set("set_repl", set_repl_fn)
        .map_err(|e| e.to_string())?;

    // redis.setresp() - validate arg (must be 2 or 3)
    let setresp_fn = lua
        .create_function(|_, version: i32| {
            if version != 2 && version != 3 {
                return Err(LuaError::RuntimeError(
                    "RESP version must be 2 or 3.".to_string(),
                ));
            }
            Ok(LuaValue::Nil)
        })
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

    // Replication constants (used with set_repl)
    redis_table.set("REPL_NONE", 0).map_err(|e| e.to_string())?;
    redis_table
        .set("REPL_SLAVE", 1)
        .map_err(|e| e.to_string())?;
    redis_table
        .set("REPL_REPLICA", 1)
        .map_err(|e| e.to_string())?;
    redis_table.set("REPL_AOF", 2).map_err(|e| e.to_string())?;
    redis_table.set("REPL_ALL", 3).map_err(|e| e.to_string())?;

    lua.globals()
        .set("redis", redis_table)
        .map_err(|e| e.to_string())?;

    // Execute the script: compile to a function, then call it.
    // This ensures that only explicit `return` statements produce return values
    // (unlike eval() which may try to prepend `return` to the code).
    let func = lua
        .load(script)
        .into_function()
        .map_err(|e| err_lua_parse_error(&e.to_string()))?;

    // Cache the script after successful compilation (Redis caches on EVAL even if
    // execution fails at runtime, but not on compilation or argument errors).
    {
        let mut inner = state.lock();
        inner.scripts.insert(sha.to_string(), script.to_string());
    }

    let result: LuaValue = match func.call(()) {
        Ok(v) => v,
        Err(e) => {
            let msg = sanitize_lua_error(&e.to_string());
            // Check if it looks like a Redis error (starts with ERR, WRONGTYPE, etc.)
            if msg.starts_with("ERR ")
                || msg.starts_with("WRONGTYPE ")
                || msg.starts_with("NOSCRIPT ")
                || msg.starts_with("NOGROUP ")
                || msg.starts_with("BUSYKEY ")
                || msg.contains("@user_script")
            {
                return Err(msg);
            }
            return Err(format!("ERR @user_script:0: {}", msg));
        }
    };

    Ok(lua_to_frame(result))
}

/// Implementation of redis.call() / redis.pcall() within Lua.
#[allow(clippy::too_many_arguments)]
fn redis_call_impl(
    lua: &Lua,
    state: &Arc<SharedState>,
    selected_db_cell: &Arc<AtomicUsize>,
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

    // Check if the command is disallowed in scripts
    if DISALLOWED_IN_SCRIPTS.contains(&cmd.as_str()) {
        let msg = msg_not_from_scripts(sha);
        if fail_fast {
            return Err(LuaError::RuntimeError(msg));
        }
        let tbl = lua.create_table()?;
        tbl.set("err", msg)?;
        return Ok(LuaValue::Table(tbl));
    }

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
    nested_ctx.selected_db = selected_db_cell.load(Ordering::Relaxed);
    nested_ctx.authenticated = authenticated;
    nested_ctx.nested = true;
    nested_ctx.nested_sha = Some(sha.to_string());

    // Check arity before executing
    if meta.arity != 0 {
        let n = cmd_args.len() as i32; // cmd_args already includes command name
        let bad = if meta.arity > 0 {
            n != meta.arity
        } else {
            n < -meta.arity
        };
        if bad {
            let msg = format!(
                "ERR wrong number of arguments for '{}' command",
                cmd.to_lowercase()
            );
            if fail_fast {
                return Err(LuaError::RuntimeError(msg));
            }
            let tbl = lua.create_table()?;
            tbl.set("err", msg)?;
            return Ok(LuaValue::Table(tbl));
        }
    }

    // Execute the command
    let frame = (meta.handler)(state, &mut nested_ctx, cmd_args_rest);

    // If this was a SELECT command, update the shared selected_db
    if cmd == "SELECT" && !matches!(&frame, Frame::Error(_)) {
        selected_db_cell.store(nested_ctx.selected_db, Ordering::Relaxed);
    }

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
    if ctx.nested {
        return Frame::error(msg_not_from_scripts(
            ctx.nested_sha.as_deref().unwrap_or(""),
        ));
    }

    let script = String::from_utf8_lossy(&args[0]).to_string();
    let sha = sha1_hex(&script);
    let remaining = &args[1..];

    match run_lua_script(state, ctx, &sha, &script, read_only, remaining) {
        Ok(frame) => frame,
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
