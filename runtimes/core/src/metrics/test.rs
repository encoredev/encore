use super::*;
use std::sync::Arc;

#[cfg(test)]
mod counter_tests {
    use super::*;

    #[test]
    fn test_counter_with_labels() {
        let manager = Manager::new();
        let registry = manager.registry();

        registry
            .get_or_create_counter::<u64>(
                "test_counter_with_labels",
                [("service", "test_service"), ("method", "GET")],
            )
            .increment();

        let collected = manager.collect_metrics();
        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_counter_with_labels");
        assert!(metric.is_some());

        let m = metric.unwrap();
        assert_eq!(m.key.labels().len(), 2);

        let labels: Vec<_> = m.key.labels().map(|l| (l.key(), l.value())).collect();

        assert!(labels.contains(&("service", "test_service")));
        assert!(labels.contains(&("method", "GET")));

        // Verify it's the correct counter type
        if let MetricValue::CounterU64(value) = m.value {
            assert_eq!(value, 1);
        } else {
            panic!("Expected CounterU64, got {:?}", m.value);
        }
    }

    #[test]
    fn test_counter_increment() {
        let manager = Manager::new();
        let registry = manager.registry();

        registry
            .get_or_create_counter::<u64>("test_increment", [])
            .increment();

        let collected = manager.collect_metrics();
        let metric = collected.iter().find(|m| m.key.name() == "test_increment");
        assert!(metric.is_some());

        let m = metric.unwrap();
        if let MetricValue::CounterU64(value) = m.value {
            assert_eq!(value, 1);
        } else {
            panic!("Expected CounterU64, got {:?}", m.value);
        }
    }
}

#[cfg(test)]
mod registry_tests {
    use super::*;

    #[test]
    fn test_registry_collect_counters() {
        let registry = Arc::new(Registry::new());

        let counter = registry.get_or_create_counter::<u64>("test_counter", [("label", "value")]);
        counter.increment();
        counter.increment();
        counter.increment();
        counter.increment();
        counter.increment();

        let collected = registry.collect();

        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_counter")
            .expect("test_counter not found");
        assert_eq!(metric.key.name(), "test_counter");
        assert_eq!(metric.key.labels().len(), 1);
        match metric.value {
            MetricValue::CounterU64(value) => assert_eq!(value, 5),
            _ => panic!("Expected CounterU64 value, got {:?}", metric.value),
        }
    }

    #[test]
    fn test_registry_collect_gauges() {
        let registry = Arc::new(Registry::new());

        registry
            .get_or_create_gauge::<f64>("test_gauge", [])
            .set(42.5);

        let collected = registry.collect();

        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_gauge")
            .expect("test_gauge not found");
        assert_eq!(metric.key.name(), "test_gauge");
        match metric.value {
            MetricValue::GaugeF64(value) => assert!((value - 42.5).abs() < f64::EPSILON),
            _ => panic!("Expected GaugeF64 value, got {:?}", metric.value),
        }
    }

    #[test]
    fn test_registry_registered_at() {
        let registry = Arc::new(Registry::new());

        let before = std::time::SystemTime::now();
        registry.get_or_create_counter::<u64>("test_metric", []);
        let after = std::time::SystemTime::now();

        // Find the key that was created
        let collected = registry.collect();
        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_metric")
            .unwrap();

        let time = metric.registered_at;
        assert!(time >= before && time <= after);

        // Creating the counter again should not update first seen
        registry.get_or_create_counter::<u64>("test_metric", []);
        // Find the key that was created
        let collected = registry.collect();
        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_metric")
            .unwrap();
        let time2 = metric.registered_at;

        assert_eq!(time, time2)
    }

    #[test]
    fn test_registry_always_collects_system_metrics() {
        let registry = Arc::new(Registry::new());

        let collected = registry.collect();

        // Check that system metrics are present
        let has_memory_metrics = collected
            .iter()
            .any(|m| m.key.name() == "e_sys_memory_used_bytes");

        assert!(has_memory_metrics, "Memory metrics should be present");
    }
}

