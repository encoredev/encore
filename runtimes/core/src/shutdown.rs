use std::time::Duration;

use tokio_util::sync::CancellationToken;

use crate::encore::runtime::v1 as runtimepb;

/// Default total shutdown time.
const DEFAULT_TOTAL: Duration = Duration::from_secs(5);

/// Default grace period after force shutdown before process exit.
const FORCE_EXIT_GRACE: Duration = Duration::from_secs(1);

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
/// 3. Force-exits the process if the total deadline is exceeded
///
/// The force exit is a safety net — normally `run_blocking` returns before
/// the deadline, and the process exits cleanly when the tokio runtime drops.
pub async fn run(config: ShutdownConfig) -> ShutdownHandle {
    let initiated = CancellationToken::new();
    let handle = ShutdownHandle {
        initiated: initiated.clone(),
    };

    tokio::spawn(async move {
        // Wait for shutdown signal.
        wait_for_signal().await;

        // Initiate shutdown — components stop accepting new work.
        initiated.cancel();

        // Safety net: force-exit the process if shutdown takes too long.
        // The deadline includes the keep_accepting grace period (K8s LB propagation)
        // plus the total drain/flush time.
        // Normally run_blocking calls process::exit(0) well before this.
        tokio::time::sleep(config.keep_accepting + config.total).await;
        ::log::warn!("graceful shutdown deadline reached, forcing exit");
        tokio::time::sleep(FORCE_EXIT_GRACE).await;
        ::log::error!("force shutdown grace period exceeded, exiting");
        std::process::exit(1);
    });

    handle
}
