use encore_supervisor::config;
use encore_supervisor::proxy;
use encore_supervisor::supervisor::Supervisor;
use std::env;
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

    // Assign a unique port to each hosted service
    let mut service_ports = std::collections::HashMap::new();

    // Start assigning services to ports from the exposed port (if there are gateways, start at +1)
    let mut port: u16 = exposed_port + 1;
    for service in &supervisor_config.hosted_services {
        service_ports.insert(service.clone(), port);
        port += 1;
    }

    // Run all gateways on the first port
    let mut procs = Vec::new();

    // Create a process for each service and assign it to the selected port
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

    let proxy = proxy::GatewayProxy::new(
        std::net::SocketAddr::new("127.0.0.1".parse().unwrap(), port),
        service_ports.clone(),
    );
    let sv = Supervisor::new(procs);
    let root_token = CancellationToken::new();
    let supervisor_token = root_token.child_token();

    // Spawn a task to listen for SIGINT or SIGTERM and cancel the root token
    tokio::spawn(async move {
        // Wait for SIGINT or SIGTERM
        tokio::signal::ctrl_c()
            .await
            .expect("Failed to listen for ctrl+c");
        log::info!("Received shutdown signal. Initiating graceful shutdown...");
        root_token.cancel();
    });
    tokio::join!(
        sv.supervise(&supervisor_token),
        proxy.serve(format!("0.0.0.0:{}", exposed_port), &supervisor_token)
    );
    log::info!("All processes have exited. Shutting down.");
}
