---
seotitle: Encore Application Model
seodesc: How Encore understands your application using static analysis
title: Encore Application Model
subtitle: How Encore understands your application
lang: ts
faq:
  - q: "Can I see the model?"
    a: "Yes. The [local development dashboard](/docs/ts/observability/dev-dash) renders it as a service catalog and an architecture diagram, so you can inspect the services, endpoints and resources the parser found."
  - q: "Does building the model send my code anywhere?"
    a: "No. The parser runs locally as part of the Encore CLI, which is open source. Parsing happens on your machine during `encore run` and during builds."
  - q: "The model can't drift, but what if someone changes a resource in the cloud console?"
    a: "Those are two different things. The model describes what the application needs and is derived from your code, so it cannot fall out of step with it. The per-environment settings are separate: if a resource is modified outside Encore, the next deploy detects it and updates Encore's own record to match rather than overwriting your change. See [managing infrastructure](/docs/platform/infrastructure/managing-infrastructure)."
  - q: "Does static analysis constrain the rest of my code?"
    a: "No. The requirements above apply to resource declarations only. Everything else is ordinary TypeScript, with whatever libraries, patterns and abstractions you would use anyway, and the parser does not look at it."
  - q: "Does static analysis slow down my builds?"
    a: "Parsing and topology analysis take under a second for a typical application, and run before provisioning rather than during it."
  - q: "Is the model specific to TypeScript?"
    a: "No. Encore builds the same model for TypeScript and Go applications. The parser differs per language, the model it produces does not."
---

Encore uses static analysis to build a model of your application, called the Encore Application Model. The parser reads your source and produces a structured description of the services, APIs and infrastructure your code declares, together with how each part uses the others.

Take a SQL database, one of [Encore's infrastructure primitives](/docs/ts/primitives):

```ts
const db = new SQLDatabase("orders", { migrations: "./migrations" });
```

From that line the parser records three things: the application uses a SQL database called `orders`, its schema comes from the `./migrations` directory, and these are the services that query it. Nothing else about the database is in the code, because nothing else affects how the code behaves.

That same declaration produces a Postgres database in a local container when you run `encore run`, and a managed Postgres instance in your own cloud account when you deploy, with the migrations applied in each case. Which managed service backs it is [an environment setting](/docs/platform/infrastructure/infra) rather than something in the code, so a change that works on your machine behaves the same way in production. The [development workflow](/docs/platform/workflow) covers how one declaration carries from your laptop through per-PR preview environments to production.

The model is rebuilt from source on every `encore run` and every build, so it cannot drift from the code it describes. Because the parser and the SDK are designed together, a declaration the parser cannot resolve fails the build instead of failing at runtime. The SDK, parser and compiler that produce the model are all [Open Source](https://github.com/encoredev/encore).

Encore renders the model as an architecture diagram, without you drawing anything. Each box is a service, with a count of its public, authenticated and private endpoints and the databases it uses. Hovering a service shows which services call it and how many RPCs each caller makes, because the model records those call sites too:

<video autoPlay playsInline loop muted className="w-full h-auto">
	<source src="/assets/docs/flow-diagram.mp4" type="video/mp4" />
</video>

## What the model contains

- Services, and the directory each one lives in
- API endpoints, with the full type schema of every request and response
- SQL databases and their migration directories
- Pub/Sub topics and subscriptions
- Cron jobs, object storage buckets, caches and secrets
- Middleware and API gateways

Running a build without supplying infrastructure configuration prints the part of the model that has to be satisfied:

```
$ encore build docker myapp:latest

Your infra configuration is incomplete

Missing Resource Configurations:
  Secrets      : SlackWebhookURL
  Databases    : monitor, site
  Subscriptions: uptime-transition/slack-notification, site.added/check-site
  Topics       : uptime-transition, site.added
```

## What the model makes possible

Because the model records usage and not only declaration, Encore can derive things you would otherwise write and maintain by hand, and IAM is the clearest case. Say two services share one `uploads` bucket:

```ts
// in the ingest service
await uploads.upload("q3.csv", csvBytes);

// in the reports service
const csv = await uploads.download("q3.csv");
```

The parser sees a download in one and an upload in the other, so Encore grants `reports` read access to that bucket and `ingest` write, and neither service gets more than it uses. Bucket usage alone distinguishes nine operations, from reading object contents to generating signed upload URLs. The same holds for which service publishes to which topic and which service calls which endpoint.

The model drives provisioning, [request validation](/docs/ts/primitives/validation) against the declared schemas, [generated clients](/docs/ts/cli/client-generation) and [API documentation](/docs/ts/develop/api-docs), and [distributed tracing](/docs/ts/observability/tracing) across service boundaries. None of these can disagree with each other, because there is only one description for them to disagree with.

## Requirements on your code

Resource declarations are read from your source without running it, which constrains how you write them.

### Names must be string literals

A name assembled from a variable or a template fails the build:

```ts
const regions = ["eu", "us", "ap"];
export const orderTopics = regions.map(
  (r) => new Topic<string>(`orders-${r}`, { deliveryGuarantee: "at-least-once" }),
);
```

```
error: expected string literal
 --> orders/topics.ts:5:10
  |
5 |   (r) => new Topic<string>(`orders-${r}`, { deliveryGuarantee: "at-least-once" }),
  |          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
```

### Declarations must be assigned at module scope

The parser collects top-level bindings, so a constructor call nested inside a function, a conditional or a loop never reaches the model, and nothing warns you:

```ts
if (process.env.SHARD) {
  // Not part of the model, so never provisioned.
  new SQLDatabase("shard", { migrations: "./migrations" });
}
```

API endpoints have to be exported rather than merely bound, and service definitions go the other way, since a service is the module's default export rather than a named variable.

## Infrastructure outside the model

The model covers a fixed set of resource kinds, so a search cluster, a third-party API, or a set of queues whose number depends on runtime data all sit outside it. You provision those however you like and reach them from your code the way you would any external dependency, with connection details held in a [secret](/docs/ts/primitives/secrets):

```ts
import { secret } from "encore.dev/config";

const searchEndpoint = secret("SearchEndpoint");

export const search = api(
  { expose: true, method: "GET", path: "/search" },
  async ({ q }: { q: string }): Promise<Results> => {
    const resp = await fetch(`${searchEndpoint()}/query?q=${q}`);
    return resp.json();
  },
);
```

Encore knows about the secret and the endpoint that uses it, so both appear in the model, while the cluster behind the endpoint does not.

Going the other way, the [Terraform Provider](/docs/platform/integrations/terraform) exposes data sources for the resources Encore did provision, so existing Terraform can reference an Encore database or topic by name.
