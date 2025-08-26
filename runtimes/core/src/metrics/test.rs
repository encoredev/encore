use super::*;
use metrics::{Key, Label, Metadata, Recorder};
use std::sync::Arc;

#[cfg(test)]
mod counter_tests {
    use super::*;

    #[test]
    fn test_counter_with_labels() {
        let manager = Manager::new();

        manager.with_local_recorder(|| {
            let counter = Counter::new("test_counter_with_labels").with_labels([
                ("service", "test_service".to_string()),
                ("method", "GET".to_string()),
            ]);
            counter.increment();
        });

        let collected = manager.collect_metrics();
        let metric = collected
            .iter()
            .find(|m| m.name == "test_counter_with_labels");
        assert!(metric.is_some());

        let m = metric.unwrap();
        assert_eq!(m.labels.len(), 2);
        assert!(m
            .labels
            .contains(&("service".to_string(), "test_service".to_string())));
        assert!(m
            .labels
            .contains(&("method".to_string(), "GET".to_string())));
    }

    #[test]
    fn test_counter_increment() {
        let manager = Manager::new();

        manager.with_local_recorder(|| {
            let counter = Counter::new("test_increment");
            counter.increment();
        });

        let collected = manager.collect_metrics();
        assert!(!collected.is_empty());

        let metric = collected.iter().find(|m| m.name == "test_increment");
        assert!(metric.is_some());

        if let Some(m) = metric {
            match m.value {
                MetricValue::Counter(value) => assert_eq!(value, 1),
                _ => panic!("Expected counter value"),
            }
        }
    }

    #[test]
    fn test_counter_increment_with_extra_labels() {
        let manager = Manager::new();

        manager.with_local_recorder(|| {
            let counter = Counter::new("test_increment_with_labels")
                .with_labels([("service", "test".to_string())]);

            counter.increment_with([("status", "200"), ("method", "POST")]);
        });

        let collected = manager.collect_metrics();
        let metric = collected
            .iter()
            .find(|m| m.name == "test_increment_with_labels");
        assert!(metric.is_some());

        if let Some(m) = metric {
            assert_eq!(m.labels.len(), 3); // base + 2 extra labels
            assert!(m
                .labels
                .contains(&("service".to_string(), "test".to_string())));
            assert!(m
                .labels
                .contains(&("status".to_string(), "200".to_string())));
            assert!(m
                .labels
                .contains(&("method".to_string(), "POST".to_string())));
        }
    }
}

#[cfg(test)]
mod registry_tests {
    use super::*;
    use std::time::Duration;

    #[test]
    fn test_registry_collect_counters() {
        let registry = Registry::new();

        // Register a counter manually
        let key = Key::from_parts("test_counter", vec![Label::new("label", "value")]);
        let metadata = Metadata::new(module_path!(), metrics::Level::INFO, Some(module_path!()));
        let counter = registry.register_counter(&key, &metadata);
        counter.increment(5);

        let collected = registry.collect();
        assert_eq!(collected.len(), 1);

        let metric = &collected[0];
        assert_eq!(metric.name, "test_counter");
        assert_eq!(metric.labels.len(), 1);
        match metric.value {
            MetricValue::Counter(value) => assert_eq!(value, 5),
            _ => panic!("Expected counter value"),
        }
    }

    #[test]
    fn test_registry_collect_gauges() {
        let registry = Registry::new();

        let key = Key::from_parts("test_gauge", vec![]);
        let metadata = Metadata::new(module_path!(), metrics::Level::INFO, Some(module_path!()));
        let gauge = registry.register_gauge(&key, &metadata);
        gauge.set(42.5);

        let collected = registry.collect();
        assert_eq!(collected.len(), 1);

        let metric = &collected[0];
        assert_eq!(metric.name, "test_gauge");
        match metric.value {
            MetricValue::Gauge(value) => assert!((value - 42.5).abs() < f64::EPSILON),
            _ => panic!("Expected gauge value"),
        }
    }

    #[test]
    fn test_registry_first_seen() {
        let registry = Registry::new();
        let key = Key::from_parts("test_metric", vec![]);

        let before = std::time::SystemTime::now();
        let metadata = Metadata::new(module_path!(), metrics::Level::INFO, Some(module_path!()));
        let _counter = registry.register_counter(&key, &metadata);
        let after = std::time::SystemTime::now();

        let first_seen = registry.first_seen();
        let recorded_time = first_seen.get(&key.get_hash());
        assert!(recorded_time.is_some());

        let time = *recorded_time.unwrap();
        assert!(time >= before && time <= after);

        // re-register should not update first seen
        let _counter = registry.register_counter(&key, &metadata);
        let first_seen2 = registry.first_seen();
        let recorded_time2 = first_seen2.get(&key.get_hash());
        let time2 = *recorded_time2.unwrap();

        assert_eq!(time, time2)
    }

