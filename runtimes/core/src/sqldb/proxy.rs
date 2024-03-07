use std::str::FromStr;

use tokio::net::TcpStream;

pub struct Proxy {
    backend_config: tokio_postgres::Config,
}

impl Proxy {
    pub fn new(conn_uri: &str) -> Result<Self, tokio_postgres::Error> {
        let backend_config = tokio_postgres::Config::from_str(conn_uri)?;
        Ok(Self::with_config(backend_config))
    }

    pub fn with_config(backend_config: tokio_postgres::Config) -> Self {
        Self { backend_config }
    }

    pub async fn run(self, addr: impl tokio::net::ToSocketAddrs) -> anyhow::Result<()> {
        let listener = tokio::net::TcpListener::bind(addr).await?;
        log::info!("listening on {}", listener.local_addr()?);

        loop {
            let (stream, _) = listener.accept().await?;
            let config = self.backend_config.clone();

            tokio::spawn(async move {
                log::debug!("proxying connection from {}", stream.peer_addr().unwrap());
                match proxy_conn(stream, config).await {
                    Ok(()) => log::debug!("proxy connection closed"),
                    Err(err) => log::error!("proxy connection error: {}", err),
                }
            });
        }
    }
}

async fn proxy_conn(
    client_stream: TcpStream,
    config: tokio_postgres::Config,
) -> Result<(), tokio_postgres::Error> {
    // TODO handle TLS
    let mut proxy =
        tokio_postgres::proxy::proxy(client_stream, tokio_postgres::NoTls, config).await?;
    proxy.copy_data().await?;
    Ok(())
}
