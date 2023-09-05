---
seotitle: Encore TypeScript (Preview)
seodesc: Encore makes it easy to define fully type-safe, idiomatic APIs in TypeScript.
title: TypeScript (Preview)
subtitle: Your feedback is welcome!
---
_This section shows how Encore's upcoming TypeScript support is intended to work and focuses only on TypeScript specifics. It does not contain full documentation on all Encore features. Please use [the complete Go docs](/docs) as a reference point for how Encore works generally._

## Example backend application

Encore makes it easy to define fully type-safe, idiomatic APIs in TypeScript.

Each API endpoint is a regular TypeScript async function that receives the request data as input and returns response data.
Encore automatically handles the JSON parsing & validation of incoming HTTP requests, as well as serializing
the response back to JSON for the HTTP response.

Here's an example backend application written with Encore's TypeScript APIs, representing a simple uptime monitoring service:

<div className="not-prose my-10">
   <Editor projectName="typescriptUptime" />
</div>

While the example clocks in at just over 200 lines of code, it contains 10 API endpoints across 3 backend services,
two databases, two Pub/Sub topics & subscriptions, and a Cron Job. Check out the full architecture below:

<img className="w-full h-auto" src="/assets/tutorials/uptime/encore-flow.png" title="Encore Flow" />

[Check out Encore's Cloud Dashboard](https://app.encore.dev/uptime-7chi) for this application to understand
what it looks like when the application is deployed to the cloud.

## Defining API Endpoints

Encore allows you to easily define type-safe, idiomatic TypeScript API endpoints.

It's easy to accept both the URL path parameters, as well as JSON request body data, HTTP headers, and query strings.

It's all done in a way that is fully declarative, enabling Encore to automatically parse and validate the incoming request
and ensure it matches the schema, with zero boilerplate.

Here's an example of a simple "Hello World" API endpoint.

```typescript
import { APIEndpoint } from "@encore.dev/api"

export interface Request {
  name: string;
}

export interface Response {
  message: string;
}

export const Hello = APIEndpoint<Request, Response>(
  {path: "/hello"},
  async ({name}) => {
    return {message: `Hello ${name}!`};
  }
);
```

With this code you can run:

```shell
$ encore run  # Run the API
$ curl 'http://localhost:4000/hello' -d '{"name": "World"}'
{"message":"Hello World!"}
```

Encore handles setting up the HTTP server, parsing the incoming request, validating it against the schema,
and serializing the response back to JSON, and instrumenting the endpoint with distributed tracing.

Everything is type-safe and simple, idiomatic TypeScript, with no boilerplate required.

Additionally, Encore statically parses the API endpoints and provides automatic API documentation out of the box.

### Path Parameters

Encore supports defining URL path parameters are specified in the `path` field using the `:name` syntax
and changing the type in the `Request` interface to `Path`.

For example, we could change the Hello World API above to take the name as a path parameter:

```typescript
import { APIEndpoint, Path } from "@encore.dev/api"

export interface Request {
  name: Path; // use Path here to indicate `name` is read from the URL path
}

export interface Response {
  message: string;
}

export const Hello = APIEndpoint<Request, Response>(
  {path: "/hello/:name"}, // add the path parameter here
  async ({name}) => {
    return {message: `Hello ${name}!`};
  }
);
```

The rest of the code stays the same. This can now be called as:

```shell
$ curl http://localhost:4000/hello/World
{"message":"Hello World!"}
```

### Headers

Similarly to path parameters, you can just as easily handle HTTP headers
by changing the field type to `Header<"My-Header-Name">`.

This can be used in both request and response data types.

For request types Encore reads the data from the HTTP header (instead of from the JSON request body).
For response types Encore writes the data as a HTTP header (instead of to the JSON response body).

In the example below, the `language` field of `ListBlogPost` will be fetched from the
`Accept-Language` HTTP header.

```typescript
import { Header } from "@encore.dev/api"

interface ListBlogPost {
    language: Header<"Accept-Language">; // read from the Accept-Language header
    author: string;                      // read from the JSON body
}
```

### Query parameters

Similarly to HTTP headers, the field type can be changed to `Query` to read the data from the query string
instead of from the JSON request body. 

Note that query strings are not supported in HTTP responses and therefore `Query` fields become
regular body parameters in responses.

For example:

```typescript
import { Query } from "@encore.dev/api"

interface ListBlogPost {
    limit: Query<number> // always a query parameter
    author: string       // query if GET, HEAD or DELETE, otherwise body parameter
}
```

## Declarative Infrastructure

Encore offers a simple, declarative way to define cloud infrastructure resources, directly in your application code.
This comes with several, substantial benefits over traditional Infrastructure as Code (IaC) tools like Terraform

* **Develop new features locally as if the infrastructure is already set up**:
  Encore automatically compiles your app and sets up the necessary infrastructure on the fly.

* **No manual maintenance required**:
  There is no need to manually write IaC configuration, like Terraform, and no need to maintain configuration for multiple environments manually.
  Encore uses your application code as the single source of truth and automatically keeps all environments in sync.
  
* **One codebase for all environments**:
  Encore automatically provisions your local, preview, and cloud environments (using your own cloud account).

* **Cloud-agnostic by default**:
  The Infrastructure SDK is an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
 
* **Evolve infrastructure without code changes**:
  As your requirements evolve, you can change and configure the provisioned infrastructure by using Encore's Cloud Dashboard or your cloud provider's console.

See below for specific examples (or in the example at the top).

### Pub/Sub

Define a topic with `new Topic` in a single line of code:

```typescript
import { Topic } from "@encore.dev/pubsub"

interface MyMessage { message: string }

const MyTopic = new Topic<MyMessage>("my-topic", { deliveryGuarantee: "at-least-once" });

// Publish a message with no additional configuration
await MyTopic.publish({ message: "Hello, world!" });
```

Similarly define a subscription with `new Subscription` to start receiving messages:

```typescript
import { Subscription } from "@encore.dev/pubsub"

const _ = new Subscription(MyTopic, "my-subscription", {
    handler: async (message: MyMessage) => {
        console.log(message.message);
    }
})
```

With this, Encore automatically handles everything else, including infrastructure provisioning, local development
and testing, IAM and security, and more, using the native Pub/Sub services of your cloud provider.

### SQL Databases

Define a SQL database with `new SQLDatabase` in a single line of code:

```typescript
import { SQLDatabase } from "@encore.dev/storage/sqldb"

const MyDB = new SQLDatabase("my-database");

// Start querying the database with no additional configuration
await MyDB.query("SELECT * FROM my_table");
```

Like with Pub/Sub, Encore automatically handles provisioning the database, local development and testing,
IAM and security, and more.

### Cron Jobs

Define a Cron Job with a single line of code:

```typescript
import { CronJob } from "@encore.dev/cron"

const _ = new CronJob("my-cron-job", { endpoint: MyEndpoint, every: "1 hour"});
```

Like other resources Encore takes care of provisioning the cron job and monitors its executions
using its automatic distributed tracing.

### Secrets

Define and access secrets with a single line of code:

```typescript
import { Secrets } from "@encore.dev/api"

const secrets = Secrets<{ my_secret: string; }>();

// Access secrets.my_secret anywhere in your code.
// It's automatically loaded by Encore as soon as the service starts up.
```

The secret values are managed via the Encore CLI or the Cloud Dashboard, and automatically propagate to your
Cloud Provider on deployment and delivered securely to the application at runtime.

## Migrating to Encore

Encore makes it easy to migrate existing applications to Encore, as you can keep using your existing cloud account in AWS or GCP.
In most cases, gradually migrating your existing backend services to Encore takes minimal effort.

### Migrating using Fallback routes

A common migration path is starting with an existing router and using Fallback routes that will be called if no other endpoint matches the request.

Encore supports defining fallback routes using the syntax `path: "/!fallback"`.

This is often useful when migrating an existing Express backend service over to Encore, as it allows you to gradually
migrate endpoints over to Encore while routing the remaining endpoints to the existing HTTP router using a raw endpoint (see below) with a fallback route.

For example:

```typescript
import * as express from "express";

const oldRouter = express.Router()

export const Fallback = APIEndpoint(
  {raw: true, path: "/!fallback"},
  (req: express.Request, res: express.Response, next: express.NextFunction) => {
    oldRouter(req, res, next);
  }
);
```

### Raw endpoints

In some cases you may need to fulfill an API schema that is defined by someone else, for instance when you want to accept webhooks.
This often requires you to parse custom HTTP headers and do other low-level things that Encore usually lets you skip.

For these circumstances Encore lets you define raw endpoints. Raw endpoints operate at a lower abstraction level, giving you access to the underlying HTTP request.

To define a raw endpoint, add `raw: true` to the `APIEndpoint` options.
This will cause the function to receive the raw HTTP request and response objects from Express instead of the parsed request and response data.

For example:

```typescript
export const MyRawEndpoint = APIEndpoint(
  {raw: true, path: "/raw"},
  (req: express.Request, res: express.Response) => {
    res.send("Hello, raw world!")
  }
);
```

```shell
$ curl 'http://localhost:4000/raw'
Hello, raw world!
```
