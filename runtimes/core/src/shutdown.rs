use std::sync::{Arc, Mutex};
use std::time::Duration;

use tokio_util::sync::CancellationToken;

use crate::encore::runtime::v1 as runtimepb;

/// Default total shutdown time.
const DEFAULT_TOTAL: Duration = Duration::from_secs(5);

/// Grace period after an exit has been requested before the process is
/// force-terminated with `force_exit`. Must be generous enough to cover the
/// host's orderly exit path (worker-thread termination plus Node teardown).
const FORCE_EXIT_GRACE: Duration = Duration::from_secs(3);

/// Configuration for graceful shutdown timings.
#[derive(Debug, Clone)]
pub struct ShutdownConfig {
    /// Total time allowed for the entire shutdown process.
    pub total: Duration,

    /// How long to keep accepting requests after the shutdown signal,
    /// giving load balancers time to observe the 503 healthz and stop routing.
    /// Controlled by the `ENCORE_K8S_GRACE_TERMINATION_SECONDS` env var.
    pub keep_accepting: Duration,
}

impl ShutdownConfig {
    pub fn from_proto(gs: Option<runtimepb::GracefulShutdown>) -> Self {
        let gs = match gs {
            Some(gs) => gs,
            None => return Self::default(),
        };

        let total = gs
            .total
            .and_then(|d| Duration::try_from(d).ok())
            .unwrap_or(DEFAULT_TOTAL);

        // Note: the proto also defines `shutdown_hooks` and `handlers` for cancelling
        // user-defined shutdown hooks and in-flight handler contexts respectively.
        // The Rust runtime doesn't support these yet — JS handlers don't receive a
        // cancellation token/AbortSignal, so there's no way to cooperatively cancel them.
        // These fields are intentionally ignored until that mechanism exists.

        let keep_accepting = parse_k8s_grace_period(total);

        Self {
            total,
            keep_accepting,
        }
    }
}

impl Default for ShutdownConfig {
    fn default() -> Self {
        Self {
            total: DEFAULT_TOTAL,
            keep_accepting: Duration::ZERO,
        }
    }
}

/// Parses the `ENCORE_K8S_GRACE_TERMINATION_SECONDS` environment variable.
/// Returns how long to keep accepting requests after receiving a SIGTERM,
/// giving load balancers time to detect the unhealthy status.
fn parse_k8s_grace_period(total: Duration) -> Duration {
    let Ok(val) = std::env::var("ENCORE_K8S_GRACE_TERMINATION_SECONDS") else {
        return Duration::ZERO;
    };
    let Ok(secs) = val.parse::<u64>() else {
        return Duration::ZERO;
    };
    let k8s_grace = Duration::from_secs(secs);

    // The keep-accepting period is the K8s grace period minus our total
    // shutdown time, so we have enough time to drain after accepting stops.
    k8s_grace.saturating_sub(total)
}

/// Coordinates the process exit code between the graceful-shutdown path, the
/// force-exit watchdog, and the host-language layer that performs the actual
/// exit (e.g. `process.exit(code)` on Node's main thread).
///
/// The first request wins; later requests are ignored. This makes the
/// graceful path and the watchdog race-safe: whoever reaches their exit
/// condition first decides the code, and the host observes a single value.
pub(crate) struct ExitCell {
    code: Mutex<Option<i32>>,
    notify: tokio::sync::Notify,
}

impl ExitCell {
    pub(crate) fn new() -> Self {
        Self {
            code: Mutex::new(None),
            notify: tokio::sync::Notify::new(),
        }
    }

    /// Requests that the process exit with the given code.
    /// The first request wins; subsequent requests are no-ops.
    ///
    /// Once an exit has been requested the process must terminate, so the
    /// first request also arms a backstop that force-exits (with the same
    /// code) if the host's exit path hasn't terminated the process within
    /// the grace period — e.g. a wedged event loop, or a worker thread stuck
    /// in native code during `worker.terminate()`. This covers every exit
    /// origin (graceful completion, the watchdog deadline, and panic
    /// recovery) without depending on a signal having been received.
    pub(crate) fn request(&self, code: i32) {
        {
            let mut guard = self.code.lock().expect("exit cell lock poisoned");
            if guard.is_some() {
                return;
            }
            *guard = Some(code);
            self.notify.notify_waiters();
        }

        let spawned = std::thread::Builder::new()
            .name("encore-exit-backstop".into())
            .spawn(move || {
                std::thread::sleep(FORCE_EXIT_GRACE);
                ::log::error!(
                    "process still alive {FORCE_EXIT_GRACE:?} after exit was requested, force-exiting"
                );
                force_exit(code);
            });
        if let Err(err) = spawned {
            // Extremely unlikely (thread exhaustion). The exit code is already
            // recorded and waiters notified, so the normal exit path still
            // works — we just lose the wedge protection.
            ::log::error!("failed to spawn exit backstop thread: {err}");
        }
    }

    /// Resolves once an exit code has been requested.
    pub(crate) async fn wait(&self) -> i32 {
        loop {
            // Register interest before checking the state so a request
            // between the check and the await can't be missed.
            let notified = self.notify.notified();
            if let Some(code) = *self.code.lock().expect("exit cell lock poisoned") {
                return code;
            }
            notified.await;
        }
    }
}

