use std::collections::HashMap;
use std::fmt;
use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::frame::Frame;

// ── Error message constants ──────────────────────────────────────────

pub const MSG_WRONG_TYPE: &str =
    "WRONGTYPE Operation against a key holding the wrong kind of value";
pub const MSG_INVALID_INT: &str = "ERR value is not an integer or out of range";
pub const MSG_INT_OVERFLOW: &str = "ERR increment or decrement would overflow";
pub const MSG_INVALID_FLOAT: &str = "ERR value is not a valid float";
pub const MSG_INVALID_MIN_MAX: &str = "ERR min or max is not a float";
pub const MSG_INVALID_RANGE_ITEM: &str = "ERR min or max not valid string range item";
pub const MSG_INVALID_TIMEOUT: &str = "ERR timeout is not a float or out of range";
pub const MSG_SYNTAX_ERROR: &str = "ERR syntax error";
pub const MSG_KEY_NOT_FOUND: &str = "ERR no such key";
pub const MSG_OUT_OF_RANGE: &str = "ERR index out of range";
pub const MSG_INVALID_CURSOR: &str = "ERR invalid cursor";
pub const MSG_XX_AND_NX: &str = "ERR XX and NX options at the same time are not compatible";
pub const MSG_TIMEOUT_NEGATIVE: &str = "ERR timeout is negative";
pub const MSG_INVALID_SE_TIME: &str = "ERR invalid expire time in set";
pub const MSG_INVALID_SETEX_TIME: &str = "ERR invalid expire time in setex";
pub const MSG_INVALID_PSETEX_TIME: &str = "ERR invalid expire time in psetex";
pub const MSG_INVALID_KEYS_NUMBER: &str = "ERR Number of keys can't be greater than number of args";
pub const MSG_NEGATIVE_KEYS_NUMBER: &str = "ERR Number of keys can't be negative";
pub const MSG_NO_SCRIPT_FOUND: &str = "NOSCRIPT No matching script. Please use EVAL.";
pub const MSG_DB_INDEX_OUT_OF_RANGE: &str = "ERR DB index is out of range";
pub const MSG_SINGLE_ELEMENT_PAIR: &str =
    "ERR INCR option supports a single increment-element pair";
pub const MSG_NOT_VALID_HLL_VALUE: &str = "WRONGTYPE Key is not a valid HyperLogLog string value.";
pub const MSG_INVALID_RANGE: &str = "ERR value is out of range, must be positive";
pub const MSG_TIMEOUT_IS_OUT_OF_RANGE: &str = "ERR timeout is out of range";
pub const MSG_GT_LT_AND_NX: &str =
    "ERR GT, LT, and/or NX options at the same time are not compatible";
pub const MSG_INVALID_STREAM_ID: &str =
    "ERR Invalid stream ID specified as stream command argument";
pub const MSG_STREAM_ID_TOO_SMALL: &str =
    "ERR The ID specified in XADD is equal or smaller than the target stream top item";
pub const MSG_STREAM_ID_ZERO: &str = "ERR The ID specified in XADD must be greater than 0-0";
pub const MSG_UNSUPPORTED_UNIT: &str = "ERR unsupported unit provided. please use M, KM, FT, MI";
pub const MSG_XGROUP_KEY_NOT_FOUND: &str = "ERR The XGROUP subcommand requires the key to exist. Note that for CREATE you may want to use the MKSTREAM option to create an empty stream automatically.";
pub const MSG_LIMIT_COMBINATION: &str =
    "ERR syntax error, LIMIT is only supported in combination with either BYSCORE or BYLEX";
pub const MSG_GT_AND_LT: &str = "ERR GT and LT options at the same time are not compatible";
pub const MSG_NX_AND_XX_GT_LT: &str =
    "ERR NX and XX, GT or LT options at the same time are not compatible";
pub const MSG_NUM_FIELDS_INVALID: &str = "ERR Parameter `numFields` should be greater than 0";
pub const MSG_NUM_FIELDS_PARAMETER: &str =
    "ERR The `numfields` parameter must match the number of arguments";

/// Generate the "wrong number of arguments" error for a command.
pub fn err_wrong_number(cmd: &str) -> String {
    format!(
        "ERR wrong number of arguments for '{}' command",
        cmd.to_lowercase()
    )
}

/// Generate the "unknown command" error.
pub fn err_unknown_command(cmd: &str, args: &[Vec<u8>]) -> String {
    let mut s = format!("ERR unknown command `{}`, with args beginning with: ", cmd);
    for (i, a) in args.iter().enumerate() {
        if i >= 20 {
            break;
        }
        let a_str = String::from_utf8_lossy(a);
        s.push_str(&format!("`{}`, ", a_str));
    }
    s
}

