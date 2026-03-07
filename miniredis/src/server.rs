use std::sync::Arc;

use tokio::net::TcpListener;
use tokio::sync::broadcast;

use crate::connection::{ConnCtx, Connection};
use crate::db::SharedState;
use crate::dispatch::{CommandTable, dispatch, err_wrong_number};
use crate::frame::Frame;
use crate::pubsub::PubsubCtx;

/// Start the server: bind to the given address, accept connections, and
/// dispatch commands.
///
/// Returns the bound address (useful when binding to port 0).
/// The server runs until a shutdown signal is received via `shutdown_rx`.
/// If `tls_acceptor` is Some, connections are wrapped with TLS.
pub async fn run(
    listener: TcpListener,
    state: Arc<SharedState>,
    mut shutdown_rx: broadcast::Receiver<()>,
    #[cfg(feature = "tls")] tls_acceptor: Option<tokio_rustls::TlsAcceptor>,
    #[cfg(not(feature = "tls"))] _tls_acceptor: Option<()>,
) {
    let table = Arc::new(CommandTable::new());
    let _ = state.command_table.set(Arc::clone(&table));

    loop {
        tokio::select! {
            result = listener.accept() => {
                let (socket, _addr) = match result {
                    Ok(s) => s,
                    Err(_) => continue,
                };

                let state = Arc::clone(&state);
                let table = Arc::clone(&table);
                let shutdown_rx = state.shutdown_tx.subscribe();

                #[cfg(feature = "tls")]
                let tls = tls_acceptor.clone();

                tokio::spawn(async move {
                    #[cfg(feature = "tls")]
                    if let Some(acceptor) = tls {
                        if let Ok(tls_stream) = acceptor.accept(socket).await {
                            handle_connection_stream(
                                Connection::new_stream(tls_stream),
                                state,
                                table,
                                shutdown_rx,
                            ).await;
                            return;
                        }
                        return;
                    }

                    handle_connection_stream(
                        Connection::new(socket),
                        state,
                        table,
                        shutdown_rx,
                    ).await;
                });
            }
            _ = shutdown_rx.recv() => {
                // Shutdown signal received
                return;
            }
        }
    }
}

/// Handle a single client connection (plain or TLS).
async fn handle_connection_stream(
    mut conn: Connection,
    state: Arc<SharedState>,
    table: Arc<CommandTable>,
    mut shutdown_rx: broadcast::Receiver<()>,
) {
    use std::sync::atomic::Ordering;

    state
        .total_connections_received
        .fetch_add(1, Ordering::Relaxed);
    state.connected_clients.fetch_add(1, Ordering::Relaxed);

    let mut ctx = ConnCtx::new();
    let mut pubsub: Option<PubsubCtx> = None;

    handle_connection_inner(
        &mut conn,
        &mut ctx,
        &mut pubsub,
        &state,
        &table,
        &mut shutdown_rx,
    )
    .await;

    // Cleanup: remove subscriber from registry if in pub/sub mode
    if let Some(ps) = pubsub.take() {
        let mut registry = state.pubsub.lock().unwrap();
        registry.remove(&ps.handle);
    }

    state.connected_clients.fetch_sub(1, Ordering::Relaxed);
}

