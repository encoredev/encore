---
seotitle: Custom metrics in TypeScript
seodesc: Learn how to define and use custom metrics in your TypeScript backend application with Encore.
title: Metrics
subtitle: Track custom metrics in your TypeScript application
infobox: {
  title: "Metrics",
  import: "encore.dev/metrics",
}
lang: ts
---

Encore provides built-in support for defining custom metrics in your TypeScript applications. Once defined, metrics are automatically collected and displayed in the Encore Cloud Dashboard, and can be exported to third-party observability services.

See the [Platform metrics documentation](/docs/platform/observability/metrics) for information about integrations with third-party services like Grafana Cloud and Datadog.

## Defining custom metrics

Define custom metrics by importing from [`encore.dev/metrics`](https://encore.dev/docs/ts/primitives/metrics) and
creating a new metric using the `Counter`, `CounterGroup`, `Gauge`, or `GaugeGroup` classes.

For example, to count the number of orders processed:

```typescript
import { Counter } from "encore.dev/metrics";

export const ordersProcessed = new Counter("orders_processed");

function process(order: Order) {
    // ...
    ordersProcessed.increment();
}
```

## Metric types

Encore currently supports two metric types: counters and gauges.

**Counters** measure the count of something. A counter's value must always increase, never decrease. (Note that the value gets reset to 0 when the application restarts.) Typical use cases include counting the number of requests, the amount of data processed, and so on.

**Gauges** measure the current value of something. Unlike counters, a gauge's value can fluctuate up and down. Typical use cases include measuring CPU usage, the number of active instances running of a process, and so on.

### Counter example

```typescript
import { Counter } from "encore.dev/metrics";

export const ordersProcessed = new Counter("orders_processed");

function processOrder() {
    ordersProcessed.increment();
    // ... process order
}
```

You can also increment by a specific value instead of 1:

```typescript
import { Counter } from "encore.dev/metrics";

export const bytesProcessed = new Counter("bytes_processed");

function processData(data: Buffer) {
    bytesProcessed.increment(data.length);
    // ... process data
}
```

### Gauge example

```typescript
import { Gauge } from "encore.dev/metrics";

export const cpuUsage = new Gauge("cpu_usage");

function updateMetrics() {
    const usage = getCpuUsage(); // returns a number between 0-100
    cpuUsage.set(usage);
}
```

Another example tracking active connections:

```typescript
import { Gauge } from "encore.dev/metrics";

let activeCount = 0;
export const activeConnections = new Gauge("active_connections");

function onConnect() {
    activeCount++;
    activeConnections.set(activeCount);
}

function onDisconnect() {
    activeCount--;
    activeConnections.set(activeCount);
}
```

## Defining labels

Encore's metrics package provides a type-safe way of attaching labels to metrics. To define labels, create an interface type representing the labels and then use `CounterGroup` or `GaugeGroup`.

The labels interface defines the structure of labels, where each property corresponds to a single label. Each property must be of type `string`, `number`, or `boolean`.

<Callout type="info">

When using `number` type for labels, the value will be converted to an integer using `Math.floor()`.

</Callout>

### Counter with labels

```typescript
import { CounterGroup } from "encore.dev/metrics";

interface Labels {
    success: boolean;
}

export const ordersProcessed = new CounterGroup<Labels>("orders_processed");

function process(order: Order) {
    let success = false;
    try {
        // ... process order
        success = true;
    } catch (err) {
        success = false;
    }
    ordersProcessed.with({ success }).increment();
}
```

### Gauge with labels

```typescript
import { GaugeGroup } from "encore.dev/metrics";

interface ConnectionLabels {
    region: string;
}

export const activeConnectionsByRegion = new GaugeGroup<ConnectionLabels>("active_connections");
const connectionCounts = new Map<string, number>();

function onConnect(region: string) {
    const count = (connectionCounts.get(region) || 0) + 1;
    connectionCounts.set(region, count);
    activeConnectionsByRegion.with({ region }).set(count);
}

function onDisconnect(region: string) {
    const count = Math.max(0, (connectionCounts.get(region) || 0) - 1);
    connectionCounts.set(region, count);
    activeConnectionsByRegion.with({ region }).set(count);
}
```

### Multiple labels

You can define multiple labels for a metric:

```typescript
import { CounterGroup } from "encore.dev/metrics";

interface JobLabels {
    jobType: string;
    priority: number;
    success: boolean;
}

export const jobsProcessed = new CounterGroup<JobLabels>("jobs_processed");

function processJob(jobType: string, priority: number) {
    try {
        // ... process job
        jobsProcessed.with({ jobType, priority, success: true }).increment();
    } catch (err) {
        jobsProcessed.with({ jobType, priority, success: false }).increment();
    }
}
```

## Metric references

Encore uses static analysis to determine which services are using each metric, and what operations each service is performing.

This means metric objects can't be passed around however you like, as it makes static analysis impossible in many cases. To simplify your workflow, given these restrictions, Encore supports defining a "reference" to a metric that can be passed around any way you want.

To create a reference, call the `.ref()` method on any metric:

```typescript
import { Counter } from "encore.dev/metrics";

export const ordersProcessed = new Counter("orders_processed");

// Create a reference that can be passed around
const metricRef = ordersProcessed.ref();

// Pass the reference to other functions
function logMetric(metric: Counter) {
    metric.increment();
}

logMetric(metricRef);
```

This works for all metric types (`Counter`, `CounterGroup`, `Gauge`, and `GaugeGroup`).

<Callout type="important">

Each combination of label values creates a unique time series tracked in memory and stored by the monitoring system.
Using numerous labels can lead to a combinatorial explosion, causing high cloud expenses and degraded performance.

As a general rule, limit the unique time series to tens or hundreds at most, rather than thousands.

</Callout>
