use crate::encore::runtime::v1 as runtimepb;
use base64::Engine;
use encore_supervisor::supervisor::{Process, Supervisor};
use prost::Message;
use std::fs::File;
use std::io::Read;
use std::{collections::HashMap, env};
use tokio_util::sync::CancellationToken;

pub mod encore {
    pub mod runtime {
        pub mod v1 {
            include!(concat!(env!("OUT_DIR"), "/encore.runtime.v1.rs"));
        }
    }
}

// Supervisor config is the config bundled with the supervisor binary
// It contains the list of available binaries and which services and gateways they implement
#[derive(serde::Serialize, serde::Deserialize)]
struct SupervisorConfig {
    procs: Vec<Proc>,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct Proc {
    id: String,
    command: Vec<String>,
    env: Vec<String>,
    services: Vec<String>,
    gateways: Vec<String>,
}

// Process config is the config for a given process
// It overrides the RuntimeConfig for local service discovery, port allocation
// and which services and gateways are hosted on this process
#[derive(serde::Serialize, serde::Deserialize)]
struct ProcessConfig {
    local_service_ports: HashMap<String, u16>,
    hosted_gateways: Vec<String>,
    hosted_services: Vec<String>,
}

// RuntimeConfig is a partial version of the config used for GO apps
// Only hosted services and gateways are parsed as those are the only ones
// we need to produce ProcessConfigs
#[derive(serde::Serialize, serde::Deserialize)]
struct RuntimeConfig {
    #[serde(default)]
    hosted_services: Vec<String>,
    #[serde(default)]
    gateways: Vec<GatewayConfig>,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct GatewayConfig {
    name: String,
}

// Create a process config for a given set of services and gateways
fn create_process_config(
    services: Vec<String>,
    gateways: Vec<String>,
    port: u16,
    service_ports: &HashMap<String, u16>,
    cfg: &SupervisorConfig,
) -> Result<Process, String> {
    // Append all supervisor environment variables
    let mut env = std::env::vars().collect::<HashMap<String, String>>();

    // Find a process config that contains all the services and gateways
    let binary_config = cfg
        .procs
        .iter()
        .find(|p| {
            (services.iter().all(|s| p.services.contains(s)))
                && (gateways.iter().all(|g| p.gateways.contains(g)))
        })
        .ok_or_else(|| {
            format!(
                "No matching proc found for services {:?} gateways {:?}",
                services, gateways
            )
        })?;

    // Add proc-specific environment variables
    env.extend(binary_config.env.iter().map(|e| {
        let parts: Vec<&str> = e.splitn(2, '=').collect();
        (
            parts[0].to_string(),
            parts.get(1).map_or("".to_string(), |&v| v.to_string()),
        )
    }));

    // Add the port and process config to the environment
    env.extend(vec![
        ("PORT".to_string(), port.to_string()),
        (
            "ENCORE_PROCESS_CONFIG".to_string(),
            base64::engine::general_purpose::STANDARD.encode(
                serde_json::to_string(&ProcessConfig {
                    hosted_gateways: gateways,
                    hosted_services: services,
                    local_service_ports: service_ports.clone(),
                })
                .unwrap(),
            ),
        ),
    ]);

    Ok(Process {
        name: binary_config.id.clone(),
        program: binary_config
            .command
            .first()
            .unwrap_or(&"".to_string())
            .to_string(),
        args: binary_config.command[1..].to_vec(),
        env: env.into_iter().collect(),
        cwd: std::env::current_dir().expect("Failed to get current directory"),
        restart_policy: Box::new(std::iter::empty()),
    })
}

#[tokio::main]
pub async fn main() {
    env_logger::init();

    // Parse config path from args, defaulting to /encore/supervisor.config.json if not specified
    let args: Vec<String> = env::args().collect();
    let config_path = args
        .iter()
        .position(|arg| arg == "-c")
        .and_then(|index| args.get(index + 1))
        .map(|s| s.to_string())
        .unwrap_or_else(|| "/encore/supervisor.config.json".to_string());

    // Open and read the supervisor config file
    let supervisor_config: SupervisorConfig = {
        let mut file = File::open(config_path).expect("Failed to open supervisor_config.json");
        let mut contents = String::new();
        file.read_to_string(&mut contents)
            .expect("Failed to read supervisor_config.json");
        serde_json::from_str(&contents).expect("Failed to parse SupervisorConfig JSON")
    };

    // Read and decode the runtime config bytes from the environment variable
    let runtime_config = env::var("ENCORE_RUNTIME_CONFIG")
        .map_err(|e| format!("Failed to read ENCORE_RUNTIME_CONFIG: {}", e))
        .and_then(|encoded| {
            if encoded.starts_with("gzip:") {
                let gzipped = encoded.trim_start_matches("gzip:");
                base64::engine::general_purpose::STANDARD
                    .decode(gzipped.as_bytes())
                    .map_err(|e| format!("Failed to decode base64: {}", e))
                    .and_then(|bytes| {
                        let mut decoder = flate2::read::GzDecoder::new(&bytes[..]);
                        let mut decompressed = Vec::new();
                        decoder
                            .read_to_end(&mut decompressed)
                            .map_err(|e| format!("Failed to decompress gzip data: {}", e))?;
                        Ok(decompressed)
                    })
            } else {
                base64::engine::general_purpose::STANDARD
                    .decode(encoded.as_bytes())
                    .map_err(|e| format!("Failed to decode base64: {}", e))
            }
        })
        .expect("Failed to parse RuntimeConfig");

    // Decode the runtime config based on its format (protobuf or JSON)
    let (hosted_services, hosted_gateways) =
        match runtimepb::RuntimeConfig::decode(&runtime_config[..]) {
            Ok(config) => {
                let deployment = config
                    .deployment
                    .expect("Deployment not found in RuntimeConfig");
                let gateways = config
                    .infra
                    .expect("Infrastructure not found in RuntimeConfig")
                    .resources
                    .expect("Resources not found in Infrastructure")
                    .gateways;
                (
                    deployment
                        .hosted_services
                        .iter()
                        .map(|s| s.name.clone())
                        .collect(),
                    deployment
                        .hosted_gateways
                        .iter()
                        .map(|rid| {
                            gateways
                                .iter()
                                .find(|g| g.rid == *rid)
                                .expect("Gateway rid not found in infra resources")
                                .encore_name
                                .clone()
                        })
                        .collect::<Vec<String>>(),
                )
            }
            Err(_) => {
                // If protobuf decoding fails, try JSON decoding
                let config: RuntimeConfig = serde_json::from_slice(&runtime_config)
                    .expect("Failed to parse RuntimeConfig as JSON");
                (
                    config.hosted_services,
                    config.gateways.iter().map(|g| g.name.clone()).collect(),
                )
            }
        };

    // Get the exposed port from the environment variable, defaulting to 8080 if not set
    let exposed_port = env::var("PORT")
        .ok()
        .and_then(|p| p.parse::<u16>().ok())
        .unwrap_or(8080);

    // Assign a unique port to each hosted service
    let mut service_ports = std::collections::HashMap::new();

    // Start assigning services to ports from the exposed port (if there are gateways, start at +1)
    let mut port: u16 = exposed_port + if hosted_gateways.is_empty() { 0 } else { 1 };
    for service in &hosted_services {
        service_ports.insert(service.clone(), port);
        port += 1;
    }

    // Run all gateways on the first port
    let mut procs = Vec::new();
    if !hosted_gateways.is_empty() {
        procs.push(
            create_process_config(
                vec![],
                hosted_gateways,
                exposed_port,
                &service_ports,
                &supervisor_config,
            )
            .expect("Failed to create process for gateways"),
        );
    }

    // Create a process for each service and assign it to the selected port
    for (service_name, service_port) in &service_ports {
        procs.push(
            create_process_config(
                vec![service_name.clone()],
                vec![],
                *service_port,
                &service_ports,
                &supervisor_config,
            )
            .expect("Failed to create process for service"),
        );
    }

    let sv = Supervisor::new(procs);
    let root_token = CancellationToken::new();
    let supervisor_token = root_token.child_token();

    // Spawn a task to listen for SIGINT or SIGTERM and cancel the root token
    tokio::spawn(async move {
        // Wait for SIGINT or SIGTERM
        tokio::signal::ctrl_c()
            .await
            .expect("Failed to listen for ctrl+c");
        println!("Received shutdown signal. Initiating graceful shutdown...");
        root_token.cancel();
    });

    // Run the supervisor
    sv.supervise(supervisor_token).await;
    println!("All processes have exited. Shutting down.");
}
