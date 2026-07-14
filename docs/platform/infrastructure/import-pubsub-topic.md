---
seotitle: How to import an existing GCP Pub/Sub topic into your Encore application
seodesc: Learn how to connect an existing GCP Pub/Sub topic to your Encore application in place of an Encore-provisioned one.
title: Import an existing Pub/Sub topic
subtitle: Connecting a Pub/Sub topic to an existing GCP Pub/Sub topic
lang: platform
---

# Overview

When deploying to your own cloud, Encore Cloud provisions the infrastructure backing your Pub/Sub topics automatically. If you already have a GCP Pub/Sub topic, you can instead connect an Encore Pub/Sub topic directly to that existing topic.

<Callout type="important">

The Infrastructure page only lists resources after they've been provisioned. This means importing **replaces** a topic that Encore has already provisioned — deploy your application first so the topic appears, then import it to point at your existing Pub/Sub topic.

</Callout>

## Benefits

Using an existing Pub/Sub topic allows you to:
- Reuse a topic that is shared with systems outside of Encore
- Preserve existing configuration and access policies
- Point Encore at a topic you already manage

## Prerequisites

- A [connected GCP account](/platform/infrastructure/gcp) on the environment.
- The Encore service account must have permission to access the Pub/Sub topic you want to import.
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

## Importing a Pub/Sub topic

1. Open the **Infrastructure** page for your environment in the [Encore Cloud dashboard](https://app.encore.dev).
2. Locate the Pub/Sub topic you want to connect and click **Import**.
3. In the dialog:
   - Select the **GCP Account** that owns the topic.
   - Enter the **Project ID** and **Topic ID** of the existing topic.
4. Click **Import**.

Importing creates a proposed infrastructure change. Review it in the pending changes summary and click **Deploy** to apply it. On the next deploy, Encore Cloud switches your Pub/Sub topic to the existing topic, replacing the topic it previously provisioned.

<Callout type="info">

If the resource can't be resolved, double-check the project and topic ID, and that Encore has permission to view the topic in the selected GCP account.

</Callout>