#[cfg(test)]
mod gauge_tests {
    use super::*;

    #[test]
    fn test_gauge_with_labels() {
        let manager = Manager::new();

        let registry = manager.registry();
        // Create gauge with labels using new API
        registry
            .get_or_create_gauge::<f64>(
                "test_gauge_with_labels",
                [("service", "test_service"), ("region", "us-west")],
            )
            .set(42.5);

        let collected = manager.collect_metrics();
        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_gauge_with_labels");
        assert!(metric.is_some());

        let m = metric.unwrap();
        assert_eq!(m.key.labels().len(), 2);
        assert!(m
            .key
            .labels()
            .any(|label| label.key() == "service" && label.value() == "test_service"));
        assert!(m
            .key
            .labels()
            .any(|label| label.key() == "region" && label.value() == "us-west"));

        match m.value {
            MetricValue::GaugeF64(value) => assert!((value - 42.5).abs() < f64::EPSILON),
            _ => panic!("Expected gauge value"),
        }
    }
}

#[cfg(test)]
mod manager_tests {
    use super::*;

    #[test]
    fn test_manager_collect_metrics() {
        let manager = Manager::new();

        manager
            .registry()
            .get_or_create_counter::<u64>("test_manager_counter", [])
            .increment();

        let collected = manager.collect_metrics();
        assert!(!collected.is_empty());

        let metric = collected
            .iter()
            .find(|m| m.key.name() == "test_manager_counter");
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
    impl super::super::exporter::Exporter for MockExporter {
        async fn export(&self, metrics: Vec<CollectedMetric>) {
            self.exported_metrics.lock().unwrap().push(metrics);
        }
    }

    #[tokio::test]
    async fn test_manager_with_exporter() {
        let mock_exporter = Arc::new(MockExporter::new());
        let manager = Manager::new().with_exporter(mock_exporter.clone());

        let registry = manager.registry();
        registry
            .get_or_create_counter::<u64>("test_export_counter", [])
            .increment();

        // Collect and export
        manager.collect_and_export().await;

        let exported = mock_exporter.get_exported_metrics();
        assert_eq!(exported.len(), 1);
        assert!(!exported[0].is_empty());

        let metric = exported[0]
            .iter()
            .find(|m| m.key.name() == "test_export_counter");
        assert!(metric.is_some());
    }
}

#[cfg(test)]
mod collected_metric_tests {
    use super::*;

    #[test]
    fn test_gauge_types_integer_and_float() {
        let manager = Manager::new();

        let registry = manager.registry();

        // Test float gauge
        let float_gauge: Gauge<f64> = registry.get_or_create_gauge("test_float_metric", []);
        float_gauge.set(42.5);

        let float_gauge_temp: Gauge<f64> =
            registry.get_or_create_gauge("test_float_metric", [("type", "temperature")]);
        float_gauge_temp.set(99.9);

        // Test integer gauge
        let int_gauge: Gauge<i64> = registry.get_or_create_gauge("test_int_metric", []);
        int_gauge.set(12345);

        let int_gauge_count: Gauge<i64> =
            registry.get_or_create_gauge("test_int_metric", [("type", "count")]);
        int_gauge_count.set(67890);

        // Test large integer to verify precision
        let large_int = i64::MAX;
        let int_gauge_large: Gauge<i64> =
            registry.get_or_create_gauge("test_int_metric", [("type", "large")]);
        int_gauge_large.set(large_int);

        // Test small negative integer to verify precision
        let small_int = i64::MIN;
        let int_gauge_small: Gauge<i64> =
            registry.get_or_create_gauge("test_int_metric", [("type", "small")]);
        int_gauge_small.set(small_int);

        let metrics = manager.collect_metrics();

        let float_metrics: Vec<_> = metrics
            .iter()
            .filter(|m| m.key.name() == "test_float_metric")
            .collect();
        let int_metrics: Vec<_> = metrics
            .iter()
            .filter(|m| m.key.name() == "test_int_metric")
            .collect();

        // Should have 2 float metrics and 4 int metrics
        assert_eq!(float_metrics.len(), 2);
        assert_eq!(int_metrics.len(), 4);

        // Check float metrics with their labels
        for m in &float_metrics {
            if let MetricValue::GaugeF64(val) = m.value {
                if m.key.labels().count() == 0 {
                    // No labels - should be 42.5
                    assert_eq!(val, 42.5);
                } else {
                    // Should have type=temperature label
                    let has_temp_label = m
                        .key
                        .labels()
                        .any(|l| l.key() == "type" && l.value() == "temperature");
                    assert!(has_temp_label);
                    assert_eq!(val, 99.9);
                }
            } else {
                panic!("Float metric has wrong type: {:?}", m.value);
            }
        }

        // Check int metrics with their labels
        for m in &int_metrics {
            if let MetricValue::GaugeI64(val) = m.value {
                if m.key.labels().count() == 0 {
                    // No labels - should be 12345
                    assert_eq!(val, 12345);
                } else {
                    // Check which label it has
                    let label_value = m
                        .key
                        .labels()
                        .find(|l| l.key() == "type")
                        .map(|l| l.value())
                        .expect("Should have 'type' label");

                    match label_value {
                        "count" => assert_eq!(val, 67890),
                        "large" => assert_eq!(val, i64::MAX),
                        "small" => assert_eq!(val, i64::MIN),
                        _ => panic!("Unexpected label value: {}", label_value),
                    }
                }
            } else {
                panic!("Integer metric has wrong type: {:?}", m.value);
            }
        }
    }

