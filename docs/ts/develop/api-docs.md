---
seotitle: API Documentation – Document your Encore.ts app
seodesc: Learn how to write doc comments in your Encore.ts app that appear in the Service Catalog, generated clients, and OpenAPI specs.
title: API Documentation
subtitle: Write doc comments that are automatically surfaced across your app
lang: ts
---

Encore parses doc comments from your TypeScript source code and surfaces them in the [Service Catalog](/docs/ts/observability/service-catalog), [generated API clients](/docs/ts/cli/client-generation), and OpenAPI specs. This means your documentation stays in your code and is always up to date.

Both `//` line comments and `/** */` JSDoc comments are supported.

## Services

A service's documentation comes from the comment immediately above the `new Service` call in `encore.service.ts`.

```ts
import { Service } from "encore.dev/service";

// Payments handles payment processing, billing,
// and subscription management.
export default new Service("payments");
```

Service docs appear in the Service Catalog and in generated API clients.

## API endpoints

Add a comment above the endpoint function to document it.

```ts
import { api } from "encore.dev/api";

// Charge charges the given payment method.
// It is idempotent; calling it multiple times
// with the same idempotency key has no additional effect.
export const charge = api(
  { expose: true, auth: true, method: "POST", path: "/charge" },
  async (params: ChargeParams): Promise<ChargeResponse> => {
    // ...
  },
);
```

In the OpenAPI spec, the first line becomes the `summary` and the remaining lines become the `description`.

## Auth handlers

```ts
import { authHandler } from "encore.dev/auth";

// AuthenticateRequest validates the API key provided in the Authorization header
// and returns the authenticated user's information.
export const auth = authHandler(async (params: AuthParams): Promise<AuthData> => {
    // ...
});
```

## Types

Document interfaces and their fields with comments above each declaration.

```ts
// ChargeParams defines the parameters for the Charge endpoint.
interface ChargeParams {
  // The identifier of the payment method to charge.
  paymentMethodId: string;

  // The amount to charge, in the smallest currency unit (e.g. cents).
  amount: number;

  // ISO 4217 currency code.
  currency: string;
}
```

Both the type-level and field-level comments are surfaced in generated clients and OpenAPI schemas.

## Infrastructure resources

Doc comments are also supported on infrastructure resource declarations.

```ts
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { Topic, Subscription } from "encore.dev/pubsub";
import { CronJob } from "encore.dev/cron";
import { CacheCluster, CacheKeyspace } from "encore.dev/storage/cache";
import { Bucket } from "encore.dev/storage/objects";
import { Counter } from "encore.dev/metrics";
import { secret } from "encore.dev/config";
import { Gateway } from "encore.dev/api";

// TransactionDB stores payment transactions and related records.
const TransactionDB = new SQLDatabase("transactions", {
  migrations: "./migrations",
});

// PaymentEvents publishes events when payment states change.
const PaymentEvents = new Topic<PaymentEvent>("payment-events", {
  deliveryGuarantee: "at-least-once",
});

// ProcessPaymentEvent handles incoming payment events.
const _ = new Subscription(PaymentEvents, "process-payment", {
  handler: handlePaymentEvent,
});

// DailySettlement runs the settlement process every day at midnight.
const DailySettlement = new CronJob("daily-settlement", {
  every: "24h",
  endpoint: settle,
});

// PaymentCache caches frequently accessed payment records.
const PaymentCache = new CacheCluster("payment-cache", {
  evictionPolicy: "allkeys-lru",
});

// PaymentByID caches payments by their unique identifier.
const PaymentByID = new CacheKeyspace<Payment>(PaymentCache, {
  keyPattern: "payment/:id",
});

// Receipts stores generated receipt PDFs.
const Receipts = new Bucket("receipts", {});

// ChargeAmount tracks the total amount charged per currency.
const ChargeAmount = new Counter<ChargeLabels>("charge_amount", {});

interface ChargeLabels {
  // ISO 4217 currency code.
  currency: string;
}

// StripeAPIKey is the secret key for the Stripe API.
const StripeAPIKey = secret("StripeAPIKey");

// PaymentGateway is the API gateway for the payments service.
const PaymentGateway = new Gateway({ authHandler: auth });
```

These docs appear in the Service Catalog and are available through the [MCP server](/docs/ts/ai-integration#mcp-server) for AI-assisted development.

## Where docs appear

| Where | What's included |
|---|---|
| [Service Catalog](/docs/ts/observability/service-catalog) | Services, endpoints, types, fields |
| [Generated clients](/docs/ts/cli/client-generation) | Services, endpoints, types, fields |
| OpenAPI spec | Endpoints (summary + description), types, fields |
| [MCP server](/docs/ts/ai-integration#mcp-server) | All resource types |