/// Terminates the process immediately without running C `atexit` handlers or
/// C++ static destructors.
///
/// We must NOT use `std::process::exit` (which calls libc `exit`) here. The
/// runtime is embedded in Node via a NAPI addon, and the exit is driven from a
/// background thread while Node's event loop, V8, libuv, and worker threads are
/// all still live. libc `exit` runs the global teardown chain on the calling
/// thread — Node/V8 platform teardown and OpenSSL's `OPENSSL_cleanup` (Node
/// registers it via `atexit`; it frees process-global crypto state) — while
/// those other threads keep using that very state. That data race crashes the
/// majority of shutdowns under load (SIGSEGV/SIGABRT).
///
/// `_exit` terminates the process immediately (`exit_group(2)` on Linux) with
/// no handlers and no destructors — exactly what we want when force-terminating
/// with live threads. It is reached only from the exit backstop (the host
/// failed to exit within the grace period after an exit was requested; see
/// [`ExitCell::request`]) and from test mode, where nothing awaits the exit
/// code. Known tradeoff: log lines emitted immediately before this call go
/// through the async log writer thread (`log::writers`) and may not reach
/// stderr before termination; the process exit code remains the authoritative
/// shutdown signal.
#[cfg(unix)]
pub fn force_exit(code: i32) -> ! {
    use std::io::Write;

    // Best-effort flush of Rust's stdio buffers (stderr is unbuffered; the
    // runtime doesn't write Rust stdout, so this is insurance for stray
    // debug output).
    let _ = std::io::stdout().flush();
    let _ = std::io::stderr().flush();
    // SAFETY: `_exit` has no preconditions and is safe to call from any
    // thread; it never returns.
    unsafe { libc::_exit(code) }
}

/// On non-unix platforms we keep the standard exit. The crash this guards
/// against is specific to the Linux/glibc + Node + OpenSSL teardown chain that
/// production deployments run on, and `_exit` semantics differ on Windows.
#[cfg(not(unix))]
pub fn force_exit(code: i32) -> ! {
    std::process::exit(code)
}

/// Handle used by components to observe when shutdown has been initiated.
///
/// When a SIGINT/SIGTERM signal is received, the handle fires, signaling
/// components to stop accepting new work. Health checks return 503.
#[derive(Clone)]
pub struct ShutdownHandle {
    initiated: CancellationToken,
}

impl ShutdownHandle {
    /// Returns a future that completes when shutdown is initiated.
    pub async fn cancelled(&self) {
        self.initiated.cancelled().await;
    }
}

/// Waits for a shutdown signal (SIGINT/SIGTERM on Unix, Ctrl-C/Ctrl-Break/Ctrl-Close on Windows).
#[cfg(unix)]
async fn wait_for_signal() {
    use tokio::signal::unix::{signal, SignalKind};

    let mut sigint = signal(SignalKind::interrupt()).expect("failed to install SIGINT handler");
    let mut sigterm = signal(SignalKind::terminate()).expect("failed to install SIGTERM handler");

    tokio::select! {
        _ = sigint.recv() => {
            ::log::info!("received SIGINT, initiating graceful shutdown");
        },
        _ = sigterm.recv() => {
            ::log::info!("received SIGTERM, initiating graceful shutdown");
        },
    }
}

#[cfg(windows)]
async fn wait_for_signal() {
    use tokio::signal::windows::{ctrl_break, ctrl_c, ctrl_close};

    let mut ctrl_c = ctrl_c().expect("failed to install Ctrl-C handler");
    let mut ctrl_break = ctrl_break().expect("failed to install Ctrl-Break handler");
    let mut ctrl_close = ctrl_close().expect("failed to install Ctrl-Close handler");

    tokio::select! {
        _ = ctrl_c.recv() => {
            ::log::info!("received Ctrl-C, initiating graceful shutdown");
        },
        _ = ctrl_break.recv() => {
            ::log::info!("received Ctrl-Break, initiating graceful shutdown");
        },
        _ = ctrl_close.recv() => {
            ::log::info!("received Ctrl-Close, initiating graceful shutdown");
        },
    }
}

/// Runs the graceful shutdown sequence.
///
/// This function:
/// 1. Waits for a shutdown signal (SIGINT/SIGTERM) in a background task
/// 2. Initiates graceful shutdown (components stop accepting new work)
/// 3. If the total deadline is exceeded, requests a process exit (code 1)
///    through the host's normal exit path
///
/// The watchdog is a safety net — normally `run_blocking` finishes before the
/// deadline and the JS layer exits the process cleanly from Node's main
/// thread. The deadline request covers a wedged drain (the host is healthy,
/// so it can still exit normally); a host that can't exit at all (wedged
/// event loop, stuck worker termination) is covered by the force-exit
/// backstop that arming any exit request starts (see [`ExitCell::request`]).
pub(crate) async fn run(config: ShutdownConfig, exit: Arc<ExitCell>) -> ShutdownHandle {
    let initiated = CancellationToken::new();
    let handle = ShutdownHandle {
        initiated: initiated.clone(),
    };

    tokio::spawn(async move {
        // Wait for shutdown signal.
        wait_for_signal().await;

        // Initiate shutdown — components stop accepting new work.
        initiated.cancel();

        // When the deadline is reached, request a process exit through the
        // host's normal exit path — the same mechanism graceful completion
        // uses (first request wins). The deadline includes the keep_accepting
        // grace period (K8s LB propagation) plus the total drain/flush time.
        // Normally the process has exited well before this and this request
        // is never made.
        tokio::time::sleep(config.keep_accepting + config.total).await;
        ::log::warn!("graceful shutdown deadline reached, requesting process exit");
        exit.request(1);
    });

    handle
}
