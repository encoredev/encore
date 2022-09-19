---
title: Development Dashboard
---

Encore comes with a built-in Development Dashboard for simplifying local development, with several features to make building your application even better

* [Distributed Tracing](./tracing) for easy and powerful debugging
* [API Documentation](../develop/api-docs) for knowledge sharing and answering questions
* [Encore Flow](/docs/develop/encore-flow) for visualizing your cloud microservices architecture
* [Live-streamed logs](./logging) from your application
* An API Explorer for easily making API calls against your backend

All of these features update in real-time as you make changes to your application.

To access the dashboard, start your Encore application with `encore run` and then follow the link in your terminal:

```bash
$ encore run
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:62709/hello-world-cgu2
```

You can also open up the API Base URL in your browser (defaults to [http://localhost:4000](http://localhost:4000)) and it will redirect to the Development Dashboard.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/localdevdash.mp4" className="w-full h-full" type="video/mp4" />
</video>