async fn handle_connection_inner(
    conn: &mut Connection,
    ctx: &mut ConnCtx,
    pubsub: &mut Option<PubsubCtx>,
    state: &Arc<SharedState>,
    table: &Arc<CommandTable>,
    shutdown_rx: &mut broadcast::Receiver<()>,
) {
    loop {
        if let Some(ps) = pubsub.as_mut() {
            // ── Pub/Sub mode event loop ────────────────────────────
            tokio::select! {
                result = conn.read_frame() => {
                    let frame = match result {
                        Ok(Some(frame)) => frame,
                        Ok(None) => return,
                        Err(_) => return,
                    };

                    let args = match frame_to_args(frame) {
                        Some(args) => args,
                        None => {
                            let _ = conn.write_frame(&Frame::error("ERR invalid command format")).await;
                            continue;
                        }
                    };

                    if args.is_empty() {
                        let _ = conn.write_frame(&Frame::error("ERR empty command")).await;
                        continue;
                    }

                    let cmd = String::from_utf8_lossy(&args[0]).to_uppercase();
                    let cmd_args = &args[1..];

                    match cmd.as_str() {
                        "SUBSCRIBE" => {
                            if cmd_args.is_empty() {
                                let _ = conn.write_frame(&Frame::error(err_wrong_number("subscribe"))).await;
                                continue;
                            }
                            for arg in cmd_args {
                                let channel = String::from_utf8_lossy(arg).to_string();
                                let count = ps.subscribe(&channel);
                                let confirm = pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("subscribe".into()),
                                    Frame::Bulk(channel.into()),
                                    Frame::Integer(count as i64),
                                ]);
                                if conn.write_frame(&confirm).await.is_err() {
                                    return;
                                }
                            }
                        }
                        "UNSUBSCRIBE" => {
                            if cmd_args.is_empty() {
                                // Unsubscribe from all channels
                                let channels = ps.channels();
                                if channels.is_empty() {
                                    let confirm = pubsub_msg(ctx.resp3, vec![
                                        Frame::Bulk("unsubscribe".into()),
                                        Frame::Null,
                                        Frame::Integer(ps.total_count() as i64),
                                    ]);
                                    if conn.write_frame(&confirm).await.is_err() {
                                        return;
                                    }
                                } else {
                                    for channel in channels {
                                        let count = ps.unsubscribe(&channel);
                                        let confirm = pubsub_msg(ctx.resp3, vec![
                                            Frame::Bulk("unsubscribe".into()),
                                            Frame::Bulk(channel.into()),
                                            Frame::Integer(count as i64),
                                        ]);
                                        if conn.write_frame(&confirm).await.is_err() {
                                            return;
                                        }
                                    }
                                }
                            } else {
                                for arg in cmd_args {
                                    let channel = String::from_utf8_lossy(arg).to_string();
                                    let count = ps.unsubscribe(&channel);
                                    let confirm = pubsub_msg(ctx.resp3, vec![
                                        Frame::Bulk("unsubscribe".into()),
                                        Frame::Bulk(channel.into()),
                                        Frame::Integer(count as i64),
                                    ]);
                                    if conn.write_frame(&confirm).await.is_err() {
                                        return;
                                    }
                                }
                            }
                            // Exit pub/sub mode if no subscriptions left
                            if ps.total_count() == 0 {
                                let handle = &ps.handle;
                                let mut registry = state.pubsub.lock().unwrap();
                                registry.remove(handle);
                                drop(registry);
                                *pubsub = None;
                            }
                        }
                        "PSUBSCRIBE" => {
                            if cmd_args.is_empty() {
                                let _ = conn.write_frame(&Frame::error(err_wrong_number("psubscribe"))).await;
                                continue;
                            }
                            for arg in cmd_args {
                                let pattern = String::from_utf8_lossy(arg).to_string();
                                let count = ps.psubscribe(&pattern);
                                let confirm = pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("psubscribe".into()),
                                    Frame::Bulk(pattern.into()),
                                    Frame::Integer(count as i64),
                                ]);
                                if conn.write_frame(&confirm).await.is_err() {
                                    return;
                                }
                            }
                        }
                        "PUNSUBSCRIBE" => {
                            if cmd_args.is_empty() {
                                let patterns = ps.patterns();
                                if patterns.is_empty() {
                                    let confirm = pubsub_msg(ctx.resp3, vec![
                                        Frame::Bulk("punsubscribe".into()),
                                        Frame::Null,
                                        Frame::Integer(ps.total_count() as i64),
                                    ]);
                                    if conn.write_frame(&confirm).await.is_err() {
                                        return;
                                    }
                                } else {
                                    for pattern in patterns {
                                        let count = ps.punsubscribe(&pattern);
                                        let confirm = pubsub_msg(ctx.resp3, vec![
                                            Frame::Bulk("punsubscribe".into()),
                                            Frame::Bulk(pattern.into()),
                                            Frame::Integer(count as i64),
                                        ]);
                                        if conn.write_frame(&confirm).await.is_err() {
                                            return;
                                        }
                                    }
                                }
                            } else {
                                for arg in cmd_args {
                                    let pattern = String::from_utf8_lossy(arg).to_string();
                                    let count = ps.punsubscribe(&pattern);
                                    let confirm = pubsub_msg(ctx.resp3, vec![
                                        Frame::Bulk("punsubscribe".into()),
                                        Frame::Bulk(pattern.into()),
                                        Frame::Integer(count as i64),
                                    ]);
                                    if conn.write_frame(&confirm).await.is_err() {
                                        return;
                                    }
                                }
                            }
                            // Exit pub/sub mode if no subscriptions left
                            if ps.total_count() == 0 {
                                let handle = &ps.handle;
                                let mut registry = state.pubsub.lock().unwrap();
                                registry.remove(handle);
                                drop(registry);
                                *pubsub = None;
                            }
                        }
                        "PING" => {
                            if cmd_args.is_empty() {
                                let pong = pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("pong".into()),
                                    Frame::Bulk("".into()),
                                ]);
                                if conn.write_frame(&pong).await.is_err() {
                                    return;
                                }
                            } else {
                                let msg = String::from_utf8_lossy(&cmd_args[0]).to_string();
                                let pong = pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("pong".into()),
                                    Frame::Bulk(msg.into()),
                                ]);
                                if conn.write_frame(&pong).await.is_err() {
                                    return;
                                }
                            }
                        }
                        "QUIT" => {
                            let _ = conn.write_frame(&Frame::ok()).await;
                            return;
                        }
                        _ => {
                            let err = format!(
                                "ERR Can't execute '{}': only (P)SUBSCRIBE / (P)UNSUBSCRIBE / PING / QUIT are allowed in this context",
                                cmd.to_lowercase()
                            );
                            if conn.write_frame(&Frame::error(err)).await.is_err() {
                                return;
                            }
                        }
                    }
                }
                msg = ps.rx.recv() => {
                    match msg {
                        Some(m) => {
                            let frame = match m.kind {
                                "message" => pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("message".into()),
                                    Frame::Bulk(m.channel.into()),
                                    Frame::Bulk(m.data.into()),
                                ]),
                                "pmessage" => pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("pmessage".into()),
                                    Frame::Bulk(m.pattern.unwrap_or_default().into()),
                                    Frame::Bulk(m.channel.into()),
                                    Frame::Bulk(m.data.into()),
                                ]),
                                _ => continue,
                            };
                            if conn.write_frame(&frame).await.is_err() {
                                return;
                            }
                        }
                        None => return, // channel closed
                    }
                }
                _ = shutdown_rx.recv() => {
                    return;
                }
            }
        } else {
            // ── Normal command loop ────────────────────────────────
            tokio::select! {
                result = conn.read_frame() => {
                    let frame = match result {
                        Ok(Some(frame)) => frame,
                        Ok(None) => return,
                        Err(_) => return,
                    };

                    let args = match frame_to_args(frame) {
                        Some(args) => args,
                        None => {
                            let _ = conn.write_frame(&Frame::error("ERR invalid command format")).await;
                            continue;
                        }
                    };

                    if args.is_empty() {
                        let _ = conn.write_frame(&Frame::error("ERR empty command")).await;
                        continue;
                    }

                    let cmd = String::from_utf8_lossy(&args[0]).to_uppercase();

                    // Handle SUBSCRIBE/PSUBSCRIBE — enter pub/sub mode
                    // (but not inside MULTI — let dispatch queue it)
                    if (cmd == "SUBSCRIBE" || cmd == "PSUBSCRIBE") && !ctx.in_tx() {
                        let cmd_args = &args[1..];
                        if cmd_args.is_empty() {
                            let _ = conn.write_frame(&Frame::error(err_wrong_number(&cmd.to_lowercase()))).await;
                            continue;
                        }

                        // Create pub/sub context
                        let ps = {
                            let mut registry = state.pubsub.lock().unwrap();
                            PubsubCtx::new(&mut registry)
                        };

                        *pubsub = Some(ps);
                        let ps = pubsub.as_mut().unwrap();

                        if cmd == "SUBSCRIBE" {
                            for arg in cmd_args {
                                let channel = String::from_utf8_lossy(arg).to_string();
                                let count = ps.subscribe(&channel);
                                let confirm = pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("subscribe".into()),
                                    Frame::Bulk(channel.into()),
                                    Frame::Integer(count as i64),
                                ]);
                                if conn.write_frame(&confirm).await.is_err() {
                                    return;
                                }
                            }
                        } else {
                            for arg in cmd_args {
                                let pattern = String::from_utf8_lossy(arg).to_string();
                                let count = ps.psubscribe(&pattern);
                                let confirm = pubsub_msg(ctx.resp3, vec![
                                    Frame::Bulk("psubscribe".into()),
                                    Frame::Bulk(pattern.into()),
                                    Frame::Integer(count as i64),
                                ]);
                                if conn.write_frame(&confirm).await.is_err() {
                                    return;
                                }
                            }
                        }
                        continue;
                    }

                    // Handle UNSUBSCRIBE/PUNSUBSCRIBE outside pub/sub mode (no-op)
                    // But not inside MULTI — let dispatch queue it.
                    if cmd == "UNSUBSCRIBE" && !ctx.in_tx() {
                        let confirm = pubsub_msg(ctx.resp3, vec![
                            Frame::Bulk("unsubscribe".into()),
                            Frame::Null,
                            Frame::Integer(0),
                        ]);
                        let _ = conn.write_frame(&confirm).await;
                        continue;
                    }
                    if cmd == "PUNSUBSCRIBE" && !ctx.in_tx() {
                        let confirm = pubsub_msg(ctx.resp3, vec![
                            Frame::Bulk("punsubscribe".into()),
                            Frame::Null,
                            Frame::Integer(0),
                        ]);
                        let _ = conn.write_frame(&confirm).await;
                        continue;
                    }

                    state.total_commands_processed.fetch_add(1, std::sync::atomic::Ordering::Relaxed);

                    // Intercept blocking commands (outside MULTI/EXEC)
                    if !ctx.in_tx() && matches!(cmd.as_str(), "BLPOP" | "BRPOP" | "BRPOPLPUSH" | "BLMOVE") {
                        let response = handle_blocking_command(
                            &cmd, &args[1..], state, ctx, shutdown_rx
                        ).await;

                        conn.resp3 = ctx.resp3;
                        if conn.write_frame(&response).await.is_err() {
                            return;
                        }
                        continue;
                    }

                    // Intercept XREAD/XREADGROUP with BLOCK (outside MULTI/EXEC)
                    if !ctx.in_tx() && matches!(cmd.as_str(), "XREAD" | "XREADGROUP")
                        && has_block_arg(&args[1..])
                    {
                        let response = handle_blocking_stream_command(
                            &cmd, &args[1..], state, ctx, shutdown_rx
                        ).await;

                        conn.resp3 = ctx.resp3;
                        if conn.write_frame(&response).await.is_err() {
                            return;
                        }
                        continue;
                    }

                    let (response, should_close) = dispatch(table, state, ctx, &args);

                    // Sync RESP3 flag (set by HELLO command)
                    conn.resp3 = ctx.resp3;

                    if conn.write_frame(&response).await.is_err() {
                        return;
                    }

                    if should_close {
                        return;
                    }

                    // Check if SUBSCRIBE/PSUBSCRIBE was executed inside EXEC.
                    // If so, enter pub/sub mode with the pending channels/patterns.
                    if !ctx.pending_subscribe.is_empty() || !ctx.pending_psubscribe.is_empty() {
                        let channels = std::mem::take(&mut ctx.pending_subscribe);
                        let patterns = std::mem::take(&mut ctx.pending_psubscribe);

                        let ps = {
                            let mut registry = state.pubsub.lock().unwrap();
                            PubsubCtx::new(&mut registry)
                        };
                        *pubsub = Some(ps);
                        let ps = pubsub.as_mut().unwrap();

                        for channel in channels {
                            ps.subscribe(&channel);
                        }
                        for pattern in patterns {
                            ps.psubscribe(&pattern);
                        }
                    }
                }
                _ = shutdown_rx.recv() => {
                    return;
                }
            }
        }
    }
}