    #[test]
    fn test_registry_first_seen_only_once() {
        let registry = Registry::new();
        let key = Key::from_parts("test_metric", vec![]);

        let metadata = Metadata::new(module_path!(), metrics::Level::INFO, Some(module_path!()));
        let _counter1 = registry.register_counter(&key, &metadata);
        std::thread::sleep(Duration::from_millis(10));
        let _counter2 = registry.register_counter(&key, &metadata);

        let first_seen = registry.first_seen();
        assert_eq!(first_seen.len(), 1); // Only one entry should exist
    }
}

#[cfg(test)]
mod manager_tests {
    use super::*;

    #[test]
    fn test_manager_new() {
        let manager = Manager::new();
        // Test that manager starts with empty metrics
        assert!(manager.collect_metrics().is_empty());
    }

    #[test]
    fn test_manager_collect_metrics() {
        let manager = Manager::new();

        manager.with_local_recorder(|| {
            let counter = Counter::new("test_manager_counter");
            counter.increment();
        });

        let collected = manager.collect_metrics();
        assert!(!collected.is_empty());

        let metric = collected.iter().find(|m| m.name == "test_manager_counter");
        assert!(metric.is_some());
    }

    #[derive(Debug)]
    struct MockExporter {
        exported_metrics: Arc<std::sync::Mutex<Vec<Vec<CollectedMetric>>>>,
    }

    impl MockExporter {
        fn new() -> Self {
            Self {
                exported_metrics: Arc::new(std::sync::Mutex::new(Vec::new())),
            }
        }

        fn get_exported_metrics(&self) -> Vec<Vec<CollectedMetric>> {
            self.exported_metrics.lock().unwrap().clone()
        }
    }

    #[async_trait::async_trait]
    impl super::super::manager::Exporter for MockExporter {
        async fn export(&self, metrics: Vec<CollectedMetric>) {
            self.exported_metrics.lock().unwrap().push(metrics);
        }
    }

    #[tokio::test]
    async fn test_manager_with_exporter() {
        let mock_exporter = Arc::new(MockExporter::new());
        let manager = Manager::new().with_exporter(mock_exporter.clone());

        manager.with_local_recorder(|| {
            let counter = Counter::new("test_export_counter");
            counter.increment();
        });

        // Collect and export
        manager.collect_and_export().await;

        let exported = mock_exporter.get_exported_metrics();
        assert_eq!(exported.len(), 1);
        assert!(!exported[0].is_empty());

        let metric = exported[0].iter().find(|m| m.name == "test_export_counter");
        assert!(metric.is_some());
    }
}

#[cfg(test)]
mod helper_function_tests {
    use super::*;

    #[test]
    fn test_requests_total_counter() {
        let manager = Manager::new();

        manager.with_local_recorder(|| {
            let counter = requests_total_counter("user_service", "/api/users");
            counter.increment();
        });

        let collected = manager.collect_metrics();
        let metric = collected.iter().find(|m| m.name == "e_requests_total");
        assert!(metric.is_some());

        let m = metric.unwrap();
        assert_eq!(m.labels.len(), 2);
        assert!(m
            .labels
            .contains(&("service".to_string(), "user_service".to_string())));
        assert!(m
            .labels
            .contains(&("endpoint".to_string(), "/api/users".to_string())));
    }

    #[test]
    fn test_requests_total_counter_usage() {
        let manager = Manager::new();

        manager.with_local_recorder(|| {
            let counter = requests_total_counter("api", "/health");
            counter.increment();
            counter.increment();
        });

        let collected = manager.collect_metrics();
        let metric = collected.iter().find(|m| m.name == "e_requests_total");
        assert!(metric.is_some());

        if let Some(m) = metric {
            match m.value {
                MetricValue::Counter(value) => assert_eq!(value, 2),
                _ => panic!("Expected counter value"),
            }

            assert!(m
                .labels
                .contains(&("service".to_string(), "api".to_string())));
            assert!(m
                .labels
                .contains(&("endpoint".to_string(), "/health".to_string())));
        }
    }
}

#[cfg(test)]
mod collected_metric_tests {
    use super::*;

    #[test]
    fn test_collected_metric_new() {
        let key = Key::from_parts("test", vec![]);
        let timestamp = std::time::SystemTime::now();
        let metric = CollectedMetric {
            name: "test_metric".to_string(),
            labels: vec![("label".to_string(), "value".to_string())],
            value: MetricValue::Counter(42),
            timestamp,
            key: key.clone(),
        };

        assert_eq!(metric.name, "test_metric");
        assert_eq!(
            metric.labels,
            vec![("label".to_string(), "value".to_string())]
        );
        assert_eq!(metric.timestamp, timestamp);
        assert_eq!(metric.key, key);
        match metric.value {
            MetricValue::Counter(value) => assert_eq!(value, 42),
            _ => panic!("Expected counter value"),
        }
    }

    #[test]
    fn test_metric_value_variants() {
        let counter_value = MetricValue::Counter(100);
        let gauge_value = MetricValue::Gauge(3.21);

        match counter_value {
            MetricValue::Counter(val) => assert_eq!(val, 100),
            _ => panic!("Expected counter"),
        }

        match gauge_value {
            MetricValue::Gauge(val) => assert!((val - 3.21).abs() < f64::EPSILON),
            _ => panic!("Expected gauge"),
        }
    }
}
