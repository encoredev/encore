---
seotitle: Hello World in Encore.ts
seodesc: Get to know Encore.ts with this simple Hello World example.
title: Hello World
subtitle: Get to know the basics
toc: false
lang: ts
---

Encore lets you easily define type-safe, idiomatic TypeScript API endpoints.
It's done in a fully declarative way, enabling Encore to automatically parse and validate the incoming request and ensure it matches the schema, with zero boilerplate.

To define an API, use the `api` function from the `encore.dev/api` module to wrap a regular TypeScript async function that receives the request data as input and returns response data. This tells Encore that the function is an API endpoint. Encore will then automatically generate the necessary boilerplate at compile-time.

This means you need less than 10 lines of code to define a production-ready deployable service and API endpoint:

```TypeScript
import { api } from "encore.dev/api";

export const get = api(
  { expose: true, method: "GET", path: "/hello/:name" },
  async ({ name }: { name: string }): Promise<Response> => {
    const msg = `Hello ${name}!`;
    return { message: msg };
  }
);

interface Response {
  message: string;
}
```

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/hello-world" 
    desc="Hello World REST API example application." 
/>

## Getting started video

Get to know the basics of Encore.ts in this getting started video.

<iframe width="360" height="202" src="https://www.youtube.com/embed/wiLDz-JUuqY?si=BxmW0BV1hx2LIvtO" title="Getting Started with Encore.ts" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>

## Using databases, Pub/Sub, and other primitives

Encore's Backend Framework makes it simple to add more primitives, such as additional microservices, databases, Pub/Sub, etc.
See how to use each primitive:

- [Services](/docs/ts/primitives/services)
- [APIs](/docs/ts/primitives/defining-apis)
- [Databases](/docs/ts/primitives/databases)
- [Cron Jobs](/docs/ts/primitives/cron-jobs)
- [Pub/Sub & Queues](/docs/ts/primitives/pubsub)
- [Secrets](/docs/ts/primitives/secrets)
