use crate::metrics::gauge;

use super::{memory_usage_gauge_schema, Registry};
use std::sync::Mutex;
use sysinfo::System;

/// Collector for system metrics
#[derive(Debug)]
pub struct SystemMetricsCollector {
    system: Mutex<System>,
    memory_schema: std::sync::OnceLock<gauge::Schema<u64>>,
}

impl SystemMetricsCollector {
    pub fn new() -> Self {
        let mut system = System::new();
        Self::refresh(&mut system);

        Self {
            system: Mutex::new(system),
            memory_schema: std::sync::OnceLock::new(),
        }
    }

    fn refresh(system: &mut System) {
        system.refresh_memory();
    }

    /// Collect and record all system metrics directly to the metrics registry
    pub fn update(&self, registry: &std::sync::Arc<Registry>) {
        let mut system = match self.system.try_lock() {
            Ok(guard) => guard,
            Err(_) => {
                // Already an update in progress
                return;
            }
        };

        Self::refresh(&mut system);

        // Initialize schemas if not already done (lazy initialization)
        let memory_schema = self
            .memory_schema
            .get_or_init(|| memory_usage_gauge_schema(registry));

        // Record memory metrics using schema
        memory_schema.set(system.used_memory());
    }
}

impl Default for SystemMetricsCollector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_system_metrics_collector_creation() {
        use crate::metrics::Manager;

        let manager = Manager::new();
        let collector = SystemMetricsCollector::new();

        // Update system metrics using the registry directly
        collector.update(manager.registry());

        let collected = manager.collect_metrics();

        // Check if system metrics were recorded
        let memory_metrics: Vec<_> = collected
            .iter()
            .filter(|m| m.key.name() == "e_sys_memory_used_bytes")
            .collect();

        // Should have memory metrics
        assert!(
            !memory_metrics.is_empty(),
            "Memory metrics should be present"
        );
    }
}
