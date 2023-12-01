---
seotitle: Developing Services and APIs
seodesc: Learn how to create microservices and define APIs for your cloud backend application using TypeScript and Encore. The easiest way of building cloud backends.
title: Services and APIs
subtitle: Simplifying (micro-)service development
---

Encore makes it simple to build applications with one or many services, without needing to manually handle the typical complexity of developing microservices.

## Defining a service

With Encore you define a service by creating a folder and inside that folder [defining one or more APIs](#defining-apis) within a regular TypeScript file. Encore recognizes this as a service, and uses the folder name as the service name. When deploying, Encore will automatically [provision the required infrastructure](/docs/deploy/infra) for each service.

On disk it might look like this:

```
/my-app
├── encore.app          // ... and other top-level project files
├── package.json  
│
├── hello               // hello service (a folder)
│   ├── hello.ts        // hello service code
│   └── hello_test.ts   // tests for hello service
│
└── world               // world service (a Go package)
    └── world.ts        // world service code
```


This means building a microservices architecture is as simple as creating multiple directories within your application.

## Defining APIs

Encore allows you to easily define type-safe, idiomatic TypeScript API endpoints.

It's easy to accept both the URL path parameters, as well as JSON request body data, HTTP headers, and query strings.

It's all done in a way that is fully declarative, enabling Encore to automatically parse and validate the incoming request
and ensure it matches the schema, with zero boilerplate.

To define an API, use the `api` function from the `@encore.dev/api` module to wrap a regular TypeScript async function that receives the request data as input and returns response data.
This tells Encore that the function is an API endpoint. Encore will then automatically generate the necessary boilerplate at compile-time.

In the example below, we define the API endpoint `ping` which accepts `POST` requests and is exposed as `hello.ping` (because our service name is `hello`).

```typescript
// inside the hello.ts file
import { api } from "@encore.dev/api"

export const ping = api(
  { method: "POST" },
  async ({ name }: PingParams): Promise<PingResponse> => {
    return { message: `Hello ${name}!` };
  }
);
```

### Request and response schemas

In the example above we defined an API that uses request and response schemas. The request data is of type `PingParams` and the response data of type `PingResponse`. That means we need to define them like so:

```typescript
// inside the hello.ts file
import { api } from "@encore.dev/api"

// PingParams is the request data for the Ping endpoint.
interface PingParams {
  name: string;
}

// PingResponse is the response data for the Ping endpoint.
interface PingResponse {
  message: string;
}

// hello is an API endpoint that responds with a simple response.
// This is exposed as "hello.ping".
export const hello = api(
  { method: "POST", path: "/hello" },
  async ({ name }: PingParams): Promise<PingResponse> => {
    return { message: `Hello ${name}!` };
  }
);
```

Request and response schemas are both optional in case you don't need them.
That means there are four different ways of defining an API:

* `api(async (params: Params): Promise<Response> => {});` &ndash; when you need both.
* `api(async (): Promise<Response> => {});` &ndash; when you only return a response.
* `api(async (params: Params): Promise<void> => {});` &ndash; when you only respond with success/fail.
* `api(async (): Promise<void> => {});` &ndash; when you need neither request nor response data.

The `api` function is a generic function. 

You can also pass the type arguments for the request and response objects to the `api` function which looks like this: `api<Params, Response>(async (params) => {});`

This approach is simple but very powerful. It lets Encore use [static analysis](/docs/introduction#meet-the-encore-application-model)
to understand the request and response schemas of all your APIs, which enables Encore to automatically generate API documentation, type-safe API clients, and much more.

### Access controls

When you define an API, you have the option to supply an `APIOptions` object as the first argument to `api`. 
In the options object you can set how the endpoint can be accessed:

* `{ access: "public" }` &ndash; defines a public API that anybody on the internet can call (this is the default value if no access field is set).
* `{ access: "private" }` &ndash; defines a private API that is never accessible to the outside world. It can only be called from other services in your app and via cron jobs.
* `{ access: "auth" }` &ndash; defines a public API that anybody can call, but requires valid authentication.

You can optionally send in auth data to `public` and `private` APIs, in which case the auth handler will be used. When used for `private` APIs, they are still not accessible from the outside world.

[//]: # (TODO: Add link to auth guide when it's ready.)
[//]: # (For more on defining APIs that require authentication, see the [authentication guide]&#40;/docs/develop/auth&#41;.)

### REST APIs
Encore has support for RESTful APIs and lets you easily define resource-oriented API URLs, parse parameters out of them, and more.

To create a REST API, start by defining an endpoint and specify the `method` and `path` fields in the `APIOptions` object.

[//]: # (TODO: Add link to when it's ready.)
[//]: # (Learn more in the [API schemas guide](/docs/develop/api-schemas#path-parameters).)

To specify a placeholder variable, use `:name` and add a function parameter with the same name to the function signature. Encore parses the incoming request URL and makes sure it matches the type of the parameter.

For example, if you want to have a `getBlogPost` endpoint that takes a numeric id as a parameter:

```typescript


// getBlogPost retrieves a blog post by id.
const getBlogPost = api(
  { method: "GET", path: "/blog/:id" },
  async ({ id }: { id: number }): Promise<BlogPost> => { 
	  // Use id to query database...
  },
);
```

You can also combine path parameters with body payloads. For example, if you want to have an `updateBlogPost` endpoint:

```typescript
interface Params {
  id: number;
  post: BlogPost;
}

// updateBlogPost updates an existing blog post by id.
const updateBlogPost = api(
  { method: "PUT", path: "/blog/:id" },
  async ({ id, post }: Params): Promise<BlogPost> => { 
	  // Use id to query database...
  },
);
```

<Callout type="important">

You cannot define paths that conflict with each other, including paths
where the static part can be mistaken for a parameter, e.g both `/blog` and `/blog/:id` would conflict with `/:username`.

</Callout>

As a rule of thumb, try to place path parameters at the end of the path and
prefix them with the service name, e.g:

```
GET /blog/posts
GET /blog/posts/:id
GET /user/profile/:username
GET /user/me
```

#### Query parameters

Query parameters are coming soon for the TypeScript beta.

[//]: # (TODO: Add info about how to use query params when they are available.)

### Raw endpoints

In case you need to operate at a lower abstraction level, Encore will support defining raw endpoints that let you access the underlying HTTP request. This is often useful for things like accepting webhooks.

Raw endpoints are coming soon for the TypeScript beta.

[//]: # (TODO: Add info about raw endpoints when they are available.)

## Calling APIs
Calling an API endpoint looks like a regular function call with Encore. To call an endpoint you first need to import the service from `encore.app/clients` and then call the API endpoint like a regular function.
When compiling your application, Encore uses [static analysis](/docs/introduction#meet-the-encore-application-model) to parse all APIs and make then available through the `encore.app/clients` module for internal calls.

In the example below, we import the service `hello` and call the `ping` endpoint using a function call to `hello.ping`.

```typescript
import { hello } from "encore.app/clients"; // import service

export const myOtherAPI = api(async (): Promise<void> => {
    const resp = await hello.ping({ name: "World" });
    console.log(resp.message); // "Hello World!"
  }
);
```

This means your development workflow is as simple as building a monolith, even if you use multiple services.
You get all the benefits of function calls, like compile-time checking of all the parameters and auto-completion in your editor, while still allowing the division of code into logical components, services, and systems.

[//]: # (TODO: Add info about the current request meta data when available.)