// ── Command handler types ────────────────────────────────────────────

/// The signature for a command handler function.
/// Takes shared state, per-connection context, and the raw arguments.
/// Returns a Frame to send as the response.
pub type CommandHandler =
    fn(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame;

/// Metadata about a registered command.
pub struct CommandMeta {
    pub handler: CommandHandler,
    pub read_only: bool,
    /// Redis arity. Positive = exact arg count (including cmd name).
    /// Negative = minimum arg count. Zero = skip check.
    pub arity: i32,
}

/// The command dispatch table.
pub struct CommandTable {
    commands: HashMap<&'static str, CommandMeta>,
}

impl Default for CommandTable {
    fn default() -> Self {
        Self::new()
    }
}

impl CommandTable {
    pub fn new() -> Self {
        let mut table = CommandTable {
            commands: HashMap::new(),
        };

        // Register all implemented commands.
        crate::cmd::connection::register(&mut table);
        crate::cmd::string::register(&mut table);
        crate::cmd::generic::register(&mut table);
        crate::cmd::server::register(&mut table);
        crate::cmd::hash::register(&mut table);
        crate::cmd::list::register(&mut table);
        crate::cmd::set::register(&mut table);
        crate::cmd::sorted_set::register(&mut table);
        crate::cmd::transactions::register(&mut table);
        crate::cmd::hll::register(&mut table);
        crate::cmd::geo::register(&mut table);
        crate::cmd::pubsub::register(&mut table);
        crate::cmd::client::register(&mut table);
        crate::cmd::cluster::register(&mut table);
        crate::cmd::object::register(&mut table);
        crate::cmd::stream::register(&mut table);
        crate::cmd::scripting::register(&mut table);

        table
    }

    /// Register a command with arity.
    /// Arity follows Redis convention: positive = exact arg count (incl. cmd name),
    /// negative = minimum arg count, zero = skip check.
    pub fn add(
        &mut self,
        name: &'static str,
        handler: CommandHandler,
        read_only: bool,
        arity: i32,
    ) {
        self.commands.insert(
            name,
            CommandMeta {
                handler,
                read_only,
                arity,
            },
        );
    }

    /// Look up a command by name (uppercase).
    pub fn get(&self, name: &str) -> Option<&CommandMeta> {
        self.commands.get(name)
    }
}

impl fmt::Debug for CommandTable {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("CommandTable")
            .field("commands", &self.commands.keys().collect::<Vec<_>>())
            .finish()
    }
}

// ── Dispatch logic ───────────────────────────────────────────────────

