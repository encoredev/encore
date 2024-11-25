---
seotitle: Development dashboard for local development
seodesc: Encore's Local Development Dashboard comes with build-in distributed tracing, API docs, and real-time architecture diagrams.
title: Local Development Dashboard
subtitle: Built-in tools for simplicity and productivity
lang: ts
---

Encore provides an efficient local development workflow that automatically provisions [local infrastructure](/docs/platform/infrastructure/infra#local-development) and supports automated testing with dedicated test infrastructure.

The local environment also comes with a built-in Local Development Dashboard to simplify development and improve productivity. It has several features to help you design, develop, and debug your application:

* [Service Catalog](/docs/ts/observability/service-catalog) with Automatic API Documentation
* API Explorer to call your APIs
* [Distributed Tracing](/docs/ts/observability/tracing) for simple and powerful debugging
* [Encore Flow](/docs/develop/encore-flow) for visualizing your microservices architecture

All these features update in real-time as you make changes to your application.

To access the dashboard, start your Encore application with `encore run` and it will open automatically. You can also follow the link in your terminal:

```bash
$ encore run
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:9400/hello-world-cgu2
```
<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/localdashvideo.mp4" className="w-full h-full" type="video/mp4" />
</video>
