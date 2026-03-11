use std::net::SocketAddr;
use std::time::Duration;

use miniredis_rs::Miniredis;

/// An in-process miniredis server with a background cleanup task
/// that fast-forwards time and prunes excess keys (matching the
/// behavior of the old Go miniredis-encore binary).
///
/// The server and cleanup task run as tokio tasks and will shut
/// down when the tokio runtime is dropped.
pub struct MiniredisServer {
    server: Miniredis,
    addr: SocketAddr,
}

impl MiniredisServer {
    /// Start a new in-process miniredis server on a random port.
    ///
    /// Also spawns a background cleanup task that fast-forwards
    /// time by 1s every second and prunes keys above 100 every 15s.
    pub async fn start() -> anyhow::Result<Self> {
        let server = Miniredis::run()
            .await
            .map_err(|e| anyhow::anyhow!("failed to start miniredis: {}", e))?;
        let addr = server.addr();

        // Spawn the cleanup task on the current runtime.
        tokio::spawn(cleanup_task(server.clone()));

        Ok(Self { server, addr })
    }

    /// Returns the bound address of the miniredis server.
    pub fn addr(&self) -> SocketAddr {
        self.addr
    }

    /// Returns a reference to the underlying miniredis server.
    pub fn server(&self) -> &Miniredis {
        &self.server
    }
}

/// Background task that fast-forwards miniredis time and periodically prunes
/// excess keys, matching the Go binary's doCleanup behavior.
async fn cleanup_task(server: Miniredis) {
    let mut interval = tokio::time::interval(Duration::from_secs(1));
    let mut acc = Duration::ZERO;
    loop {
        interval.tick().await;
        server.fast_forward(Duration::from_secs(1));
        acc += Duration::from_secs(1);
        if acc >= Duration::from_secs(15) {
            acc = Duration::ZERO;
            // Prune to 100 keys.
            let keys = server.keys();
            if keys.len() > 100 {
                let to_delete = keys.len() - 100;
                for key in keys.iter().take(to_delete) {
                    server.del(key);
                }
            }
        }
    }
}
