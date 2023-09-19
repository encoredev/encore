---
seotitle: Development dashboard for local development
seodesc: Encore's Local Development Dashboard comes with build-in distributed tracing, API docs, and real-time architecture diagrams.
title: Local Development Dashboard
subtitle: Built-in tools for simplicity and productivity
---

Encore provides an efficient local development workflow that automatically provisions [local infrastructure](/docs/deploy/infra#local-development) and supports [automated testing](/docs/develop/testing) with dedicated test infrastructure.

The local environment also comes with a built-in Local Development Dashboard to simplify development and improve productivity. It has several features to help you design, develop, and debug your application:

* [Service Catalog](/docs/develop/api-docs) and API Explorer for easily making API calls to your local backend
* [Distributed Tracing](./tracing) for simple and powerful debugging
* [Automatic API Documentation](../develop/api-docs) for knowledge sharing and answering questions
* [Encore Flow](/docs/develop/encore-flow) for visualizing your microservices architecture

All these features update in real-time as you make changes to your application.

To access the dashboard, start your Encore application with `encore run` and it opens automatically. You can also follow the link in your terminal:

```bash
$ encore run
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:9400/hello-world-cgu2
```
<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/localdashvideo.mp4" className="w-full h-full" type="video/mp4" />
</video>