    #[test]
    fn test_counter_types_integer() {
        let manager = Manager::new();
        let registry = manager.registry();

        // Test u64 counter
        let u64_counter: Counter<u64> = registry.get_or_create_counter("test_u64_counter", []);
        u64_counter.increment();
        u64_counter.increment();
        u64_counter.increment();

        let u64_counter_labeled: Counter<u64> =
            registry.get_or_create_counter("test_u64_counter", [("type", "requests")]);
        u64_counter_labeled.increment();
        u64_counter_labeled.increment();

        // Test i64 counter
        let i64_counter = registry.get_or_create_counter::<i64>("test_i64_counter", []);
        i64_counter.increment();
        i64_counter.increment();
        i64_counter.increment();
        i64_counter.increment();

        let i64_counter_labeled: Counter<i64> =
            registry.get_or_create_counter("test_i64_counter", [("type", "errors")]);
        i64_counter_labeled.increment();

        // Collect metrics
        let metrics = manager.collect_metrics();

        let u64_metrics: Vec<_> = metrics
            .iter()
            .filter(|m| m.key.name() == "test_u64_counter")
            .collect();
        let i64_metrics: Vec<_> = metrics
            .iter()
            .filter(|m| m.key.name() == "test_i64_counter")
            .collect();

        // Should have 2 u64 metrics and 2 i64 metrics
        assert_eq!(u64_metrics.len(), 2);
        assert_eq!(i64_metrics.len(), 2);

        // Check u64 counter metrics
        for m in &u64_metrics {
            if let MetricValue::CounterU64(val) = m.value {
                if m.key.labels().count() == 0 {
                    // No labels - should be 3
                    assert_eq!(val, 3);
                } else {
                    // Should have type=requests label
                    let has_requests_label = m
                        .key
                        .labels()
                        .any(|l| l.key() == "type" && l.value() == "requests");
                    assert!(has_requests_label);
                    assert_eq!(val, 2);
                }
            } else {
                panic!("u64 counter metric has wrong type: {:?}", m.value);
            }
        }

        // Check i64 counter metrics
        for m in &i64_metrics {
            if let MetricValue::CounterI64(val) = m.value {
                if m.key.labels().count() == 0 {
                    // No labels - should be 4
                    assert_eq!(val, 4);
                } else {
                    // Should have type=errors label
                    let has_errors_label = m
                        .key
                        .labels()
                        .any(|l| l.key() == "type" && l.value() == "errors");
                    assert!(has_errors_label);
                    assert_eq!(val, 1);
                }
            } else {
                panic!("i64 counter metric has wrong type: {:?}", m.value);
            }
        }
    }
}

#[cfg(test)]
mod concurrency_tests {
    use super::*;
    use std::sync::{Arc, Barrier};
    use std::thread;

