use std::collections::HashMap;

use napi_derive::napi;

#[napi(object)]
#[derive(Debug, Clone)]
pub struct Metric {
    pub name: String,
    pub services: Vec<String>,
}

#[napi(object)]
#[derive(Debug, Clone)]
pub struct RuntimeConfig {
    pub metrics: HashMap<String, Metric>,
}

impl From<encore_runtime_core::runtime_config::Metric> for Metric {
    fn from(metric: encore_runtime_core::runtime_config::Metric) -> Self {
        Self {
            name: metric.name,
            services: metric.services,
        }
    }
}

impl From<encore_runtime_core::runtime_config::RuntimeConfig> for RuntimeConfig {
    fn from(config: encore_runtime_core::runtime_config::RuntimeConfig) -> Self {
        Self {
            metrics: config
                .metrics
                .into_iter()
                .map(|(k, v)| (k, v.into()))
                .collect(),
        }
    }
}
