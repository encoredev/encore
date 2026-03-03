---
seotitle: Using Polar with Encore.ts for Payments & Subscriptions
seodesc: Learn how to add payments, subscriptions, and license keys to your Encore.ts application using Polar as your Merchant of Record.
title: Polar
lang: ts
---

[Polar](https://polar.sh) handles payments, subscriptions, and license keys as your Merchant of Record. This guide shows how to integrate Polar with an Encore application.

To get started quickly, create a new app from the example:

```shell
$ encore app create --example=ts/polar
```

Or follow the steps below to add Polar to an existing Encore app.

<Callout type="info">

If you haven't installed Encore yet, see the [installation guide](https://encore.dev/docs/ts/install) first.

</Callout>

## Install the SDK

```shell
$ npm install @polar-sh/sdk
```

## Polar setup

Before writing code, you'll need to set up a few things in the [Polar dashboard](https://sandbox.polar.sh) (use the sandbox for development):

1. **Create an access token.** Go to Settings > [Developers > Personal Access Tokens](https://sandbox.polar.sh/settings/developers/pat) and create a new token.
2. **Create a product.** Go to [Products](https://sandbox.polar.sh/products) and create at least one product. Copy its **product ID**, you'll need it to create checkout sessions.
3. **Set up a webhook** (optional for local dev). Go to Settings > [Webhooks](https://sandbox.polar.sh/settings/webhooks) and point it to your API URL followed by `/webhooks/polar`. For local development, use a tunnel like [ngrok](https://ngrok.com) to expose your local server.

See the [Polar documentation](https://docs.polar.sh) for more details on products, pricing, and webhooks.

## Set your secrets

Store your Polar credentials as [Encore secrets](https://encore.dev/docs/ts/primitives/secrets):

```shell
$ encore secret set --type dev,local,pr,production PolarAccessToken
```

<Callout type="info">

Locally, secrets are stored on your machine and injected when you run `encore run`. No `.env` files needed.

</Callout>

## Initialize the client

Create a file to configure the Polar SDK. Use Encore's [`secret()`](https://encore.dev/docs/ts/primitives/secrets) function to access the token. Use `sandbox` for development and `production` when deployed:

```ts
-- polar.ts --
import { Polar } from "@polar-sh/sdk";
import { secret } from "encore.dev/config";

const polarAccessToken = secret("PolarAccessToken");

const server = process.env.ENCORE_ENVIRONMENT === "production"
  ? "production"
  : "sandbox";

export const polar = new Polar({
  accessToken: polarAccessToken(),
  server,
});
```

## Create a checkout

Use the Polar SDK to create checkout sessions for your products:

```ts
-- checkout.ts --
import { api } from "encore.dev/api";
import { polar } from "./polar";
import { getAuthData } from "~encore/auth";

interface CreateCheckoutRequest {
  productId: string;
}

interface CreateCheckoutResponse {
  checkoutUrl: string;
}

export const createCheckout = api(
  { auth: true, expose: true, method: "POST", path: "/checkout" },
  async (req: CreateCheckoutRequest): Promise<CreateCheckoutResponse> => {
    const authData = getAuthData()!;

    const baseUrl = process.env.ENCORE_API_URL || "http://localhost:4000";

    const session = await polar.checkouts.create({
      products: [req.productId],
      customerEmail: authData.email,
      successUrl: `${baseUrl}/?success=true`,
    });

    return { checkoutUrl: session.url || "" };
  }
);
```

## Handle webhooks

Create a [raw endpoint](https://encore.dev/docs/ts/primitives/raw-endpoints) to receive webhook events from Polar:

```ts
-- webhooks.ts --
import { api } from "encore.dev/api";
import log from "encore.dev/log";

export const handleWebhook = api.raw(
  { expose: true, path: "/webhooks/polar", method: "POST" },
  async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) {
      chunks.push(chunk);
    }
    const event = JSON.parse(Buffer.concat(chunks).toString());

    log.info("Received Polar webhook", { type: event.type });

    switch (event.type) {
      case "subscription.active":
        // Grant access to your product
        break;
      case "subscription.canceled":
        // Revoke access
        break;
      case "order.paid":
        // Fulfill the order
        break;
    }

    res.writeHead(200);
    res.end();
  }
);
```

Register your webhook URL in the [Polar dashboard](https://sandbox.polar.sh/settings/webhooks) under Settings > Webhooks. Use your Encore API URL followed by `/webhooks/polar`. Enable the events you want to handle (e.g. `subscription.active`, `subscription.canceled`, `order.paid`).

## Deploy

When you deploy, Encore automatically provisions and manages the infrastructure your app needs:

- **Secrets** encrypted per environment (preview, staging, production), never shared between them.
- **Databases** provisioned as Cloud SQL on GCP or RDS on AWS.
- **Networking** including TLS, load balancing, and DNS.

### Self-hosting

Build a Docker image and deploy anywhere:

```shell
$ encore build docker my-app:latest
```

See the [self-hosting docs](https://encore.dev/docs/ts/self-host/build) for more details.

### Encore Cloud

Deploy your application to a free staging environment in Encore's development cloud:

```shell
$ git push encore main
```

You can also connect your own AWS or GCP account and Encore will automatically provision the infrastructure and manage secrets in your cloud. See [Connect your cloud account](https://encore.dev/docs/platform/deploy/own-cloud) for details.

## Related resources

- [Polar + Encore example app](https://github.com/encoredev/examples/tree/main/ts/polar)
- [Polar documentation](https://docs.polar.sh)
- [Polar sandbox dashboard](https://sandbox.polar.sh)
- [Encore secrets](https://encore.dev/docs/ts/primitives/secrets)
- [Raw endpoints](https://encore.dev/docs/ts/primitives/raw-endpoints)
