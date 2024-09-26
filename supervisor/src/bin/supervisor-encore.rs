use encore_supervisor::config;
use encore_supervisor::proxy;
use encore_supervisor::supervisor::Supervisor;
use std::env;
use std::net::IpAddr;
use std::net::Ipv4Addr;
use tokio_util::sync::CancellationToken;

#[tokio::main]
pub async fn main() {
    env_logger::init();

    let supervisor_config =
        config::load_supervisor_config().expect("could not load supervisor config");

    // Get the exposed port from the environment variable, defaulting to 8080 if not set
    let exposed_port = env::var("PORT")
        .ok()
        .and_then(|p| p.parse::<u16>().ok())
        .unwrap_or(8080);

    if supervisor_config.hosted_gateways.is_empty() && supervisor_config.hosted_services.len() > 1 {
        panic!("Cannot run supervisor with no gateways and multiple services.");
    }

    let use_proxy = !supervisor_config.hosted_gateways.is_empty()
        && !supervisor_config.hosted_services.is_empty();

    // Assign a unique port to each hosted service
    let mut service_ports = std::collections::HashMap::new();

    // Start assigning services to ports (reserve the exposed port to the proxy if it's used)
    let mut port: u16 = exposed_port + if use_proxy { 1 } else { 0 };

    for service in &supervisor_config.hosted_services {
        service_ports.insert(service.clone(), port);
        port += 1;
    }

    // Create a process for each service and assign it to the selected port
    let mut procs = Vec::new();
    for (service_name, service_port) in &service_ports {
        procs.push(
            config::create_process_config(
                vec![service_name.clone()],
                vec![],
                *service_port,
                &service_ports,
                &supervisor_config.binary_config,
            )
            .expect("Failed to create process for service"),
        );
    }

    let upstream_port = port;
    if !supervisor_config.hosted_gateways.is_empty() {
        service_ports.insert("api-gateway".to_string(), port);
        procs.push(
            config::create_process_config(
                vec![],
                supervisor_config.hosted_gateways,
                port,
                &service_ports,
                &supervisor_config.binary_config,
            )
            .expect("Failed to create process for gateways"),
        );
    }

    let mut handles = Vec::new();
    let mut results = Vec::new();
    let root_token = CancellationToken::new();

    let sv = Supervisor::new(procs);
    let supervisor_token = root_token.child_token();
    handles.push(tokio::spawn(sv.supervise(supervisor_token)));

    if use_proxy {
        let proxy = proxy::GatewayProxy::new(
            reqwest::Client::new(),
            std::net::SocketAddr::new(IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1)), upstream_port),
            service_ports.clone(),
        );
        let proxy_token = root_token.child_token();
        let proxy_fut = proxy.serve(format!("0.0.0.0:{}", exposed_port), proxy_token);
        handles.push(tokio::spawn(proxy_fut));
    }

    // Spawn a task to listen for SIGINT or SIGTERM and cancel the root token
    tokio::spawn(async move {
        // Wait for SIGINT or SIGTERM
        tokio::signal::ctrl_c()
            .await
            .expect("Failed to listen for ctrl+c");
        log::info!("Received shutdown signal. Initiating graceful shutdown...");
        root_token.cancel();
    });

    for handle in handles {
        results.push(handle.await);
    }

    for result in results {
        if let Err(e) = result {
            log::error!("Error while shutting down process: {:?}", e);
        }
    }
    log::info!("All processes have exited. Shutting down.");
}
