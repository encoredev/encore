use crate::supervisor::Process;
use anyhow::{Context, Result};
use base64::Engine;
use prost::Message;
use runtime::v1 as runtimepb;
use std::io::Read;
use std::time::Duration;
use std::{collections::HashMap, env, fs::File};
use tokio_retry::strategy::ExponentialBackoff;

pub mod runtime {
    pub mod v1 {
        include!(concat!(env!("OUT_DIR"), "/encore.runtime.v1.rs"));
    }
}

// loads the binary config and the runtime config and merges them into a supervisor config
pub fn load_supervisor_config() -> Result<SupervisorConfig> {
    let (services, gateways) = load_hosted_processes()?;
    Ok(SupervisorConfig {
        binary_config: load_binary_config()?,
        hosted_services: services,
        hosted_gateways: gateways,
    })
}

// loads the binary config (defaults to /encore/supervisor.config.json). It contains
// information on how to start the available binaries and what services/gateways they contain
fn load_binary_config() -> Result<BinaryConfig> {
    // Parse config path from args, defaulting to /encore/supervisor.config.json if not specified
    let args: Vec<String> = env::args().collect();
    let config_path = args
        .iter()
        .position(|arg| arg == "-c")
        .and_then(|index| args.get(index + 1))
        .map(|s| s.to_string())
        .unwrap_or_else(|| "/encore/supervisor.config.json".to_string());

    // Open and read the supervisor config file
    let mut file = File::open(config_path)?;
    let mut contents = String::new();
    file.read_to_string(&mut contents)?;
    serde_json::from_str(&contents).map_err(|e| anyhow::anyhow!(e))
}

// attempts to read the encore runtime config either as proto or json. Extracts and returns the
// hosted services and gateways.
fn load_hosted_processes() -> Result<(Vec<String>, Vec<String>)> {
    // Read and decode the runtime config bytes from the environment variable
    let runtime_config = env::var("ENCORE_RUNTIME_CONFIG")
        .context("Failed to read ENCORE_RUNTIME_CONFIG env var")
        .and_then(|encoded| {
            if encoded.starts_with("gzip:") {
                let gzipped = encoded.trim_start_matches("gzip:");
                base64::engine::general_purpose::STANDARD
                    .decode(gzipped.as_bytes())
                    .context("failed base64 decoding ENCORE_RUNTIME_CONFIG")
                    .and_then(|bytes| {
                        let mut decoder = flate2::read::GzDecoder::new(&bytes[..]);
                        let mut decompressed = Vec::new();
                        decoder
                            .read_to_end(&mut decompressed)
                            .context("failed unzipping runtime config")?;
                        Ok(decompressed)
                    })
            } else {
                base64::engine::general_purpose::STANDARD
                    .decode(encoded.as_bytes())
                    .context("failed base64 decoding ENCORE_RUNTIME_CONFIG")
            }
        })?;

    // Decode the runtime config based on its format (protobuf or JSON)
    match runtimepb::RuntimeConfig::decode(&runtime_config[..]) {
        Ok(config) => {
            let deployment = config
                .deployment
                .context("Deployment not found in RuntimeConfig")?;
            let gateways = config
                .infra
                .context("Infrastructure not found in RuntimeConfig")?
                .resources
                .context("Resources not found in Infrastructure")?
                .gateways;
            Ok((
                deployment
                    .hosted_services
                    .iter()
                    .map(|s| s.name.clone())
                    .collect(),
                deployment
                    .hosted_gateways
                    .iter()
                    .map(|rid| {
                        Ok(gateways
                            .iter()
                            .find(|g| g.rid == *rid)
                            .context("Gateway rid not found in infra resources")?
                            .encore_name
                            .clone())
                    })
                    .collect::<Result<Vec<String>>>()?,
            ))
        }
        Err(_) => {
            // If protobuf decoding fails, try JSON decoding
            let config: RuntimeConfig = serde_json::from_slice(&runtime_config)
                .context("Failed to parse RuntimeConfig as JSON")?;
            Ok((
                config.hosted_services,
                config.gateways.iter().map(|g| g.name.clone()).collect(),
            ))
        }
    }
}

// Create a process config for a given set of services and gateways
pub fn create_process_config(
    services: Vec<String>,
    gateways: Vec<String>,
    port: u16,
    service_ports: &HashMap<String, u16>,
    cfg: &BinaryConfig,
) -> Result<Process> {
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
        .context(format!(
            "No matching proc found for services {:?} gateways {:?}",
            services, gateways
        ))?;

    // Add proc-specific environment variables
    env.extend(binary_config.env.iter().map(|e| {
        let parts: Vec<&str> = e.splitn(2, '=').collect();
        (
            parts[0].to_string(),
            parts.get(1).unwrap_or(&"").to_string(),
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
                .context("Failed to serialize ProcessConfig")?,
            ),
        ),
    ]);

    let policy = ExponentialBackoff::from_millis(100).max_delay(Duration::from_millis(1000));

    Ok(Process {
        name: binary_config.id.clone(),
        program: binary_config
            .command
            .first()
            .context("missing binary command")?
            .to_string(),
        args: binary_config.command[1..].to_vec(),
        env: env.into_iter().collect(),
        cwd: std::env::current_dir().context("Failed to get current directory")?,
        restart_policy: Box::new(policy),
    })
}

// Supervisor config is the config bundled with the supervisor binary
// It contains the list of available binaries and which services and gateways they implement
#[derive(serde::Serialize, serde::Deserialize)]
pub struct SupervisorConfig {
    pub binary_config: BinaryConfig,
    pub hosted_services: Vec<String>,
    pub hosted_gateways: Vec<String>,
}

#[derive(serde::Serialize, serde::Deserialize)]
pub struct BinaryConfig {
    pub procs: Vec<Proc>,
}

#[derive(serde::Serialize, serde::Deserialize)]
pub struct Proc {
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
    pub hosted_services: Vec<String>,
    #[serde(default)]
    pub gateways: Vec<GatewayConfig>,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct GatewayConfig {
    pub name: String,
}