    #[test]
    fn test_counter_concurrent_increments() {
        let manager = Manager::new();
        let registry = manager.registry();

        let num_threads = 10;
        let increments_per_thread = 1000;
        let barrier = Arc::new(Barrier::new(num_threads));

        let handles: Vec<_> = (0..num_threads)
            .map(|_| {
                let counter = registry.get_or_create_counter::<u64>("concurrent_counter", []);
                let barrier = barrier.clone();
                thread::spawn(move || {
                    barrier.wait();
                    for _ in 0..increments_per_thread {
                        counter.increment();
                    }
                })
            })
            .collect();

        for handle in handles {
            handle.join().unwrap();
        }

        let collected = manager.collect_metrics();
        let metric = collected
            .iter()
            .find(|m| m.key.name() == "concurrent_counter")
            .unwrap();

        if let MetricValue::CounterU64(value) = metric.value {
            assert_eq!(value, (num_threads * increments_per_thread) as u64);
        } else {
            panic!("Expected CounterU64");
        }
    }

    #[test]
    fn test_gauge_concurrent_operations() {
        let manager = Manager::new();
        let registry = manager.registry();
        let num_threads = 8;
        let operations_per_thread = 100;

        let handles: Vec<_> = (0..num_threads)
            .map(|thread_id| {
                let registry = Arc::clone(registry);
                thread::spawn(move || {
                    for i in 0..operations_per_thread {
                        let thread_id_str = thread_id.to_string();
                        let gauge = registry.get_or_create_gauge::<f64>(
                            "concurrent_gauge",
                            [("thread", thread_id_str.as_str())],
                        );
                        gauge.set((thread_id * operations_per_thread + i) as f64);
                    }
                })
            })
            .collect();

        for handle in handles {
            handle.join().unwrap();
        }

        let collected = manager.collect_metrics();
        let gauge_metrics: Vec<_> = collected
            .iter()
            .filter(|m| m.key.name() == "concurrent_gauge")
            .collect();

        assert_eq!(gauge_metrics.len(), num_threads);

        for metric in gauge_metrics {
            if let MetricValue::GaugeF64(value) = metric.value {
                let thread_id: usize = metric
                    .key
                    .labels()
                    .find(|l| l.key() == "thread")
                    .expect("no thread label set on metric")
                    .value()
                    .parse()
                    .expect("couldn't parse thread id");

                assert_eq!(
                    value,
                    (thread_id * operations_per_thread + operations_per_thread - 1) as f64 // last value written in the loop for each thread
                );
            } else {
                panic!("Expected GaugeF64");
            }
        }
    }

    #[test]
    fn test_registry_concurrent_metric_creation() {
        let registry = Arc::new(Registry::new());
        let num_threads = 16;
        let metrics_per_thread = 50;

        let handles: Vec<_> = (0..num_threads)
            .map(|thread_id| {
                let registry = Arc::clone(&registry);
                thread::spawn(move || {
                    for i in 0..metrics_per_thread {
                        let metric_name = format!("metric_{}_{}", thread_id, i);
                        registry
                            .get_or_create_counter::<u64>(&metric_name, [])
                            .increment();
                        registry
                            .get_or_create_gauge::<f64>(&metric_name, [("type", "gauge")])
                            .set(i as f64);
                    }
                })
            })
            .collect();

        for handle in handles {
            handle.join().unwrap();
        }

        let collected = registry.collect();

        let counters = collected
            .iter()
            .filter(|m| matches!(m.value, MetricValue::CounterU64(_)))
            .count();
        let gauges = collected
            .iter()
            .filter(|m| matches!(m.value, MetricValue::GaugeF64(_)))
            .count();

        // Account for system metrics that are always collected
        assert!(counters >= num_threads * metrics_per_thread);
        assert!(gauges >= num_threads * metrics_per_thread);
    }

