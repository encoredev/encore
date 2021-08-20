---
title: Development Dashboard
---

Encore comes with a built-in Development Dashboard for simplifying local development, with several features to make building your application even better

* [Distributed Tracing](./tracing) for easy and powerful debugging
* [API Documentation](../develop/api-docs) for knowledge sharing and answering questions
* [Live-streamed logs](./logging) from your application 
* An API Explorer for easily making API calls against your backend

All of these features update in real-time as you make changes to your application.

To access the dashboard, start your Encore application with `encore run` and then follow the link in your terminal:

```bash
$ encore run
API Base URL:      http://localhost:4060
Dev Dashboard URL: http://localhost:62709/hello-world-cgu2
```

You can also open up the API Base URL in your browser (defaults to [http://localhost:4060](http://localhost:4060)) and it will redirect to the Development Dashboard.

![Dev Dashboard Screenshot](https://encore.dev/assets/img/dev-dash-screenshot.png "Dev Dashboard Screenshot")