use std::collections::HashMap;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

use crate::encore::runtime::v1 as runtimepb;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProcessConfig {
    hosted_services: Vec<String>,
    hosted_gateways: Vec<String>,
    local_service_ports: HashMap<String, u16>,
}

impl ProcessConfig {
    pub fn apply(&self, cfg: &mut runtimepb::RuntimeConfig) -> Result<()> {
        let deployment = cfg.deployment.get_or_insert_with(Default::default);

        deployment.hosted_services = self
            .hosted_services
            .iter()
            .map(|s| runtimepb::HostedService { name: s.clone() })
            .collect();
        deployment.hosted_gateways = self
            .hosted_gateways
            .iter()
            .map(|s| {
                cfg.infra
                    .as_ref()
                    .context("gateway not found in infra resources")
                    .and_then(|r| r.resources.as_ref().context("resources not found in infra"))
                    .and_then(|r| {
                        r.gateways
                            .iter()
                            .find(|g| g.encore_name == *s)
                            .context("gateway not found in infra resources")
                            .map(|r| r.rid.clone())
                    })
            })
            .collect::<Result<Vec<_>>>()?;

        let svc_discovery = deployment
            .service_discovery
            .get_or_insert_with(Default::default);
        // Iterate through service_ports and add service_discovery entries
        for (service_name, port) in &self.local_service_ports {
            let base_url = format!("http://127.0.0.1:{}", port);
            svc_discovery.services.insert(
                service_name.clone(),
                runtimepb::service_discovery::Location {
                    base_url: base_url.clone(),
                    auth_methods: deployment.auth_methods.clone(),
                },
            );
        }
        Ok(())
    }
}
