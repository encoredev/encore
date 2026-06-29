---
seotitle: External Vaults – Reference secrets from external secret stores
seodesc: Learn how to connect external secret stores like GCP Secret Manager to Encore Cloud and reference their secrets from your application at runtime.
title: External Vaults
subtitle: Reference secrets stored in external secret stores instead of storing them in Encore
lang: platform
---

By default, Encore Cloud stores your application's [secrets](/docs/ts/primitives/secrets) in the Encore vault and copies them into your cloud provider's secret manager during deploys, so they're available to your application at runtime. **External vaults** let you go a step further and reference secrets that already live in an external secret store, so the secret value never has to be entered into or stored by Encore Cloud.

This is useful when secrets are owned by another team or system, managed by an existing rotation process, or required by policy to stay in a specific secret store.

<Callout type="info">

External vaults are an **Enterprise** feature. [Contact us](https://encore.cloud/book) to enable them for your organization.

</Callout>

## How it works

You connect a vault to your Encore app, pointing it at an external secret store. Individual [secrets](/docs/ts/primitives/secrets) can then be configured to **reference** a secret in that vault — by its name and version — instead of holding a value managed by Encore. At runtime, your application reads the value directly from the external store.

### Supported providers

| Provider               | Description                                              |
| ---------------------- | -------------------------------------------------------- |
| **GCP Secret Manager** | Reference secrets stored in Google Cloud Secret Manager. |

More providers may be added over time.

## Adding a vault

Vaults are configured at the **app level**. You need the **Admin** or **Member** (owner/writer) role, and the feature must be enabled for your organization.

1. Open your app and go to **Settings → Secrets**.
2. In the **External Vaults** section, click **Add External Vault**.
3. Choose a **provider** (e.g. GCP Secret Manager).
4. Give the vault a **name** to identify it.
5. Fill in the provider configuration (see below).
6. Click **Save**.

### GCP Secret Manager configuration

To connect a GCP Secret Manager vault you'll provide:

- **GCP Account** — the connected GCP service account Encore Cloud uses to access Secret Manager. If you don't have one yet, [connect a cloud](/docs/platform/deploy/own-cloud) first.
- **Project ID** — the GCP project that hosts the Secret Manager secrets.

You then need to grant the connected account access to the secrets. In the GCP project that holds the secrets:

1. Create a custom [IAM role][gcp-iam-roles] with the following permissions:
   - `secretmanager.secrets.getIamPolicy`
   - `secretmanager.secrets.setIamPolicy`
2. In [Secret Manager][gcp-secret-manager], grant that role to the connected GCP service account on the secrets you want to reference.

The dashboard shows the exact account email and permissions, with copy-to-clipboard helpers, while you're filling in the configuration.

Notice that the connected account only needs `getIamPolicy` and `setIamPolicy` — **not** access to the secret values themselves. Encore Cloud uses these permissions to grant your application's runtime service accounts access to the referenced secrets; it never needs to read the values itself.

## Preventing Encore from reading secret values

Because the connected account holds `setIamPolicy`, it could in principle grant itself (or another principal it controls) access to the secret values. If your security model requires that Encore Cloud is *able to manage access but never able to read the secrets*, you can enforce that with two [GCP IAM deny policies][gcp-deny].

Deny policies take precedence over allow policies, so they hold even if an `allow` binding is later added. Attach them at the project, folder, or organization level that contains the secrets.

### 1. Deny secret access outside your production projects

This deny policy blocks the `access` permission on Secret Manager for every principal **except** the service accounts that belong to your production project(s). Even if Encore's connected account granted itself an allow binding, this deny rule would override it — only your runtime workloads can read the values.

Replace `123456789012` with the numeric project ID of the project whose service accounts are allowed to read the secrets (add an `exceptionPrincipals` entry per production project):

```json
{
  "displayName": "Strict App Project Boundary for Secrets",
  "rules": [
    {
      "denyRule": {
        "deniedPrincipals": ["principalSet://goog/public:all"],
        "exceptionPrincipals": [
          "principalSet://cloudresourcemanager.googleapis.com/projects/123456789012/type/ServiceAccount"
        ],
        "deniedPermissions": ["secretmanager.googleapis.com/*.access"]
      }
    }
  ]
}
```

### 2. Block Encore from impersonating service accounts

The boundary above relies on only your production service accounts being able to read secrets. To prevent Encore's connected account from sidestepping it by impersonating one of those accounts, deny it the Service Account Credentials impersonation permissions.

Replace the `deniedPrincipals` entry with the principal identifier of your Encore connected/orchestrator service account:

```json
{
  "displayName": "Block Encore Orchestrator Impersonation Loophole",
  "rules": [
    {
      "denyRule": {
        "deniedPrincipals": ["principal://goog/subject/app-1esdad@..."],
        "deniedPermissions": [
          "iam.googleapis.com/serviceAccounts.getAccessToken",
          "iam.googleapis.com/serviceAccounts.getOpenIdToken",
          "iam.googleapis.com/serviceAccounts.signBlob",
          "iam.googleapis.com/serviceAccounts.signJwt",
          "iam.googleapis.com/serviceAccounts.implicitDelegation"
        ]
      }
    }
  ]
}
```

With both policies in place, Encore Cloud can still manage which runtime identities are allowed to read each secret, but cannot read the secret values itself — directly or via impersonation.

<Callout type="warning">

Make sure the `exceptionPrincipals` in the first policy cover every service account your application actually runs as, otherwise your workloads will be denied access to the secret values too.

</Callout>

## Referencing a vault secret

Once a vault is connected, you can point a secret at it instead of storing a value in Encore:

1. Go to **Settings → Secrets** and create or edit a secret.
2. When entering the value, choose the **vault** as the source.
3. Provide the **secret ID** (the secret's name in the external store) and the **version** to reference.

Your application keeps reading the secret [by name in code](/docs/ts/primitives/secrets) — only the source of the value changes. Different environments can reference different vaults or versions, just like regular secrets.

## Managing vaults

From the **External Vaults** section you can:

- **Rename** a vault.
- **Edit configuration** — change the GCP account or project ID.
- **Remove** a vault. Each vault shows which secrets currently reference it (its "used by" list), so you can see the impact before removing it.

## Related

- [Secrets](/docs/ts/primitives/secrets) — how secrets work in Encore
- [Application Security](/docs/platform/deploy/security) — how Encore Cloud secures your application by default
- [Connect your cloud account](/docs/platform/deploy/own-cloud) — connect the GCP account used to access Secret Manager

[gcp-iam-roles]: https://cloud.google.com/iam/docs/creating-custom-roles
[gcp-secret-manager]: https://cloud.google.com/security/products/secret-manager
[gcp-deny]: https://cloud.google.com/iam/docs/deny-overview
