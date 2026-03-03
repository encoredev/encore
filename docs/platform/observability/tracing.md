---
seotitle: Distributed Tracing helps you understand your app
seodesc: See how to use distributed tracing in your backend application, across multiple services, using Encore.
title: Distributed Tracing
subtitle: Track requests across your application and infrastructure
lang: platform
---

Distributed systems often have many moving parts, making it difficult to understand what your code is doing and finding the root-cause to bugs. That’s where Tracing comes in. If you haven’t seen it before, it may just about change your life.

Tracing is a revolutionary way to gain insight into what your applications are doing. It works by capturing the series of events as they occur during the execution of your code (a “trace”). This works by propagating a trace id between all individual systems, then correlating and joining the information together to present a unified picture of what happened end-to-end.

As opposed to the labor intensive instrumentation you'd normally need to go through to use tracing, Encore automatically captures traces for your entire application – in all environments. Uniquely, this means you can use tracing even for local development to help debugging and speed up iterations.

You view traces in the [Local Development Dashboard](/docs/ts/observability/dev-dash) and in the [Encore Cloud dashboard](https://app.encore.cloud) for Production and other environments.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/tracingvideo.mp4" className="w-full h-full" type="video/mp4" />
</video>

## Encore's tracing is more comprehensive and more performant than all other tools

Unlike other tracing solutions, Encore understands what each trace event is and captures unique insights about each one. This means you get access to more information than ever before:

* Stack traces
* Structured logging
* HTTP requests
* Network connection information
* API calls
* Database queries
* etc.

## Redacting sensitive data

Encore's tracing automatically captures request and response payloads to simplify debugging.

For cases where this is undesirable, such as for passwords or personally identifiable information (PII), Encore supports redacting fields marked as containing sensitive data.

See the documentation on [API Schemas](/docs/ts/primitives/defining-apis#sensitive-data) for more information.

## Trace Sampling

Trace sampling lets you control what percentage of traces are recorded and stored. You can configure sampling rates per environment, service, and endpoint, giving you fine-grained control over your tracing volume.

### How sampling works

Sampling is determined at the root of the trace. This means if you set an endpoint to sample at 10%, it controls whether a trace is created when that endpoint is called as the initial entry point. If that same endpoint is called as part of an already-ongoing trace (e.g. as an internal service-to-service call), it will always be included in the existing trace regardless of its own sampling rate.

This design ensures that all traces are complete — you'll never see partial traces with missing spans. Either a trace is sampled in its entirety, or not at all.

### Configuring sampling rates

You can configure sampling rates in the Encore Cloud dashboard. Sampling can be set at three levels of granularity:

- **Environment level**: Set a default sampling rate for all traces in an environment.
- **Service level**: Override the environment default for a specific service.
- **Endpoint level**: Override the service default for a specific endpoint.

More specific settings take precedence. For example, if your environment is set to sample 100% of traces but a high-traffic endpoint is set to 10%, that endpoint will only generate new traces 10% of the time it's called as the root of a request.

## Trace Budgets

Trace budgets give you full predictability over your tracing costs by letting you set spending limits on a daily and monthly basis. When a budget limit is reached, tracing is paused until the next period begins, ensuring you never receive unexpected charges.

### Included events

Encore Cloud includes a generous amount of tracing events in each plan:

- **Free tier**: 1M trace events per month included.
- **Pro tier**: 20M trace events per month included.

Beyond the included events, Pro tier usage is billed at **$1.20 per million events**.

### Setting budgets

You can configure your trace budgets in the Encore Cloud dashboard. By setting daily and monthly limits, you define exactly how much you're willing to spend on tracing. This makes tracing costs fully predictable and prevents any surprises on your bill.