    #[test]
    fn test_concurrent_collection_and_updates() {
        let manager = Manager::new();
        let registry = manager.registry();

        let registry_clone = Arc::clone(registry);
        let manager_clone = manager.clone();

        let updater_handle = thread::spawn(move || {
            let counter = registry_clone.get_or_create_counter::<u64>("collection_test", []);
            let gauge = registry_clone.get_or_create_gauge::<f64>("collection_test_gauge", []);
            for i in 0..1000 {
                counter.increment();
                gauge.set(i as f64);
                thread::sleep(std::time::Duration::from_micros(1));
            }
        });

        let collector_handle = thread::spawn(move || {
            for _ in 0..100 {
                let _metrics = manager_clone.collect_metrics();
                thread::sleep(std::time::Duration::from_micros(10));
            }
        });

        updater_handle.join().unwrap();
        collector_handle.join().unwrap();

        let final_metrics = manager.collect_metrics();
        assert!(!final_metrics.is_empty());
    }
}

#[cfg(test)]
mod memory_performance_tests {
    use super::*;

    #[test]
    fn test_high_cardinality_metrics() {
        let manager = Manager::new();
        let registry = manager.registry();

        let num_services = 20;
        let num_endpoints = 50;
        let num_status_codes = 10;

        for service_id in 0..num_services {
            for endpoint_id in 0..num_endpoints {
                for status_code in 200..(200 + num_status_codes) {
                    let service_str = format!("service_{}", service_id);
                    let endpoint_str = format!("endpoint_{}", endpoint_id);
                    let status_str = status_code.to_string();
                    let counter = registry.get_or_create_counter::<u64>(
                        "high_cardinality_requests",
                        [
                            ("service", service_str.as_str()),
                            ("endpoint", endpoint_str.as_str()),
                            ("status", status_str.as_str()),
                        ],
                    );
                    counter.increment();
                }
            }
        }

        let collected = manager.collect_metrics();
        let request_metrics: Vec<_> = collected
            .iter()
            .filter(|m| m.key.name() == "high_cardinality_requests")
            .collect();

        let expected_count = num_services * num_endpoints * num_status_codes;
        assert_eq!(request_metrics.len(), expected_count);

        for metric in request_metrics {
            assert_eq!(metric.key.labels().count(), 3);
            if let MetricValue::CounterU64(value) = metric.value {
                assert_eq!(value, 1);
            }
        }
    }
}

#[cfg(test)]
mod manager_export_tests {
    use super::*;
    use std::sync::Arc;
    use std::time::Duration;

    #[tokio::test]
    async fn test_manager_without_exporter() {
        let manager = Manager::new();
        let registry = manager.registry();

        registry
            .get_or_create_counter::<u64>("no_exporter_test", [])
            .increment();

        manager.collect_and_export().await;

        let metrics = manager.collect_metrics();
        assert!(!metrics.is_empty());

        let found = metrics.iter().any(|m| m.key.name() == "no_exporter_test");
        assert!(found);
    }

    #[tokio::test]
    async fn test_manager_concurrent_collect_and_export() {
        let manager = Arc::new(Manager::new());
        let registry = manager.registry();

        let manager1 = manager.clone();
        let manager2 = manager.clone();
        let registry_clone = Arc::clone(registry);

        let updater = tokio::spawn(async move {
            let counter = registry_clone.get_or_create_counter::<u64>("concurrent_export_test", []);
            for _ in 0..100 {
                counter.increment();
                tokio::time::sleep(Duration::from_millis(1)).await;
            }
        });

        let collector1 = tokio::spawn(async move {
            for _ in 0..10 {
                manager1.collect_and_export().await;
                tokio::time::sleep(Duration::from_millis(5)).await;
            }
        });

        let collector2 = tokio::spawn(async move {
            for _ in 0..10 {
                let _metrics = manager2.collect_metrics();
                tokio::time::sleep(Duration::from_millis(3)).await;
            }
        });

        let _ = tokio::try_join!(updater, collector1, collector2);

        let final_metrics = manager.collect_metrics();
        let test_metric = final_metrics
            .iter()
            .find(|m| m.key.name() == "concurrent_export_test")
            .unwrap();

        if let MetricValue::CounterU64(value) = test_metric.value {
            assert_eq!(value, 100);
        }
    }
}

#[cfg(test)]
mod type_system_atomic_tests {
    use malachite::base::num::basic::floats::PrimitiveFloat;

