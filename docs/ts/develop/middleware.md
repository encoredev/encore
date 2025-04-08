---
seotitle: Using Middleware in your Encore.ts application
seodesc: See how you can use middleware in your Encore.ts application to handle cross-cutting generic functionality, like request logging, auth, or tracing.
title: Middleware
subtitle: Handling cross-cutting, generic functionality
lang: ts
---

Middleware is a way to write reusable code that runs before, after, or both before and after
the handling of API requests, often across several (or all) API endpoints.

Middleware is commonly used to implement cross-cutting concerns like
[request logging](/docs/ts/observability/logging),
[authentication](/docs/ts/develop/auth),
[tracing](/docs/ts/observability/tracing),
and so on. One of the benefits of Encore.ts is that
it handles these common use cases out-of-the-box, so there's no
need to write your own middleware.

However, when developing applications there's often some use cases where it can be useful to write
reusable functionality that applies to multiple API endpoints, and middleware
is a good solution for this.

Encore provides built-in support for middleware by adding functions to the
[Service definitions](/docs/ts/primitives/services) configuration.
Each middleware can be configured with a `target` option to specify what
API endpoints it applies to.

<GitHubLink
    href="https://github.com/encoredev/examples/tree/main/ts/middleware"
    desc="Example app with two middleware; a rate limiter and one for user authorization."
/>

## Middleware functions

The simplest way to create a middleware is to use the `middleware` helper in `encore.dev/api`,
here is an example of a middleware that will run for endpoints that require auth:

```ts
import { middleware } from "encore.dev/api";

export default new Service("myService", {
    middlewares: [
        middleware({ target: { auth: true } }, async (req, next) => {
            // do something before the api handler
            const resp = await next(req);
            // do something after the api handler
            return resp
        })
    ]
});

```

Middleware forms a chain, allowing each middleware to introspect and process
the incoming request before handing it off to the next middleware by calling the
`next` function that's passed in as an argument. For the last middleware in the
chain, calling `next` results in the actual API handler being called.

The `req` parameter provides information about the incoming request, it has different fields
depending on what kind of handler it is.

You can get information about the current request via `req.requestMeta` if the endpoint is a
[typed API endpoint](/docs/ts/primitives/defining-apis) or a
[Streaming API endpoint](/docs/ts/primitives/streaming-apis).

For [Streaming API endpoints](/docs/ts/primitives/streaming-apis) you can also access the stream
via `req.stream` method.

For [Raw Endpoints](/docs/ts/primitives/raw-endpoints) you can access the raw request and the
raw response via `req.rawRequest` and `req.rawResponse`.

The `next` function returns a `HandlerResponse` object which contains the response from the API.
Extra response headers can be added using `resp.header.set(key, value)` or `resp.header.add(key, value)`,
if the endpoint is a [typed API endpoint](/docs/ts/primitives/defining-apis).

To pass data from middleware to an API handler, you can assign values to `req.data` within the middleware. These values can then be accessed in the handler using `currentRequest()`.

Here’s an example:

```ts
const mw = middleware(async (req, next) => {
  // Assign a value to the request
  req.data.myMiddlewareData = { some: "data" };

  return await next(req);
});

export const ep = api(
  { expose: true, method: "GET", path: "/endpoint" },
  async () => {
    const callMeta = currentRequest() as APICallMeta;

    // Access the value in the API handler
    const myData = callMeta.middlewareData?.myMiddlewareData;
    // Use the data as needed
  },
);
```

## Middleware ordering

Middleware runs in the order they are defined in the [Service definitions](/docs/ts/primitives/services)
configuration, i.e:

```ts
export default new Service("myService", {
    middlewares: [
        first,
        second,
        third
    ],
});

```

## Targeting APIs

The `target` option specifies which endpoints within the service the middleware should run on. If not set, the middleware will run for all endpoints by default.

For better performance, use the `target` option instead of filtering within the middleware function. This allows the applicable middleware to be determined per endpoint during startup, reducing runtime overhead.

The following options are available for targeting endpoints:

- `tags`: A list of tags evaluated with `OR`, meaning the middleware applies to an endpoint if the endpoint has at least one of these tags.
- `expose`: A boolean indicating whether the middleware should be applied to endpoints that are exposed or not exposed.
- `auth`: A boolean indicating whether the middleware should be applied to endpoints that require authentication or not.
- `isRaw`: A boolean indicating whether the middleware should be applied to raw endpoints.
- `isStream`: A boolean indicating whether the middleware should be applied to stream endpoints.