/// Dispatch a single command. Returns the response frame, and whether the
/// connection should be closed (QUIT).
pub fn dispatch(
    table: &CommandTable,
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
) -> (Frame, bool) {
    if args.is_empty() {
        return (Frame::error("ERR empty command"), false);
    }

    let cmd = String::from_utf8_lossy(&args[0]).to_uppercase();
    let cmd_args = &args[1..];

    // Handle MULTI/EXEC/DISCARD specially — they're not queued.
    // Note: the MULTI path only runs when authenticated (you can't enter MULTI
    // without being authenticated), so no auth check is needed here.
    if ctx.in_tx() && cmd != "EXEC" && cmd != "DISCARD" && cmd != "MULTI" && cmd != "WATCH" {
        // Validate the command exists before queueing
        let meta = match table.get(&cmd) {
            Some(m) => m,
            None => {
                ctx.dirty_transaction = true;
                return (Frame::error(err_unknown_command(&cmd, cmd_args)), false);
            }
        };

        // Validate arity before queueing (Redis checks this before QUEUED)
        if meta.arity != 0 {
            let n = args.len() as i32;
            let bad = if meta.arity > 0 {
                n != meta.arity
            } else {
                n < -meta.arity
            };
            if bad {
                ctx.dirty_transaction = true;
                return (Frame::error(err_wrong_number(&cmd.to_lowercase())), false);
            }
        }

        // Special case: validate SCRIPT subcommands before queueing.
        // Real Redis (and Go miniredis) rejects unknown subcommands immediately.
        if cmd == "SCRIPT" && !cmd_args.is_empty() {
            let subcmd = String::from_utf8_lossy(&cmd_args[0]).to_uppercase();
            if !["LOAD", "EXISTS", "FLUSH"].contains(&subcmd.as_str()) {
                ctx.dirty_transaction = true;
                return (
                    Frame::error(format!(
                        "ERR unknown subcommand '{}'. Try SCRIPT HELP.",
                        subcmd
                    )),
                    false,
                );
            }
        }

        // Special case: validate OBJECT subcommand arity before queueing.
        if cmd == "OBJECT" && !cmd_args.is_empty() {
            let subcmd = String::from_utf8_lossy(&cmd_args[0]).to_uppercase();
            match subcmd.as_str() {
                "ENCODING" | "IDLETIME" | "REFCOUNT" | "FREQ" => {
                    // These subcommands require exactly 1 additional arg (the key)
                    if cmd_args.len() != 2 {
                        ctx.dirty_transaction = true;
                        return (
                            Frame::error(err_wrong_number(&format!(
                                "object|{}",
                                subcmd.to_lowercase()
                            ))),
                            false,
                        );
                    }
                }
                "HELP" => {}
                _ => {
                    ctx.dirty_transaction = true;
                    return (
                        Frame::error(format!(
                            "ERR unknown subcommand or wrong number of arguments for 'object|{}' command",
                            subcmd.to_lowercase()
                        )),
                        false,
                    );
                }
            }
        }

        // Queue the command
        if let Some(ref mut tx) = ctx.transaction {
            tx.push(crate::connection::QueuedCommand {
                args: args.to_vec(),
            });
        }
        return (Frame::Simple("QUEUED".into()), false);
    }

    // Handle EXEC specially — it needs the command table to replay queued commands.
    if cmd == "EXEC" {
        return (cmd_exec(table, state, ctx, cmd_args), false);
    }

    // Look up the command
    let meta = match table.get(&cmd) {
        Some(m) => m,
        None => {
            // Unknown commands: check auth before returning unknown error
            if !ctx.authenticated {
                let inner = state.lock();
                if !inner.passwords.is_empty() {
                    return (Frame::error("NOAUTH Authentication required."), false);
                }
            }
            return (Frame::error(err_unknown_command(&cmd, cmd_args)), false);
        }
    };

    // Validate arity before auth (Redis checks arity first)
    if meta.arity != 0 {
        let n = args.len() as i32;
        let bad = if meta.arity > 0 {
            n != meta.arity
        } else {
            n < -meta.arity
        };
        if bad {
            return (Frame::error(err_wrong_number(&cmd.to_lowercase())), false);
        }
    }

    // Check auth (after arity validation)
    if !ctx.authenticated {
        let inner = state.lock();
        if !inner.passwords.is_empty() && cmd != "AUTH" && cmd != "HELLO" && cmd != "QUIT" {
            return (Frame::error("NOAUTH Authentication required."), false);
        }
    }

    // Execute the command under the lock
    let response = with_lock(state, ctx, meta.handler, cmd_args);
    let should_close = cmd == "QUIT";

    (response, should_close)
}

/// Execute a command handler under the database lock.
/// This is the normal (non-MULTI) path: lock → execute → notify → unlock.
fn with_lock(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    handler: CommandHandler,
    args: &[Vec<u8>],
) -> Frame {
    let response = handler(state, ctx, args);

    // Notify any blocking commands that data may have changed.
    state.notify.notify_waiters();

    response
}

/// EXEC — execute a queued transaction.
/// Handled in dispatch because it needs access to the command table.
fn cmd_exec(
    table: &CommandTable,
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
) -> Frame {
    if !args.is_empty() {
        return Frame::error(err_wrong_number("exec"));
    }
    if !ctx.in_tx() {
        return Frame::error("ERR EXEC without MULTI");
    }

    // Dirty transaction (e.g. unknown command was queued) — abort.
    if ctx.dirty_transaction {
        ctx.transaction = None;
        ctx.watch.clear();
        return Frame::error("EXECABORT Transaction discarded because of previous errors.");
    }

    // Check WATCHed keys.
    {
        let inner = state.lock();
        for ((db_idx, key), version) in &ctx.watch {
            let current = inner.db(*db_idx).key_version.get(key).copied().unwrap_or(0);
            if current > *version {
                // WATCH detected a change — abort.
                ctx.transaction = None;
                ctx.watch.clear();
                return Frame::NullArray;
            }
        }
    }

    // Take the queued commands and clear transaction state.
    let commands = ctx.transaction.take().unwrap_or_default();
    ctx.watch.clear();

    // Execute each queued command and collect results.
    let mut results = Vec::with_capacity(commands.len());
    for queued in &commands {
        let cmd_name = String::from_utf8_lossy(&queued.args[0]).to_uppercase();
        let cmd_args = &queued.args[1..];

        let meta = match table.get(&cmd_name) {
            Some(m) => m,
            None => {
                results.push(Frame::error(err_unknown_command(&cmd_name, cmd_args)));
                continue;
            }
        };

        let result = (meta.handler)(state, ctx, cmd_args);
        results.push(result);
    }

    // Notify any blocking commands that data may have changed.
    state.notify.notify_waiters();

    Frame::Array(results)
}