/// Wrap pub/sub confirmation/message frames: use Push in RESP3, Array in RESP2.
fn pubsub_msg(resp3: bool, elements: Vec<Frame>) -> Frame {
    if resp3 {
        Frame::Push(elements)
    } else {
        Frame::Array(elements)
    }
}

/// Handle blocking list commands (BLPOP, BRPOP, BRPOPLPUSH, BLMOVE).
/// These block until data is available or timeout expires.
async fn handle_blocking_command(
    cmd: &str,
    args: &[Vec<u8>],
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    shutdown_rx: &mut broadcast::Receiver<()>,
) -> Frame {
    use crate::dispatch::{
        MSG_INVALID_TIMEOUT, MSG_SYNTAX_ERROR, MSG_TIMEOUT_IS_OUT_OF_RANGE, MSG_TIMEOUT_NEGATIVE,
        MSG_WRONG_TYPE,
    };
    use crate::types::KeyType;

    match cmd {
        "BLPOP" | "BRPOP" => {
            if args.len() < 2 {
                return Frame::error(crate::dispatch::err_wrong_number(&cmd.to_lowercase()));
            }
            let keys = &args[..args.len() - 1];
            let timeout_str = String::from_utf8_lossy(&args[args.len() - 1]);
            let timeout_lower = timeout_str.to_lowercase();
            if timeout_lower == "inf" || timeout_lower == "+inf" || timeout_lower == "-inf" {
                return Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE);
            }
            let timeout_s: f64 = match timeout_str.parse() {
                Ok(t) => t,
                Err(_) => return Frame::error(MSG_INVALID_TIMEOUT),
            };
            if timeout_s < 0.0 {
                return Frame::error(MSG_TIMEOUT_NEGATIVE);
            }
            let is_left = cmd == "BLPOP";

            // Try immediate pop
            {
                let mut inner = state.lock();
                let now = inner.effective_now();
                for key_bytes in keys {
                    let key = String::from_utf8_lossy(key_bytes).into_owned();
                    let db = inner.db_mut(ctx.selected_db);
                    db.check_ttl(&key);
                    if let Some(t) = db.key_type(&key)
                        && t != KeyType::List
                    {
                        return Frame::error(MSG_WRONG_TYPE);
                    }
                    let val = if is_left {
                        db.list_lpop(&key, now)
                    } else {
                        db.list_rpop(&key, now)
                    };
                    if let Some(v) = val {
                        state.notify.notify_waiters();
                        return Frame::Array(vec![Frame::Bulk(key.into()), Frame::Bulk(v.into())]);
                    }
                }
            }

            // Block until data or timeout
            let timeout_dur = if timeout_s == 0.0 {
                std::time::Duration::from_secs(300) // max wait
            } else {
                std::time::Duration::from_secs_f64(timeout_s)
            };
            let deadline = tokio::time::Instant::now() + timeout_dur;

            loop {
                tokio::select! {
                    _ = state.notify.notified() => {
                        let mut inner = state.lock();
                        let now = inner.effective_now();
                        for key_bytes in keys {
                            let key = String::from_utf8_lossy(key_bytes).into_owned();
                            let db = inner.db_mut(ctx.selected_db);
                            db.check_ttl(&key);
                            let val = if is_left {
                                db.list_lpop(&key, now)
                            } else {
                                db.list_rpop(&key, now)
                            };
                            if let Some(v) = val {
                                state.notify.notify_waiters();
                                return Frame::Array(vec![
                                    Frame::Bulk(key.into()),
                                    Frame::Bulk(v.into()),
                                ]);
                            }
                        }
                    }
                    _ = tokio::time::sleep_until(deadline) => {
                        return Frame::NullArray;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::NullArray;
                    }
                }
            }
        }
        "BRPOPLPUSH" => {
            if args.len() != 3 {
                return Frame::error(crate::dispatch::err_wrong_number("brpoplpush"));
            }
            let src = String::from_utf8_lossy(&args[0]).into_owned();
            let dst = String::from_utf8_lossy(&args[1]).into_owned();
            let timeout_str = String::from_utf8_lossy(&args[2]);
            let timeout_lower = timeout_str.to_lowercase();
            if timeout_lower == "inf" || timeout_lower == "+inf" || timeout_lower == "-inf" {
                return Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE);
            }
            let timeout_s: f64 = match timeout_str.parse() {
                Ok(t) => t,
                Err(_) => return Frame::error(MSG_INVALID_TIMEOUT),
            };
            if timeout_s < 0.0 {
                return Frame::error(MSG_TIMEOUT_NEGATIVE);
            }

            // Try immediate
            {
                let mut inner = state.lock();
                let now = inner.effective_now();
                let db = inner.db_mut(ctx.selected_db);
                db.check_ttl(&src);
                db.check_ttl(&dst);
                if let Some(t) = db.key_type(&src)
                    && t != KeyType::List
                {
                    return Frame::error(MSG_WRONG_TYPE);
                }
                if let Some(t) = db.key_type(&dst)
                    && t != KeyType::List
                {
                    return Frame::error(MSG_WRONG_TYPE);
                }
                if let Some(val) = db.list_rpop(&src, now) {
                    db.list_lpush(&dst, std::slice::from_ref(&val), now);
                    state.notify.notify_waiters();
                    return Frame::Bulk(val.into());
                }
            }

            let timeout_dur = if timeout_s == 0.0 {
                std::time::Duration::from_secs(300)
            } else {
                std::time::Duration::from_secs_f64(timeout_s)
            };
            let deadline = tokio::time::Instant::now() + timeout_dur;

            loop {
                tokio::select! {
                    _ = state.notify.notified() => {
                        let mut inner = state.lock();
                        let now = inner.effective_now();
                        let db = inner.db_mut(ctx.selected_db);
                        db.check_ttl(&src);
                        if let Some(val) = db.list_rpop(&src, now) {
                            db.list_lpush(&dst, std::slice::from_ref(&val), now);
                            state.notify.notify_waiters();
                            return Frame::Bulk(val.into());
                        }
                    }
                    _ = tokio::time::sleep_until(deadline) => {
                        return Frame::NullArray;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::NullArray;
                    }
                }
            }
        }
        "BLMOVE" => {
            if args.len() != 5 {
                return Frame::error(crate::dispatch::err_wrong_number("blmove"));
            }
            let src = String::from_utf8_lossy(&args[0]).into_owned();
            let dst = String::from_utf8_lossy(&args[1]).into_owned();
            let src_dir = String::from_utf8_lossy(&args[2]).to_uppercase();
            let dst_dir = String::from_utf8_lossy(&args[3]).to_uppercase();

            let pop_left = match src_dir.as_str() {
                "LEFT" => true,
                "RIGHT" => false,
                _ => return Frame::error(MSG_SYNTAX_ERROR),
            };
            let push_left = match dst_dir.as_str() {
                "LEFT" => true,
                "RIGHT" => false,
                _ => return Frame::error(MSG_SYNTAX_ERROR),
            };

            let timeout_str = String::from_utf8_lossy(&args[4]);
            let timeout_lower = timeout_str.to_lowercase();
            if timeout_lower == "inf" || timeout_lower == "+inf" || timeout_lower == "-inf" {
                return Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE);
            }
            let timeout_s: f64 = match timeout_str.parse() {
                Ok(t) => t,
                Err(_) => return Frame::error(MSG_INVALID_TIMEOUT),
            };
            if timeout_s < 0.0 {
                return Frame::error(MSG_TIMEOUT_NEGATIVE);
            }

            // Try immediate
            {
                let mut inner = state.lock();
                let now = inner.effective_now();
                let db = inner.db_mut(ctx.selected_db);
                db.check_ttl(&src);
                db.check_ttl(&dst);
                if let Some(t) = db.key_type(&src)
                    && t != KeyType::List
                {
                    return Frame::error(MSG_WRONG_TYPE);
                }
                if let Some(t) = db.key_type(&dst)
                    && t != KeyType::List
                {
                    return Frame::error(MSG_WRONG_TYPE);
                }

                // Save TTL when src == dst
                let saved_ttl = if src == dst {
                    db.ttl.get(&src).cloned()
                } else {
                    None
                };
                let val = if pop_left {
                    db.list_lpop(&src, now)
                } else {
                    db.list_rpop(&src, now)
                };
                if let Some(v) = val {
                    if push_left {
                        db.list_lpush(&dst, std::slice::from_ref(&v), now);
                    } else {
                        db.list_rpush(&dst, std::slice::from_ref(&v), now);
                    }
                    if let Some(ttl) = saved_ttl {
                        db.ttl.insert(dst.clone(), ttl);
                    }
                    state.notify.notify_waiters();
                    return Frame::Bulk(v.into());
                }
            }

            let timeout_dur = if timeout_s == 0.0 {
                std::time::Duration::from_secs(300)
            } else {
                std::time::Duration::from_secs_f64(timeout_s)
            };
            let deadline = tokio::time::Instant::now() + timeout_dur;

            loop {
                tokio::select! {
                    _ = state.notify.notified() => {
                        let mut inner = state.lock();
                        let now = inner.effective_now();
                        let db = inner.db_mut(ctx.selected_db);
                        db.check_ttl(&src);
                        // Save TTL when src == dst
                        let saved_ttl = if src == dst {
                            db.ttl.get(&src).cloned()
                        } else {
                            None
                        };
                        let val = if pop_left {
                            db.list_lpop(&src, now)
                        } else {
                            db.list_rpop(&src, now)
                        };
                        if let Some(v) = val {
                            if push_left {
                                db.list_lpush(&dst, std::slice::from_ref(&v), now);
                            } else {
                                db.list_rpush(&dst, std::slice::from_ref(&v), now);
                            }
                            if let Some(ttl) = saved_ttl {
                                db.ttl.insert(dst.clone(), ttl);
                            }
                            state.notify.notify_waiters();
                            return Frame::Bulk(v.into());
                        }
                    }
                    _ = tokio::time::sleep_until(deadline) => {
                        return Frame::NullArray;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::NullArray;
                    }
                }
            }
        }
        _ => Frame::error("ERR unsupported blocking command"),
    }
}

