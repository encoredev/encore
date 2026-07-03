---
seotitle: How to import an existing AWS S3 bucket into your Encore application
seodesc: Learn how to connect an existing AWS S3 bucket to your Encore application in place of an Encore-provisioned one.
title: Import an existing S3 bucket
subtitle: Connecting an object storage bucket to an existing AWS S3 bucket
lang: platform
---

# Overview

When deploying to your own cloud, Encore Cloud provisions the infrastructure backing your object storage buckets automatically. If you already have an AWS S3 bucket, you can instead connect an Encore bucket directly to that existing bucket.

<Callout type="important">

The Infrastructure page only lists resources after they've been provisioned. This means importing **replaces** a bucket that Encore has already provisioned — deploy your application first so the bucket appears, then import it to point at your existing S3 bucket.

</Callout>

## Benefits

Using an existing S3 bucket allows you to:
- Reuse a bucket and its existing objects without migrating data
- Keep buckets that are shared with systems outside of Encore
- Preserve existing configuration and access policies

## Prerequisites

- A [connected AWS account](/platform/infrastructure/aws) on the environment.
- The Encore IAM role must have permission to access the S3 bucket you want to import.
- A bucket declared in your application:

```typescript
import { Bucket } from "encore.dev/storage/objects";

export const profilePictures = new Bucket("profile-pictures");
```

```go
var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{})
```

## Importing an S3 bucket

1. Open the **Infrastructure** page for your environment in the [Encore Cloud dashboard](https://app.encore.cloud).
2. Locate the bucket resource, expand it, and click **Import** next to the bucket you want to connect.
3. In the dialog:
   - Select the **AWS Account** that owns the bucket.
   - Enter the **Region** the bucket resides in.
   - Enter the **Bucket Name**.
4. Click **Import**.

Because S3 ARNs don't include the region, the region is entered separately. The account is taken from the selected AWS integration.

Importing creates a proposed infrastructure change. Review it in the pending changes summary and click **Deploy** to apply it. On the next deploy, Encore Cloud switches your bucket to the existing S3 bucket, replacing the bucket it previously provisioned.

<Callout type="info">

If the resource can't be resolved, double-check the region and bucket name, and that Encore has permission to view the bucket in the selected AWS account.

</Callout>
