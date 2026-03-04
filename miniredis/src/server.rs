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
                                let confirm = Frame::Array(vec![
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
                                    let confirm = Frame::Array(vec![
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
                                        let confirm = Frame::Array(vec![
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
                                    let confirm = Frame::Array(vec![
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
                                let confirm = Frame::Array(vec![
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
                                    let confirm = Frame::Array(vec![
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
                                        let confirm = Frame::Array(vec![
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
                                    let confirm = Frame::Array(vec![
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
                                let pong = Frame::Array(vec![
                                    Frame::Bulk("pong".into()),
                                    Frame::Bulk("".into()),
                                ]);
                                if conn.write_frame(&pong).await.is_err() {
                                    return;
                                }
                            } else {
                                let msg = String::from_utf8_lossy(&cmd_args[0]).to_string();
                                let pong = Frame::Array(vec![
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
                                "message" => Frame::Array(vec![
                                    Frame::Bulk("message".into()),
                                    Frame::Bulk(m.channel.into()),
                                    Frame::Bulk(m.data.into()),
                                ]),
                                "pmessage" => Frame::Array(vec![
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
                    if cmd == "SUBSCRIBE" || cmd == "PSUBSCRIBE" {
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
                                let confirm = Frame::Array(vec![
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
                                let confirm = Frame::Array(vec![
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
                    if cmd == "UNSUBSCRIBE" {
                        let confirm = Frame::Array(vec![
                            Frame::Bulk("unsubscribe".into()),
                            Frame::Null,
                            Frame::Integer(0),
                        ]);
                        let _ = conn.write_frame(&confirm).await;
                        continue;
                    }
                    if cmd == "PUNSUBSCRIBE" {
                        let confirm = Frame::Array(vec![
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

                    let (response, should_close) = dispatch(table, state, ctx, &args);

                    // Sync RESP3 flag (set by HELLO command)
                    conn.resp3 = ctx.resp3;

                    if conn.write_frame(&response).await.is_err() {
                        return;
                    }

                    if should_close {
                        return;
                    }
                }
                _ = shutdown_rx.recv() => {
                    return;
                }
            }
        }
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
        MSG_INVALID_TIMEOUT, MSG_SYNTAX_ERROR, MSG_TIMEOUT_IS_OUT_OF_RANGE, MSG_WRONG_TYPE,
    };
    use crate::types::KeyType;

    match cmd {
        "BLPOP" | "BRPOP" => {
            if args.len() < 2 {
                return Frame::error(crate::dispatch::err_wrong_number(&cmd.to_lowercase()));
            }
            let keys = &args[..args.len() - 1];
            let timeout_s: f64 = match String::from_utf8_lossy(&args[args.len() - 1]).parse() {
                Ok(t) => t,
                Err(_) => return Frame::error(MSG_INVALID_TIMEOUT),
            };
            if timeout_s < 0.0 {
                return Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE);
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
                        return Frame::Null;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::Null;
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
            let timeout_s: f64 = match String::from_utf8_lossy(&args[2]).parse() {
                Ok(t) => t,
                Err(_) => return Frame::error(MSG_INVALID_TIMEOUT),
            };
            if timeout_s < 0.0 {
                return Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE);
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
                        return Frame::Null;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::Null;
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

            let timeout_s: f64 = match String::from_utf8_lossy(&args[4]).parse() {
                Ok(t) => t,
                Err(_) => return Frame::error(MSG_INVALID_TIMEOUT),
            };
            if timeout_s < 0.0 {
                return Frame::error(MSG_TIMEOUT_IS_OUT_OF_RANGE);
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
                            state.notify.notify_waiters();
                            return Frame::Bulk(v.into());
                        }
                    }
                    _ = tokio::time::sleep_until(deadline) => {
                        return Frame::Null;
                    }
                    _ = shutdown_rx.recv() => {
                        return Frame::Null;
                    }
                }
            }
        }
        _ => Frame::error("ERR unsupported blocking command"),
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
