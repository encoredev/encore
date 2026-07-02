---
seotitle: Just-in-time Access – Time-bound privileges for deploys
seodesc: Learn how Encore Cloud uses GCP Privileged Access Manager (PAM) to grant temporary, time-bound permissions during deploys instead of holding standing access.
title: Just-in-time Access
subtitle: Grant temporary, time-bound deploy permissions instead of standing access
lang: platform
---

Just-in-time (JIT) access lets Encore Cloud request the permissions it needs for a deploy **only when they're needed, for a limited time**, instead of holding standing access to your cloud resources. When a deploy starts, Encore Cloud requests a temporary privilege grant, uses it to apply the change, and the grant automatically expires afterwards.

This minimizes the blast radius of long-lived credentials and gives you a full audit trail of every privileged action: what was requested, why, and for how long.

<Callout type="info">

Just-in-time access is an **Enterprise** feature. [Contact us](https://encore.dev/book) to enable it for your organization.

</Callout>

## Supported clouds

| Cloud   | Status                                                     |
| ------- | ---------------------------------------------------------- |
| **GCP** | Available, powered by GCP Privileged Access Manager (PAM). |
| **AWS** | In active development — [reach out](https://encore.dev/book) if you're interested. |

## GCP

On GCP, just-in-time access is powered by [Google Cloud Privileged Access Manager (PAM)][gcp-pam]. Instead of granting Encore Cloud's deploy identity permanent IAM roles, you define **entitlements** in GCP PAM that describe which roles can be requested and under what conditions.

### How it works

During a deploy, Encore Cloud:

1. Requests a grant for the configured entitlement, scoped to the duration it expects to need.
2. Waits for the grant to become active (subject to any approval policies you've configured in GCP PAM).
3. Performs the infrastructure change.
4. Lets the grant expire — access is no longer held once the deploy completes.

Because grants flow through GCP PAM, every request is recorded with its justification and duration, giving you a complete audit trail.

### Configuration

Just-in-time access is configured **per environment** in the Encore Cloud dashboard:

1. Open your app and go to **Environment Settings** for the environment you want to configure.
2. Find the **GCP Privileged Access Manager** section.

The section has three settings:

- **Grant duration** — how long, in minutes, each requested grant stays active.
- **Default entitlement** — the GCP PAM entitlement requested for any deploy phase that doesn't have its own override.
- **Per-phase overrides** — optional entitlements for specific deploy phases (see below).

#### Deploy phases

A deploy can involve different kinds of infrastructure changes, and you may want a different entitlement for each. You can override the entitlement on a per-phase basis:

| Phase         | When it applies                                   |
| ------------- | ------------------------------------------------- |
| **Provision** | Creating new infrastructure resources             |
| **Deploy**    | Rolling out new application versions              |
| **Delete**    | Removing infrastructure resources                 |

Any phase without an explicit override falls back to the **Default** entitlement. This lets you, for example, require a more privileged (or separately audited) entitlement for `Delete` while everything else uses a standard one.

#### Resource overrides

By default, every resource in an environment uses the environment-level configuration. When you need finer control — for example, a specific GCP project that requires a different entitlement — you can add a **resource override**.

Resource overrides accept the same entitlement settings as the environment default, including per-phase overrides. The **grant duration** is always inherited from the environment default and cannot be overridden per resource.

#### Configuration hierarchy

Settings are merged **most-specific-first**, so a more specific layer always wins:

```
Resource (project) override  ➜  Environment default  ➜  App default
```

For a given deploy phase, Encore Cloud resolves the entitlement by checking the resource override first, then the environment default, then the app-level default.

### Setting it up

1. In GCP, create the [PAM entitlements][gcp-pam] you want Encore Cloud to be able to request, granting the roles needed for deploys (and configuring any approval workflow you require).
2. In the Encore Cloud dashboard, open the environment's **Environment Settings** and configure the **GCP Privileged Access Manager** section with the entitlement name(s) and grant duration.
3. Trigger a deploy. Encore Cloud will request the grant just-in-time and you'll be able to see the activity in your GCP PAM audit logs.

## AWS

Just-in-time access for **AWS** is in active development.

If you're running on AWS and want time-bound deploy permissions, [reach out](https://encore.dev/book) — we'd love to hear about your use case and can let you know when AWS support is available.

## Related

- [Application Security](/docs/platform/deploy/security) — how Encore Cloud secures your application by default
- [GCP Infrastructure](/docs/platform/infrastructure/gcp) — how Encore Cloud provisions GCP infrastructure
- [Compliance](/docs/platform/management/compliance) — Encore's own use of time-limited privileged access

[gcp-pam]: https://cloud.google.com/iam/docs/pam-overview
