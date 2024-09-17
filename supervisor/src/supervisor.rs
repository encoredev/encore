//! Supervisor of Encore processes.
//!
//! The supervisor ensures all services (and gateways) hosted
//! by the Encore deployment are started and running.

use std::{ffi::OsStr, io, os::unix::ffi::OsStrExt, path::PathBuf, time::Duration};
use tokio::process::{Child, Command};
use tokio_retry::Retry;
use tokio_util::{sync::CancellationToken, task::TaskTracker};

/// The supervisor.
pub struct Supervisor {
    /// The processes to supervise.
    procs: Vec<Process>,
}

impl Supervisor {
    /// Creates a new supervisor.
    pub fn new(procs: Vec<Process>) -> Self {
        Self { procs }
    }

    /// Runs the supervisor.
    ///
    /// It returns when all processes have exited, due to either
    /// crashing or cancellation.
    pub async fn supervise(self, token: &CancellationToken) {
        let tracker = TaskTracker::new();

        for p in self.procs {
            let tok = token.clone();
            tracker.spawn(async move {
                let _ = p.run(tok.clone()).await;
                tok.cancel();
            });
        }

        tracker.close();
        tracker.wait().await;
    }
}

/// A supervised process.
pub struct Process {
    /// The name of the process, for display purposes.
    pub name: String,

    /// The binary to start.
    pub program: String,

    /// Arguments to the program.
    pub args: Vec<String>,

    /// The working directory of the process.
    pub cwd: PathBuf,

    /// Env variables to set for the process.
    /// The current process's env vars are NOT inherited.
    pub env: Vec<(String, String)>,

    /// How to restart the process if it exits.
    pub restart_policy: Box<dyn RestartPolicy>,
}

impl Process {
    /// Runs the process, waiting for it to exit.
    ///
    /// It restarts the process on exit according to the restart policy,
    /// unless the cancellation token is canceled.
    async fn run(&self, token: CancellationToken) -> io::Result<()> {
        let name = self.name.as_str();
        Retry::spawn(self.restart_policy.retries(), move || {
            let token = token.clone();

            log::info!(proc = name; "starting process");

            let res = self.run_once(token.clone());
            async move {
                // Wait for the process to complete.
                let _ = res.await;
                log::info!(proc = name; "process exited");
                if token.is_cancelled() {
                    Ok(())
                } else {
                    Err(io::Error::new(io::ErrorKind::Other, "exited"))
                }
            }
        })
        .await
    }

    async fn run_once(&self, token: CancellationToken) -> io::Result<()> {
        // If the token is already cancelled, do nothing.
        if token.is_cancelled() {
            return Err(io::Error::new(io::ErrorKind::Other, "canceled"));
        }

        let envs = self.env.iter().map(|(k, v)| {
            (
                OsStr::from_bytes(k.as_bytes()),
                OsStr::from_bytes(v.as_bytes()),
            )
        });

        let mut cmd = Command::new(&self.program)
            .args(&self.args)
            .env_clear()
            .envs(envs)
            .current_dir(&self.cwd)
            .spawn()?;

        // Wait for the process to exit, or the token to be cancelled,
        // whichever happens first.
        tokio::select! {
            status = cmd.wait() => status.map(|_| ()),

            _ = token.cancelled() => {
                kill_gracefully(&mut cmd).await
            },
        }
    }
}

/// Attempts to kill a child process gracefully.
async fn kill_gracefully(child: &mut Child) -> io::Result<()> {
    do_kill_gracefully(child).await
}

#[cfg(target_os = "windows")]
async fn do_kill_gracefully(child: &mut Child) -> io::Result<()> {
    child.kill().await
}

#[cfg(not(target_os = "windows"))]
async fn do_kill_gracefully(child: &mut Child) -> io::Result<()> {
    use std::time::Duration;
    if let Some(pid) = child.id() {
        for (sig, wait) in [
            (libc::SIGINT, Duration::from_secs(2)),
            (libc::SIGTERM, Duration::from_secs(2)),
        ] {
            unsafe {
                libc::kill(pid as i32, sig);
            }

            tokio::select! {
                _ = child.wait() => return Ok(()),
                _ = tokio::time::sleep(wait) => {
                    // Still running, escalate.
                }
            }
        }
    }

    // We're out of graceful signals.
    child.kill().await
}

pub trait RestartPolicy: Send + Sync + 'static {
    fn retries(&self) -> Box<dyn Iterator<Item = Duration> + Send + 'static>;
}

impl<T> RestartPolicy for T
where
    T: Iterator<Item = Duration> + Clone + Send + Sync + 'static,
{
    fn retries(&self) -> Box<dyn Iterator<Item = Duration> + Send + 'static> {
        Box::new(self.clone())
    }
}
