/**
 * Example usage of the custom metrics API
 *
 * This file demonstrates how to use counters and gauges following the Go-style API.
 */

import { Counter, CounterGroup, Gauge, GaugeGroup } from "./mod";

// Example 1: Simple counter without labels
const OrdersProcessed = new Counter<number>("orders_processed", {});

// Example 2: Counter with labels
interface RequestLabels extends Record<string, string | number | boolean> {
  success: boolean;
  method: string;
}

const RequestsTotal = new CounterGroup<RequestLabels, number>("http_requests_total", {});

// Example 3: Simple gauge
const MemoryUsage = new Gauge<number>("memory_usage_bytes", {});

// Example 4: Gauge with labels
interface CPULabels extends Record<string, string | number | boolean> {
  core: number;
}

const CPUUsage = new GaugeGroup<CPULabels, number>("cpu_usage_percent", {});

// Simulating request handling
function handleRequest(method: string, success: boolean) {
  // Simple counter
  OrdersProcessed.increment();

  // Counter with labels
  // First time: allocates slot (~1μs)
  // Subsequent: pure atomic operation (~10ns)
  RequestsTotal.with({ success, method }).increment();
}

// Simulating system monitoring
function updateSystemMetrics() {
  // Simple gauge
  MemoryUsage.set(process.memoryUsage().heapUsed);

  // Gauge with labels for each CPU core
  for (let core = 0; core < 4; core++) {
    const usage = Math.random() * 100;
    CPUUsage.with({ core }).set(usage);
  }
}

// Example usage
console.log('=== Metrics Example (Go-style API) ===\n');

// Simulate some requests
console.log('Simulating HTTP requests...');
handleRequest('GET', true);
handleRequest('GET', true);
handleRequest('POST', true);
handleRequest('GET', false);
handleRequest('POST', false);

// Update system metrics
console.log('Updating system metrics...');
updateSystemMetrics();

// Read back values (for demonstration)
console.log('\nMetric values:');
console.log('- Orders processed:', OrdersProcessed.get());
console.log('- GET requests (success):', RequestsTotal.with({ success: true, method: 'GET' }).get());
console.log('- Memory usage:', MemoryUsage.get(), 'bytes');
console.log('- CPU 0 usage:', CPUUsage.with({ core: 0 }).get(), '%');

console.log('\nDone! These metrics will be automatically collected and exported by Encore.');
