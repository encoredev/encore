---
seotitle: API Documentation – Document your Encore.go app
seodesc: Learn how to write doc comments in your Encore.go app that appear in the Service Catalog, generated clients, and OpenAPI specs.
title: API Documentation
subtitle: Write doc comments that are automatically surfaced across your app
lang: go
---

Encore parses doc comments from your Go source code and surfaces them in the [Service Catalog](/docs/go/observability/service-catalog), [generated API clients](/docs/go/cli/client-generation), and OpenAPI specs. This means your documentation stays in your code and is always up to date.

## Services

A service's documentation comes from the Go package doc comment — the comment block immediately before the `package` declaration.

```go
// Payments handles payment processing, billing,
// and subscription management.
package payments
```

Service docs appear in the Service Catalog and in generated API clients.

### Service structs

If you use a [service struct](/docs/go/primitives/service-structs), the doc comment on the struct type is also extracted.

```go
// PaymentService manages payment processing state and dependencies.
//
//encore:service
type PaymentService struct {
    // ...
}
```

## API endpoints

Add a comment block above the `//encore:api` directive to document an endpoint.

```go
// Charge charges the given payment method.
// It is idempotent; calling it multiple times
// with the same idempotency key has no additional effect.
//
//encore:api auth
func Charge(ctx context.Context, params *ChargeParams) (*ChargeResponse, error) {
    // ...
}
```

In the OpenAPI spec, the first line becomes the `summary` and the remaining lines become the `description`.

## Auth handlers

```go
// AuthenticateRequest validates the API key provided in the Authorization header
// and returns the authenticated user's information.
//
//encore:authhandler
func AuthenticateRequest(ctx context.Context, token string) (auth.UID, *UserData, error) {
    // ...
}
```

## Middleware

```go
// LogRequests logs all incoming requests and their duration.
//
//encore:middleware target=all
func LogRequests(req middleware.Request, next middleware.Next) middleware.Response {
    // ...
}
```

## Types

Document types with a comment above the type declaration.

```go
// ChargeParams defines the parameters for the Charge endpoint.
type ChargeParams struct {
    // PaymentMethodID is the identifier of the payment method to charge.
    PaymentMethodID string

    // Amount is the amount to charge, in the smallest currency unit (e.g. cents).
    Amount int

    Currency string // ISO 4217 currency code
}
```

Both the type-level and field-level comments are surfaced in generated clients and OpenAPI schemas.

## Infrastructure resources

Doc comments are also supported on infrastructure resource declarations.

```go
// TransactionDB stores payment transactions and related records.
var TransactionDB = sqldb.NewDatabase("transactions", sqldb.DatabaseConfig{
    Migrations: "./migrations",
})

// PaymentEvents publishes events when payment states change.
var PaymentEvents = pubsub.NewTopic[*PaymentEvent]("payment-events", pubsub.TopicConfig{
    DeliveryGuarantee: pubsub.AtLeastOnce,
})

// ProcessPaymentEvent handles incoming payment events.
var _ = pubsub.NewSubscription(PaymentEvents, "process-payment", pubsub.SubscriptionConfig[*PaymentEvent]{
    Handler: handlePaymentEvent,
})

// DailySettlement runs the settlement process every day at midnight.
var DailySettlement = cron.NewJob("daily-settlement", cron.JobConfig{
    Every:    24 * cron.Hour,
    Endpoint: Settle,
})

// PaymentCache caches frequently accessed payment records.
var PaymentCache = cache.NewCluster("payment-cache", cache.ClusterConfig{
    EvictionPolicy: cache.AllKeysLRU,
})

// PaymentByID caches payments by their unique identifier.
var PaymentByID = cache.NewStringKeyspace[Payment](PaymentCache, cache.KeyspaceConfig{
    KeyPattern: "payment/:id",
})

// Receipts stores generated receipt PDFs.
var Receipts = objects.NewBucket("receipts", objects.BucketConfig{})

// ChargeAmount tracks the total amount charged per currency.
var ChargeAmount = metrics.NewCounterGroup[ChargeLabels, uint64]("charge_amount", metrics.CounterConfig{})

type ChargeLabels struct {
    // Currency is the ISO 4217 currency code.
    Currency string
}
```

These docs appear in the Service Catalog and are available through the [MCP server](/docs/go/ai-integration#mcp-server) for AI-assisted development.

## Where docs appear

| Where | What's included |
|---|---|
| [Service Catalog](/docs/go/observability/service-catalog) | Services, endpoints, types, fields |
| [Generated clients](/docs/go/cli/client-generation) | Services, endpoints, types, fields |
| OpenAPI spec | Endpoints (summary + description), types, fields |
| [MCP server](/docs/go/ai-integration#mcp-server) | All resource types |