/// Check if BLOCK argument is present in the command args.
fn has_block_arg(args: &[Vec<u8>]) -> bool {
    args.iter()
        .any(|a| String::from_utf8_lossy(a).to_uppercase() == "BLOCK")
}

/// Handle blocking XREAD/XREADGROUP commands.
async fn handle_blocking_stream_command(
    cmd: &str,
    args: &[Vec<u8>],
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    shutdown_rx: &mut broadcast::Receiver<()>,
) -> Frame {
    use crate::dispatch::MSG_WRONG_TYPE;
    use crate::types::{KeyType, Stream};

    match cmd {
        "XREAD" => {
            if args.len() < 3 {
                return Frame::error(crate::dispatch::err_wrong_number("xread"));
            }

            let mut i = 0;
            let mut count: Option<usize> = None;
            let mut block_ms: Option<i64> = None;

            while i < args.len() {
                let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
                match opt.as_str() {
                    "COUNT" => {
                        i += 1;
                        if i >= args.len() {
                            return Frame::error("ERR syntax error");
                        }
                        match String::from_utf8_lossy(&args[i]).parse::<usize>() {
                            Ok(n) => count = Some(n),
                            Err(_) => {
                                return Frame::error("ERR value is not an integer or out of range");
                            }
                        }
                        i += 1;
                    }
                    "BLOCK" => {
                        i += 1;
                        if i >= args.len() {
                            return Frame::error("ERR syntax error");
                        }
                        match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                            Ok(n) if n < 0 => {
                                return Frame::error("ERR timeout is negative");
                            }
                            Ok(n) => block_ms = Some(n),
                            Err(_) => {
                                return Frame::error(
                                    "ERR timeout is not an integer or out of range",
                                );
                            }
                        }
                        i += 1;
                    }
                    "STREAMS" => {
                        i += 1;
                        break;
                    }
                    _ => {
                        return Frame::error("ERR syntax error");
                    }
                }
            }

            let remaining = &args[i..];
            if remaining.is_empty() || !remaining.len().is_multiple_of(2) {
                return Frame::error(
                    "ERR Unbalanced 'xread' list of streams: for each stream key an ID or '$' must be specified.",
                );
            }

            let half = remaining.len() / 2;
            let keys: Vec<String> = remaining[..half]
                .iter()
                .map(|a| String::from_utf8_lossy(a).to_string())
                .collect();

            // Resolve $ IDs to current last IDs
            let mut ids = Vec::with_capacity(half);
            {
                let inner = state.lock();
                let db = inner.db(ctx.selected_db);
                for (idx, a) in remaining[half..].iter().enumerate() {
                    let s = String::from_utf8_lossy(a).to_string();
                    if s == "$" {
                        ids.push(
                            db.stream_keys
                                .get(&keys[idx])
                                .map(|stream| stream.last_id().to_string())
                                .unwrap_or_else(|| "0-0".to_string()),
                        );
                    } else {
                        let normalized = Stream::normalize_id(&s);
                        if Stream::parse_id(&normalized).is_err() {
                            return Frame::error(
                                "ERR Invalid stream ID specified as stream command argument",
                            );
                        }
                        ids.push(normalized);
                    }
                }
            }

            // Helper closure to try reading from streams
            let try_read = |state: &SharedState,
                            keys: &[String],
                            ids: &[String],
                            count: Option<usize>|
             -> Option<Frame> {
                let inner = state.lock();
                let db = inner.db(ctx.selected_db);
                let mut results = Vec::new();

                for (idx, key) in keys.iter().enumerate() {
                    if let Some(kt) = db.keys.get(key)
                        && *kt != KeyType::Stream
                    {
                        return Some(Frame::error(MSG_WRONG_TYPE));
                    }

                    let entries = match db.stream_keys.get(key) {
                        Some(stream) => {
                            let mut entries = stream.after(&ids[idx]);
                            if let Some(c) = count {
                                entries.truncate(c);
                            }
                            entries
                        }
                        None => vec![],
                    };

                    if entries.is_empty() {
                        continue;
                    }

                    let entry_frames: Vec<Frame> = entries
                        .into_iter()
                        .map(|e| {
                            let vals: Vec<Frame> = e
                                .values
                                .iter()
                                .map(|v| Frame::Bulk(v.clone().into()))
                                .collect();
                            Frame::Array(vec![Frame::Bulk(e.id.clone().into()), Frame::Array(vals)])
                        })
                        .collect();

                    results.push(Frame::Array(vec![
                        Frame::Bulk(key.clone().into()),
                        Frame::Array(entry_frames),
                    ]));
                }

                if results.is_empty() {
                    None // No data yet
                } else {
                    Some(Frame::Array(results))
                }
            };

            // Try immediate read
            if let Some(result) = try_read(state, &keys, &ids, count) {
                return result;
            }

            // Block until data or timeout
            let timeout_ms = block_ms.unwrap_or(0);
            let timeout_dur = if timeout_ms == 0 {
                std::time::Duration::from_secs(300) // max wait
            } else {
                std::time::Duration::from_millis(timeout_ms as u64)
            };
            let deadline = tokio::time::Instant::now() + timeout_dur;

            loop {
                tokio::select! {
                    _ = state.notify.notified() => {
                        if let Some(result) = try_read(state, &keys, &ids, count) {
                            return result;
                        }
                    }
                    _ = tokio::time::sleep_until(deadline) => {
                        return Frame::NullArray;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::NullArray;
                    }
                }
            }
        }
        "XREADGROUP" => {
            if args.len() < 6 {
                return Frame::error(crate::dispatch::err_wrong_number("xreadgroup"));
            }

            let mut i = 0;
            let group_kw = String::from_utf8_lossy(&args[i]).to_uppercase();
            if group_kw != "GROUP" {
                return Frame::error("ERR syntax error");
            }
            i += 1;

            let group_name = String::from_utf8_lossy(&args[i]).to_string();
            i += 1;
            let consumer_name = String::from_utf8_lossy(&args[i]).to_string();
            i += 1;

            let mut count: Option<usize> = None;
            let mut block_ms: Option<i64> = None;
            let mut noack = false;

            while i < args.len() {
                let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
                match opt.as_str() {
                    "COUNT" => {
                        i += 1;
                        if i >= args.len() {
                            return Frame::error("ERR syntax error");
                        }
                        match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                            Ok(n) if n > 0 => count = Some(n as usize),
                            Ok(_) => count = None,
                            Err(_) => {
                                return Frame::error("ERR value is not an integer or out of range");
                            }
                        }
                        i += 1;
                    }
                    "BLOCK" => {
                        i += 1;
                        if i >= args.len() {
                            return Frame::error("ERR syntax error");
                        }
                        match String::from_utf8_lossy(&args[i]).parse::<i64>() {
                            Ok(n) if n < 0 => {
                                return Frame::error("ERR timeout is negative");
                            }
                            Ok(n) => block_ms = Some(n),
                            Err(_) => {
                                return Frame::error(
                                    "ERR timeout is not an integer or out of range",
                                );
                            }
                        }
                        i += 1;
                    }
                    "NOACK" => {
                        noack = true;
                        i += 1;
                    }
                    "STREAMS" => {
                        i += 1;
                        break;
                    }
                    _ => {
                        return Frame::error("ERR syntax error");
                    }
                }
            }

            let remaining = &args[i..];
            if remaining.is_empty() || !remaining.len().is_multiple_of(2) {
                return Frame::error(
                    "ERR Unbalanced XREADGROUP list of streams: for each stream key an ID or '$' must be specified.",
                );
            }

            let half = remaining.len() / 2;
            let keys: Vec<String> = remaining[..half]
                .iter()
                .map(|a| String::from_utf8_lossy(a).to_string())
                .collect();
            let ids: Vec<String> = remaining[half..]
                .iter()
                .map(|a| String::from_utf8_lossy(a).to_string())
                .collect();

            // If any ID is not ">", BLOCK is ignored — run synchronously
            let all_gt = ids.iter().all(|id| id == ">");
            if !all_gt {
                // Fall through to non-blocking execution via dispatch
                // Run non-blocking via dispatch
                let mut full_cmd = Vec::with_capacity(args.len() + 1);
                full_cmd.push(b"XREADGROUP".to_vec());
                full_cmd.extend(args.iter().cloned());
                let (response, _) = crate::dispatch::dispatch(
                    state.command_table.get().unwrap(),
                    state,
                    ctx,
                    &full_cmd,
                );
                return response;
            }

            // Validate group existence for all streams before blocking
            {
                let inner = state.lock();
                let db = inner.db(ctx.selected_db);
                for key in &keys {
                    if let Some(kt) = db.keys.get(key)
                        && *kt != KeyType::Stream
                    {
                        return Frame::error(MSG_WRONG_TYPE);
                    }
                    match db.stream_keys.get(key) {
                        Some(stream) => {
                            if !stream.groups.contains_key(&group_name) {
                                return Frame::error(format!(
                                    "NOGROUP No such consumer group '{}' for key name '{}'",
                                    group_name, key
                                ));
                            }
                        }
                        None => {
                            return Frame::error(format!(
                                "NOGROUP No such consumer group '{}' for key name '{}'",
                                group_name, key
                            ));
                        }
                    }
                }
            }

            // Helper closure to try reading from groups
            let try_read_group = |state: &SharedState,
                                  ctx: &ConnCtx,
                                  keys: &[String],
                                  group_name: &str,
                                  consumer_name: &str,
                                  count: Option<usize>,
                                  noack: bool|
             -> Result<Option<Frame>, Frame> {
                let mut inner = state.lock();
                let now = inner.effective_now();
                let db = inner.db_mut(ctx.selected_db);
                let mut results = Vec::new();

                for key in keys {
                    let stream = match db.stream_keys.get_mut(key) {
                        Some(s) => s,
                        None => {
                            return Err(Frame::error(format!(
                                "NOGROUP No such consumer group '{}' for key name '{}'",
                                group_name, key
                            )));
                        }
                    };

                    let entries = match stream.read_group(
                        group_name,
                        consumer_name,
                        ">",
                        count,
                        noack,
                        now,
                    ) {
                        Ok(entries) => entries,
                        Err(e) => return Err(Frame::error(e)),
                    };

                    if entries.is_empty() {
                        continue;
                    }

                    let entry_frames: Vec<Frame> = entries
                        .into_iter()
                        .map(|e| {
                            let vals: Vec<Frame> = e
                                .values
                                .iter()
                                .map(|v| Frame::Bulk(v.clone().into()))
                                .collect();
                            Frame::Array(vec![Frame::Bulk(e.id.into()), Frame::Array(vals)])
                        })
                        .collect();

                    results.push(Frame::Array(vec![
                        Frame::Bulk(key.clone().into()),
                        Frame::Array(entry_frames),
                    ]));
                }

                if results.is_empty() {
                    Ok(None)
                } else {
                    Ok(Some(Frame::Array(results)))
                }
            };

            // Try immediate read
            match try_read_group(state, ctx, &keys, &group_name, &consumer_name, count, noack) {
                Err(e) => return e,
                Ok(Some(result)) => return result,
                Ok(None) => {} // No data, proceed to blocking
            }

            // Block until data or timeout
            let timeout_ms = block_ms.unwrap_or(0);
            let timeout_dur = if timeout_ms == 0 {
                std::time::Duration::from_secs(300) // max wait
            } else {
                std::time::Duration::from_millis(timeout_ms as u64)
            };
            let deadline = tokio::time::Instant::now() + timeout_dur;

            loop {
                tokio::select! {
                    _ = state.notify.notified() => {
                        match try_read_group(state, ctx, &keys, &group_name, &consumer_name, count, noack) {
                            Err(e) => return e,
                            Ok(Some(result)) => return result,
                            Ok(None) => {} // Continue waiting
                        }
                    }
                    _ = tokio::time::sleep_until(deadline) => {
                        return Frame::NullArray;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::NullArray;
                    }
                }
            }
        }
        _ => Frame::error("ERR unsupported blocking stream command"),
    }
}

/// Convert a Frame (expected to be an Array of Bulk strings) into a
/// vector of byte vectors (the command arguments).
fn frame_to_args(frame: Frame) -> Option<Vec<Vec<u8>>> {
    match frame {
        Frame::Array(frames) => {
            let mut args = Vec::with_capacity(frames.len());
            for f in frames {
                match f {
                    Frame::Bulk(data) => args.push(data.to_vec()),
                    Frame::Simple(s) => args.push(s.into_bytes()),
                    Frame::Integer(n) => args.push(n.to_string().into_bytes()),
                    _ => return None,
                }
            }
            Some(args)
        }
        // Some clients send inline commands (single bulk string)
        Frame::Bulk(data) => {
            let s = String::from_utf8_lossy(&data);
            let args: Vec<Vec<u8>> = s
                .split_whitespace()
                .map(|w| w.as_bytes().to_vec())
                .collect();
            if args.is_empty() { None } else { Some(args) }
        }
        _ => None,
    }
}
