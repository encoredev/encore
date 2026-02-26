---
seotitle: Using Resend with Encore.ts for Transactional Email
seodesc: Learn how to send transactional emails from your Encore.ts application using Resend, with async delivery via Pub/Sub and built-in observability.
title: Resend
lang: ts
---

[Resend](https://resend.com) provides transactional email with high deliverability and React Email templates. Combined with Encore's [Pub/Sub](https://encore.dev/docs/ts/primitives/pubsub) and [secrets management](https://encore.dev/docs/ts/primitives/secrets), you get reliable email delivery without managing any infrastructure.

To get started quickly, create a new app from the example:

```shell
$ encore app create --example=ts/resend
```

Or follow the steps below to add Resend to an existing Encore app.

<Callout type="info">

If you haven't installed Encore yet, see the [installation guide](https://encore.dev/docs/ts/install) first.

</Callout>

## Install the SDK

```shell
$ npm install resend
```

If you want to use React Email templates:

```shell
$ npm install resend @react-email/components
```

## Resend setup

Before writing code, you'll need to configure a few things in the [Resend dashboard](https://resend.com):

1. **Create an API key** — Go to [API Keys](https://resend.com/api-keys) and create a new key.
2. **Verify a domain** (optional for testing) — Go to [Domains](https://resend.com/domains) and add your sending domain. Until you verify a domain, you can use `onboarding@resend.dev` as the `from` address for testing.

See the [Resend documentation](https://resend.com/docs) for more details on domain verification and sending limits.

## Set your API key

Store your Resend API key as an [Encore secret](https://encore.dev/docs/ts/primitives/secrets):

```shell
$ encore secret set --type dev,local,pr,production ResendApiKey
```

<Callout type="info">

Locally, secrets are stored on your machine and injected when you run `encore run`. No `.env` files needed.

</Callout>

## Initialize the client

```ts
-- resend.ts --
import { Resend } from "resend";
import { secret } from "encore.dev/config";

const resendApiKey = secret("ResendApiKey");

export const resend = new Resend(resendApiKey());
```

## Send an email

Use the Resend SDK in an Encore API endpoint:

```ts
-- send.ts --
import { api } from "encore.dev/api";
import { resend } from "./resend";

interface SendEmailRequest {
  to: string;
  subject: string;
  html: string;
}

interface SendEmailResponse {
  id: string;
}

export const sendEmail = api(
  { expose: true, method: "POST", path: "/email/send" },
  async (req: SendEmailRequest): Promise<SendEmailResponse> => {
    const { data, error } = await resend.emails.send({
      from: "Your App <hello@yourdomain.com>",
      to: req.to,
      subject: req.subject,
      html: req.html,
    });

    if (error) {
      throw new Error(`Failed to send email: ${error.message}`);
    }

    return { id: data!.id };
  }
);
```

<Callout type="info">

The `from` address must use a domain you've verified in [Resend](https://resend.com/domains). For testing, you can use `onboarding@resend.dev` which works with any API key.

</Callout>

## Async delivery with Pub/Sub

For better performance, send emails asynchronously using Encore's [Pub/Sub](https://encore.dev/docs/ts/primitives/pubsub). This keeps your API endpoints fast and handles retries automatically:

```ts
-- topic.ts --
import { Topic, Subscription } from "encore.dev/pubsub";
import { resend } from "./resend";

interface EmailEvent {
  to: string;
  subject: string;
  html: string;
}

export const emailTopic = new Topic<EmailEvent>("email-send", {
  deliveryGuarantee: "at-least-once",
});

const _ = new Subscription(emailTopic, "send-via-resend", {
  handler: async (event) => {
    const { error } = await resend.emails.send({
      from: "Your App <hello@yourdomain.com>",
      to: event.to,
      subject: event.subject,
      html: event.html,
    });

    if (error) {
      throw new Error(error.message);
    }
  },
});
```

Then publish from any endpoint:

```ts
import { emailTopic } from "./topic";

// Inside any API endpoint
await emailTopic.publish({
  to: "user@example.com",
  subject: "Welcome!",
  html: "<p>Thanks for signing up.</p>",
});
```

<Callout type="info">

Locally, Pub/Sub runs in-process — messages are delivered immediately, making it easy to test and debug.

</Callout>

## Deploy

When you deploy, Encore automatically provisions and manages the infrastructure your app needs. For Resend integrations, this includes:

- **Secrets** — encrypted per environment (preview, staging, production), never shared between them
- **Pub/Sub** — GCP Pub/Sub on Google Cloud, SQS/SNS on AWS, with automatic retries and dead-letter queues
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

- [Resend + Encore example app](https://github.com/encoredev/examples/tree/main/ts/resend)
- [Resend documentation](https://resend.com/docs)
- [Encore Pub/Sub](https://encore.dev/docs/ts/primitives/pubsub)
- [Encore secrets](https://encore.dev/docs/ts/primitives/secrets)
