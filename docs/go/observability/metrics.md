---
seotitle: Custom metrics in Go
seodesc: Learn how to define and use custom metrics in your Go backend application with Encore.
title: Metrics
subtitle: Track custom metrics in your Go application
infobox: {
  title: "Metrics",
  import: "encore.dev/metrics",
}
lang: go
---

Encore provides built-in support for defining custom metrics in your Go applications. Once defined, metrics are automatically collected and displayed in the Encore Cloud Dashboard, and can be exported to third-party observability services.

See the [Platform metrics documentation](/docs/platform/observability/metrics) for information about integrations with third-party services like Grafana Cloud and Datadog.

## Defining custom metrics

Define custom metrics by importing the [`encore.dev/metrics`](https://pkg.go.dev/encore.dev/metrics) package and
creating a new metric using one of the `metrics.NewCounter` or `metrics.NewGauge` functions.

For example, to count the number of orders processed:

```go
import "encore.dev/metrics"

var OrdersProcessed = metrics.NewCounter[uint64]("orders_processed", metrics.CounterConfig{})

func process(order *Order) {
    // ...
    OrdersProcessed.Increment()
}
```

## Metric types

Encore currently supports two metric types: counters and gauges.

**Counters** measure the count of something. A counter's value must always increase, never decrease. (Note that the value gets reset to 0 when the application restarts.) Typical use cases include counting the number of requests, the amount of data processed, and so on.

**Gauges** measure the current value of something. Unlike counters, a gauge's value can fluctuate up and down. Typical use cases include measuring CPU usage, the number of active instances running of a process, and so on.

For information about their respective APIs, see the API documentation for [Counter](https://pkg.go.dev/encore.dev/metrics#Counter) and [Gauge](https://pkg.go.dev/encore.dev/metrics#Gauge).

### Counter example

```go
import "encore.dev/metrics"

var RequestsReceived = metrics.NewCounter[uint64]("requests_received", metrics.CounterConfig{})

func handleRequest() {
    RequestsReceived.Increment()
    // ... handle request
}
```

### Gauge example

```go
import "encore.dev/metrics"

var ActiveConnections = metrics.NewGauge[int64]("active_connections", metrics.GaugeConfig{})

func onConnect() {
    ActiveConnections.Add(1)
}

func onDisconnect() {
    ActiveConnections.Add(-1)
}
```

## Defining labels

Encore's metrics package provides a type-safe way of attaching labels to metrics. To define labels, create a struct type representing the labels and then use `metrics.NewCounterGroup` or `metrics.NewGaugeGroup`.

The Labels type must be a named struct, where each field corresponds to a single label. Each field must be of type `string`, `int`, or `bool`.

### Counter with labels

```go
import "encore.dev/metrics"

type Labels struct {
    Success bool
}

var OrdersProcessed = metrics.NewCounterGroup[Labels, uint64]("orders_processed", metrics.CounterConfig{})

func process(order *Order) {
    var success bool
    // ... populate success with true/false ...
    OrdersProcessed.With(Labels{Success: success}).Increment()
}
```

### Gauge with labels

```go
import "encore.dev/metrics"

type ConnectionLabels struct {
    Region string
}

var ActiveConnections = metrics.NewGaugeGroup[ConnectionLabels, int64]("active_connections", metrics.GaugeConfig{})

func onConnect(region string) {
    ActiveConnections.With(ConnectionLabels{Region: region}).Add(1)
}
```

<Callout type="important">

Each combination of label values creates a unique time series tracked in memory and stored by the monitoring system.
Using numerous labels can lead to a combinatorial explosion, causing high cloud expenses and degraded performance.

As a general rule, limit the unique time series to tens or hundreds at most, rather than thousands.

</Callout>
