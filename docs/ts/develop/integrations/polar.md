---
seotitle: Using Polar with Encore.ts for Payments & Subscriptions
seodesc: Learn how to add payments, subscriptions, and license keys to your Encore.ts application using Polar as your Merchant of Record.
title: Polar
lang: ts
---

[Polar](https://polar.sh) handles payments, subscriptions, and license keys as your Merchant of Record. Combined with Encore, you get a full monetization stack with zero infrastructure to manage.

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

1. **Create an access token** — Go to Settings > [Developers > Personal Access Tokens](https://sandbox.polar.sh/settings/developers/pat) and create a new token.
2. **Create a product** — Go to [Products](https://sandbox.polar.sh/products) and create at least one product. Copy its **price ID** — you'll need it to create checkout sessions.
3. **Set up a webhook** (optional for local dev) — Go to Settings > [Webhooks](https://sandbox.polar.sh/settings/webhooks) and point it to your API URL followed by `/webhook/polar`. For local development, use a tunnel like [ngrok](https://ngrok.com) to expose your local server.

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

Create a file to configure the Polar SDK. Use Encore's [`secret()`](https://encore.dev/docs/ts/primitives/secrets) function to access the token, and `sandbox` for development / `production` when deployed:

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
  productPriceId: string;
}

interface CreateCheckoutResponse {
  checkoutUrl: string;
}

export const createCheckout = api(
  { auth: true, expose: true, method: "POST", path: "/checkout" },
  async (req: CreateCheckoutRequest): Promise<CreateCheckoutResponse> => {
    const authData = getAuthData()!;

    const session = await polar.checkouts.create({
      productPriceId: req.productPriceId,
      customerEmail: authData.email,
      successUrl: `${process.env.ENCORE_API_URL}/?success=true`,
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

When you deploy, Encore automatically provisions and manages the infrastructure your app needs. For Polar integrations, this includes:

- **Secrets** — encrypted per environment (preview, staging, production), never shared between them
- **Databases** — Cloud SQL on GCP, RDS on AWS
- **Networking** — TLS, load balancing, DNS

Your application code stays the same regardless of where you deploy.

### Self-hosting

Build a Docker image and deploy anywhere:

```shell
$ encore build docker my-app:latest
```

See [Self-hosting](https://encore.dev/docs/ts/self-host/build) for more details on building and deploying Docker images.

### Encore Cloud

Push your code and Encore handles the rest.

```shell
$ git push encore main
```

Start free on Encore Cloud, then connect your own AWS or GCP account when you're ready. Your application code stays exactly the same — Encore automatically provisions the right infrastructure in your cloud account, so there's nothing to rewrite or migrate. See [Connect your cloud account](https://encore.dev/docs/platform/deploy/own-cloud) for details.

## Related resources

- [Polar + Encore example app](https://github.com/encoredev/examples/tree/main/ts/polar)
- [Polar documentation](https://docs.polar.sh)
- [Polar sandbox dashboard](https://sandbox.polar.sh)
- [Encore secrets](https://encore.dev/docs/ts/primitives/secrets)
- [Raw endpoints](https://encore.dev/docs/ts/primitives/raw-endpoints)