    use super::*;

    #[test]
    fn test_float_precision_in_atomic_operations() {
        let registry = Arc::new(Registry::new());

        let test_values = [
            0.0,
            -0.0,
            1.0,
            -1.0,
            f64::MIN,
            f64::MAX,
            f64::EPSILON,
            std::f64::consts::PI,
            std::f64::consts::E,
            1.23456789012345,
            -9.87654321098765,
        ];

        for (i, &value) in test_values.iter().enumerate() {
            let gauge = registry.get_or_create_gauge::<f64>(&format!("precision_test_{}", i), []);
            gauge.set(value);

            let collected = registry.collect();
            let metric = collected
                .iter()
                .find(|m| m.key.name() == format!("precision_test_{}", i))
                .unwrap();

            if let MetricValue::GaugeF64(stored_value) = metric.value {
                assert_eq!(stored_value, value, "Precision lost for value: {}", value);
            } else {
                panic!("Expected GaugeF64, got {:?}", metric.value);
            }
        }
    }

    #[test]
    fn test_special_float_values() {
        let registry = Arc::new(Registry::new());

        let special_values = [f64::NAN, f64::INFINITY, f64::NEG_INFINITY, 0.0, -0.0];

        for (i, &value) in special_values.iter().enumerate() {
            let gauge = registry.get_or_create_gauge::<f64>(&format!("special_float_{}", i), []);
            gauge.set(value);

            let collected = registry.collect();
            let metric = collected
                .iter()
                .find(|m| m.key.name() == format!("special_float_{}", i))
                .unwrap();

            if let MetricValue::GaugeF64(stored_value) = metric.value {
                if value.is_nan() {
                    assert!(stored_value.is_nan());
                } else if value.is_infinite() {
                    assert!(stored_value.is_infinite());
                    assert_eq!(stored_value.is_sign_positive(), value.is_sign_positive());
                } else if value.is_negative_zero() {
                    assert!(stored_value.is_negative_zero())
                } else {
                    assert_eq!(stored_value.to_bits(), value.to_bits());
                }
            }
        }
    }

    #[test]
    fn test_counter_ops_trait_consistency() {
        use crate::metrics::CounterOps;
        use std::sync::atomic::{AtomicU64, Ordering};

        let atomic = Arc::new(AtomicU64::new(0));

        atomic.increment(5u64);
        assert_eq!(atomic.load(Ordering::Acquire), 5);

        atomic.increment(10i64);
        assert_eq!(atomic.load(Ordering::Acquire), 15);

        if let MetricValue::CounterU64(value) = CounterOps::<u64>::get(&atomic) {
            assert_eq!(value, 15);
        }

        if let MetricValue::CounterI64(value) = CounterOps::<i64>::get(&atomic) {
            assert_eq!(value, 15);
        }
    }

    #[test]
    fn test_gauge_ops_trait_consistency() {
        use crate::metrics::GaugeOps;
        use std::sync::atomic::AtomicU64;

        let atomic = Arc::new(AtomicU64::new(0));

        atomic.set(42u64);
        if let MetricValue::GaugeU64(value) = GaugeOps::<u64>::get(&atomic) {
            assert_eq!(value, 42);
        }

        atomic.set(-42i64);
        if let MetricValue::GaugeI64(value) = GaugeOps::<i64>::get(&atomic) {
            assert_eq!(value, -42);
        }

        atomic.set(3.21f64);
        if let MetricValue::GaugeF64(value) = GaugeOps::<f64>::get(&atomic) {
            assert!((value - 3.21).abs() < f64::EPSILON);
        }
    }
}
