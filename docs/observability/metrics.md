---
seotitle: Monitoring your backend application with custom metrics
seodesc: See how you can monitor your backend application using Encore.
title: Metrics
---

<Callout type="info">

Metrics are currently available for environments on Encore Cloud and Google Cloud Platform
created on or after January 11. Support for older environments and additional cloud providers
are launching in the next few weeks.

</Callout>

Having easy access to useful metrics is a critical part of application observability.

Encore solves this by providing automatic dashboards of all the common application-level
metrics you need, for each backend service.

Encore also makes it easy to define custom metrics for your application.
Once defined they automatically show up in the Encore metrics dashboard.

By default, Encore exports metrics data to your cloud provider's built-in monitoring service.

## Defining custom metrics

Encore applications can define custom metrics by importing
the [`encore.dev/metrics`](https://pkg.go.dev/encore.dev/metrics) package.

Then, define a new metric using one of the `metrics.NewCounter` or `metrics.NewGauge` functions.
For example, to count the number of orders processed:

```go
import "encore.dev/metrics"

var OrdersProcessed = metrics.NewCounter[uint64]("orders_processed", metrics.CounterConfig{})

func process(order *Order) {
    // ...
    OrdersProcessed.Increment()
}
```

### Metric types

Encore currently supports two metric types: counters and gauges.

Counters, like the name suggests, measure the count of something. A counter's value must always
increase, never decrease. (Note that the value gets reset to 0 when the application restarts.)
Typical use cases include counting the number of requests, the amount of data processed, and so on.

Gauges measure the current value of something. Unlike counters, a gauge's value can fluctuate up and down. Typical use
cases include measuring CPU usage, the number of active instances running of a process, and so on.

For information about their respective APIs, see the API documentation
for [Counter](https://pkg.go.dev/encore.dev/metrics#Counter) and [Gauge](https://pkg.go.dev/encore.dev/metrics#Gauge).

### Defining labels

Encore's metrics package also provides a type-safe way of attaching labels to metrics.

To do so, create a new struct type representing the labels and then use `metrics.NewCounterGroup`
or `metrics.NewGaugeGroup`:

```go
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

It's important to be aware that each combination of label values creates a unique time series
that is tracked in memory by Encore and stored by the monitoring system. This means you must
take care to only use a limited set of values to avoid a combinatorial explosion of time series,
which can result in both exorbitant costs and poor performance.

As a guiding principle, for optimal performance keep the number of unique time series to tens or hundreds at most, not
thousands.

## Integrations with monitoring services

We're working on adding integrations to external services like Grafana Cloud and Datadog. Soon you'll be able to have
metrics sent to these services instead of your cloud provider's monitoring service.