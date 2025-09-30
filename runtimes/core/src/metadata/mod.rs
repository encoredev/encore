use std::collections::HashMap;

#[derive(Debug, Clone)]
pub struct ContainerMetadata {
    pub service_id: String,
    pub revision_id: String,
    pub instance_id: String,
    pub env_name: String,
}

impl ContainerMetadata {
    pub fn labels(&self) -> HashMap<String, String> {
        let mut labels = HashMap::new();
        if !self.service_id.is_empty() {
            labels.insert("service_id".to_string(), self.service_id.clone());
        }
        if !self.revision_id.is_empty() {
            labels.insert("revision_id".to_string(), self.revision_id.clone());
        }
        if !self.instance_id.is_empty() {
            labels.insert("instance_id".to_string(), self.instance_id.clone());
        }
        if !self.env_name.is_empty() {
            labels.insert("env_name".to_string(), self.env_name.clone());
        }
        labels
    }

    /// Generate a unique instance ID based on available information
    pub fn generate_instance_id() -> String {
        // For now, generate a simple random instance ID
        // In a real implementation, this would query cloud metadata services
        format!("instance-{}", &uuid::Uuid::new_v4().to_string()[..8])
    }

    /// Create container metadata from runtime config
    pub fn from_runtime_config(runtime_config: &crate::encore::runtime::v1::RuntimeConfig) -> Self {
        let environment = runtime_config.environment.as_ref();
        let deployment = runtime_config.deployment.as_ref();

        Self {
            service_id: deployment
                .map(|d| d.deploy_id.clone())
                .unwrap_or_else(|| "unknown-service".to_string()),
            revision_id: deployment
                .map(|d| d.deploy_id.clone())
                .unwrap_or_else(|| "unknown-revision".to_string()),
            instance_id: Self::generate_instance_id(),
            env_name: environment
                .map(|e| e.env_name.clone())
                .unwrap_or_else(|| "development".to_string()),
        }
    }
}

/// Process environment variable substitution in labels
/// Replaces $ENV:VARIABLE_NAME with the actual environment variable value
pub fn process_env_substitution(labels: &mut HashMap<String, String>) {
    for (_, value) in labels.iter_mut() {
        if value.starts_with("$ENV:") {
            let env_var = &value[5..];
            if let Ok(env_value) = std::env::var(env_var) {
                *value = env_value;
            }
        }
    }
}
