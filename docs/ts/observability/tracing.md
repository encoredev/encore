---
seotitle: Distributed Tracing helps you understand your app
seodesc: See how to use distributed tracing in your backend application, across multiple services, using Encore.
title: Distributed Tracing
subtitle: Track requests across your application and infrastructure
lang: ts
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

For cases where this is undesirable, such as for passwords or personally identifiable information (PII), Encore supports marking endpoints as sensitive. When an endpoint is marked as sensitive, the request and response details from that endpoint will be automatically redacted from the traces it produces.

See the documentation on [API Schemas](/docs/ts/primitives/defining-apis#sensitive-data) for more information.
