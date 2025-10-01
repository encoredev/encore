#[cfg(test)]
mod tests {
    use crate::metrics::{Counter, Manager, RequestTotal};

    #[test]
    fn test_counter_increment_and_collect() {
        // Create the metrics manager (this will set up the global recorder)
        let manager = Manager::new();

        // Create a counter for requests_total
        let labels = RequestTotal {
            endpoint: "test_endpoint".to_string(),
            code: "200".to_string(),
        };

        let counter = Counter::new(labels);

        // Increment the counter a few times
        counter.increment_by(1u64);
        counter.increment_by(1u64);
        counter.increment_by(3u64);

        // Wait a moment for metrics to be registered
        std::thread::sleep(std::time::Duration::from_millis(100));

        // Collect metrics
        let metrics = manager.collect_metrics();

        println!("Found {} metrics:", metrics.len());
        for metric in &metrics {
            println!(
                "  Metric: {} = {} (labels: {:?})",
                metric.info.name, metric.value, metric.labels
            );
        }

        // We should have at least one metric collected
        assert!(
            !metrics.is_empty(),
            "Should have collected at least one metric, got {}",
            metrics.len()
        );

        // Find our requests_total metric
        let requests_total_metric = metrics
            .iter()
            .find(|m| m.info.name == "e_requests_total")
            .expect("Should find requests_total metric");

        // The value should be 5 (1 + 1 + 3)
        assert_eq!(
            requests_total_metric.value, 5,
            "Expected counter value to be 5, got {}",
            requests_total_metric.value
        );

        // Check that labels are present
        assert!(requests_total_metric
            .labels
            .iter()
            .any(|(k, v)| k == "endpoint" && v == "test_endpoint"));
        assert!(requests_total_metric
            .labels
            .iter()
            .any(|(k, v)| k == "code" && v == "200"));
    }
}
