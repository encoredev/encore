---
seotitle: Development dashboard for local development
seodesc: Encore's Local Development Dashboard comes with build-in distributed tracing, API docs, and real-time architecture diagrams.
title: Development Dashboard
---

Encore comes with a built-in Development Dashboard to simplify local development. It has several features to help you design, develop, and debug your application:

* An API Explorer for easily making API calls against your backend
* [Distributed Tracing](./tracing) for simple and powerful debugging
* [API Documentation](../develop/api-docs) for knowledge sharing and answering questions
* [Encore Flow](/docs/develop/encore-flow) for visualizing your microservices architecture

All these features update in real-time as you make changes to your application.

To access the dashboard, start your Encore application with `encore run` and then follow the link in your terminal:

```bash
$ encore run
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:9400/hello-world-cgu2
```
<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/localdashvideo.mp4" className="w-full h-full" type="video/mp4" />
</video>
