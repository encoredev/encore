---
seotitle: How to import an existing AWS SNS topic into your Encore application
seodesc: Learn how to connect an existing AWS SNS topic to your Encore application in place of an Encore-provisioned one.
title: Import an existing SNS topic
subtitle: Connecting a Pub/Sub topic to an existing AWS SNS topic
lang: platform
---

# Overview

When deploying to your own cloud, Encore Cloud provisions the infrastructure backing your Pub/Sub topics automatically. If you already have an AWS SNS topic, you can instead connect an Encore Pub/Sub topic directly to that existing topic.

<Callout type="important">

The Infrastructure page only lists resources after they've been provisioned. This means importing **replaces** a topic that Encore has already provisioned — deploy your application first so the topic appears, then import it to point at your existing SNS topic.

</Callout>

## Benefits

Using an existing SNS topic allows you to:
- Reuse a topic that is shared with systems outside of Encore
- Preserve existing configuration and access policies
- Point Encore at a topic you already manage

## Prerequisites

- A [connected AWS account](/platform/infrastructure/aws) on the environment.
- The Encore IAM role must have permission to access the SNS topic you want to import.
- A Pub/Sub topic declared in your application:

```typescript
import { Topic } from "encore.dev/pubsub";

export const signups = new Topic<SignupEvent>("signups", {
  deliveryGuarantee: "at-least-once",
});
```

```go
var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
```

## Importing an SNS topic

1. Open the **Infrastructure** page for your environment in the [Encore Cloud dashboard](https://app.encore.dev).
2. Locate the Pub/Sub topic you want to connect and click **Import**.
3. In the dialog:
   - Select the **AWS Account** that owns the topic.
   - Enter the **Topic ARN** in the form `arn:aws:sns:<region>:<account>:<topic>`.
4. Click **Import**.

The account ID in the ARN must match the account ID of the selected AWS integration.

Importing creates a proposed infrastructure change. Review it in the pending changes summary and click **Deploy** to apply it. On the next deploy, Encore Cloud switches your Pub/Sub topic to the existing SNS topic, replacing the topic it previously provisioned.

<Callout type="info">

If the resource can't be resolved, double-check that the ARN is correct and that Encore has permission to view the topic in the selected AWS account.

</Callout>
