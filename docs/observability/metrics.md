---
seotitle: Monitoring your backend application with custom metrics
seodesc: See how you can monitor your backend application using Encore.
title: Metrics
subtitle: Built-in support for keeping track of key metrics
infobox: {
  title: "Metrics",
  import: "encore.dev/metrics",
}
---

Having easy access to key metrics is a critical part of application observability.
Encore solves this by providing automatic dashboards of common application-level
metrics for each service.

Encore also makes it easy to define custom metrics for your application. Once defined, custom metrics are automatically displayed on metrics page in the Cloud Dashboard.

By default, Encore also exports metrics data to your cloud provider's built-in monitoring service.

<img src="/assets/docs/metrics.png" title="Encore's metrics page"/>

## Defining custom metrics

Define custom metrics by importing the [`encore.dev/metrics`](https://pkg.go.dev/encore.dev/metrics) package and
create a new metric using one of the `metrics.NewCounter` or `metrics.NewGauge` functions.

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

Encore's metrics package provides a type-safe way of attaching labels to metrics.
To define labels, create a struct type representing the labels and then use `metrics.NewCounterGroup`
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

<Callout type="important">

Each combination of label values creates a unique time series tracked in memory and stored by the monitoring system.
Using numerous labels can lead to a combinatorial explosion, causing high cloud expenses and degraded performance.

As a general rule, limit the unique time series to tens or hundreds at most, rather than thousands.

</Callout>

## Integrations with third party observability services

To make it easy to use a third party service for monitoring, we're adding direct integrations between Encore and popular observability services. This means you can send your metrics directly to these third party services instead of your cloud provider's monitoring service.

### Grafana Cloud

To send metrics data to Grafana Cloud, you first need to Add a Grafana Cloud Stack to your application.

Open your application on [app.encore.dev](https://app.encore.dev), and click on **Settings** in the main navigation.
Then select **Grafana Cloud** in the settings menu and click on **Add Stack**.

<img width="60%" src="/assets/docs/grafanastack.png" title="Add a Grafana Stack"/>

Next, open the environment **Overview** for the environment you wish to sent metrics from and click on **Settings**.
Then in the **Grafana Cloud** section, select your Grafana Cloud Stack from the drop-down and save.

<img width="60%" src="/assets/docs/configstack.png" title="Select Grafana Stack"/>

That's it! After your next deploy, Encore will start sending metrics data to your Grafana Cloud Stack.

### Datadog

Coming soon! Reach out on [Slack](https://encore.dev/slack) if you are interested in learning more.