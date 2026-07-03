---
seotitle: How to import an existing GCP Cloud Storage bucket into your Encore application
seodesc: Learn how to connect an existing GCP Cloud Storage bucket to your Encore application in place of an Encore-provisioned one.
title: Import an existing Cloud Storage bucket
subtitle: Connecting an object storage bucket to an existing GCP Cloud Storage bucket
lang: platform
---

# Overview

When deploying to your own cloud, Encore Cloud provisions the infrastructure backing your object storage buckets automatically. If you already have a GCP Cloud Storage bucket, you can instead connect an Encore bucket directly to that existing bucket.

<Callout type="important">

The Infrastructure page only lists resources after they've been provisioned. This means importing **replaces** a bucket that Encore has already provisioned — deploy your application first so the bucket appears, then import it to point at your existing Cloud Storage bucket.

</Callout>

## Benefits

Using an existing Cloud Storage bucket allows you to:
- Reuse a bucket and its existing objects without migrating data
- Keep buckets that are shared with systems outside of Encore
- Preserve existing configuration and access policies

## Prerequisites

- A [connected GCP account](/platform/infrastructure/gcp) on the environment.
- The Encore service account must have permission to access the bucket you want to import.
- A bucket declared in your application:

```typescript
import { Bucket } from "encore.dev/storage/objects";

export const profilePictures = new Bucket("profile-pictures");
```

```go
var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{})
```

## Importing a Cloud Storage bucket

1. Open the **Infrastructure** page for your environment in the [Encore Cloud dashboard](https://app.encore.cloud).
2. Locate the bucket resource, expand it, and click **Import** next to the bucket you want to connect.
3. In the dialog:
   - Select the **GCP Account** that owns the bucket.
   - Enter the **Project ID** and **Bucket Name** of the existing bucket.
4. Click **Import**.

Importing creates a proposed infrastructure change. Review it in the pending changes summary and click **Deploy** to apply it. On the next deploy, Encore Cloud switches your bucket to the existing Cloud Storage bucket, replacing the bucket it previously provisioned.

<Callout type="info">

If the resource can't be resolved, double-check the project and bucket name, and that Encore has permission to view the bucket in the selected GCP account.

</Callout>
