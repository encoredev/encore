---
seotitle: Azure Infrastructure Config Reference — Encore Self-Hosting
seodesc: Reference documentation for Azure-specific infra config JSON fields used when self-hosting Encore on Azure
title: Azure Config Reference
subtitle: Runtime configuration fields for self-hosting Encore on Azure
lang: platform
---

This page is a reference for the Azure-specific fields in Encore's runtime infrastructure configuration JSON. These fields are used when **self-hosting** Encore on Azure — for example after running `encore eject` — and are not required when using Encore Cloud managed deployments.

For the overall structure of the infrastructure config see the [infrastructure configuration guide][infra-config].

---

## `AzureServiceBusProvider` — Pub/Sub

Configures [Azure Service Bus][az-servicebus] as the pub/sub backend. Set this inside a `PubsubProvider` entry in the runtime `pubsub_providers` array.

**Source:** `runtimes/go/appruntime/exported/config/config.go` → `AzureServiceBusProvider`  
**Proto:** `proto/encore/runtime/v1/infra.proto` → `PubSubProvider.AzureServiceBus` ✅ (implemented)

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `namespace` | `string` | ✅ | The fully-qualified Azure Service Bus namespace hostname, e.g. `my-namespace.servicebus.windows.net`. |

Authentication uses **DefaultAzureCredential** — managed identity in production, Azure CLI or environment credentials locally.

### Example

```json
{
  "pubsub_providers": [
    {
      "azure": {
        "namespace": "my-namespace.servicebus.windows.net"
      }
    }
  ],
  "pubsub_topics": {
    "user-events": {
      "encore_name": "user-events",
      "provider_id": 0,
      "provider_name": "user-events",
      "subscriptions": {
        "email-service": {
          "id": "email-service",
          "encore_name": "email-service",
          "provider_name": "user-events~email-service"
        }
      }
    }
  }
}
```

> **Tip:** The `provider_name` for a topic maps to the Azure Service Bus **topic** name, and the subscription's `provider_name` maps to the **subscription** name within that topic (Azure Service Bus subscription names conventionally use the `<topic>~<subscription>` pattern, but the exact names are whatever you provision in Azure).

---

## `AzureBlobBucketProvider` — Object Storage

Configures [Azure Blob Storage][az-blob] as the object storage backend. Set this inside a `BucketProvider` entry in the runtime `bucket_providers` array.

**Source:** `runtimes/go/appruntime/exported/config/config.go` → `AzureBlobBucketProvider`  
**Proto:** `proto/encore/runtime/v1/infra.proto` → `BucketProvider.AzBlob` ✅ (implemented)

> ✅ **Proto gap resolved:** The `AzBlob` message exists in `infra.proto` and the Go parsing layer that maps `BucketCluster.az_blob` → `config.Runtime.BucketProviders[].AzureBlob` has now been implemented (fixed this sprint by Neo). Self-hosted deployments using `infra.proto` can now activate Azure Blob Storage.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `storage_account` | `string` | ✅ | The name of the Azure storage account (e.g. `myappstgprod`). |
| `connection_string` | `string \| null` | ☐ | Full Azure Blob Storage connection string. When set it takes precedence over `storage_account` + `storage_key`. The account name and key embedded in the string are also used for SAS URL generation. |
| `storage_key` | `string \| null` | ☐ | Azure storage account key for SharedKey authentication. Required if you need to generate signed (SAS) URLs. If both `connection_string` and `storage_key` are omitted, **DefaultAzureCredential** (managed identity) is used for authentication. |

> **Note:** In production on AKS or Container Apps with managed identity, omit both `connection_string` and `storage_key`. The runtime will authenticate using the pod/container's managed identity, which should be granted `Storage Blob Data Contributor` (or `Reader`) on the relevant containers.

### Example — Managed Identity (recommended for production)

```json
{
  "bucket_providers": [
    {
      "azure_blob": {
        "storage_account": "myappstgprod"
      }
    }
  ],
  "buckets": {
    "profile-images": {
      "cluster_id": 0,
      "encore_name": "profile-images",
      "cloud_name": "profile-images-a1b2c3",
      "key_prefix": "",
      "public_base_url": "https://myappstgprod.blob.core.windows.net/profile-images-a1b2c3"
    }
  }
}
```

### Example — Explicit Storage Key

```json
{
  "bucket_providers": [
    {
      "azure_blob": {
        "storage_account": "myappstgprod",
        "storage_key": "base64encodedkey=="
      }
    }
  ],
  "buckets": {
    "uploads": {
      "cluster_id": 0,
      "encore_name": "uploads",
      "cloud_name": "uploads-d4e5f6",
      "key_prefix": ""
    }
  }
}
```

---

## `AzureMonitorMetricsProvider` — Metrics

Configures [Azure Monitor custom metrics][az-monitor-custom] as the metrics export backend. Set this as the `azure_monitor` field on the `Metrics` object in the runtime config.

**Source:** `runtimes/go/appruntime/exported/config/config.go` → `AzureMonitorMetricsProvider`  
**Proto:** `proto/encore/runtime/v1/runtime.proto` → `MetricsProvider.AzureMonitor` ✅ (implemented)

> ⚠️ **Proto gap:** `MetricsProvider.AzureMonitor` is not yet defined in `proto/encore/runtime/v1/infra.proto` — it is being added this sprint by The Keymaker. Until that change ships, Azure Monitor metrics config is only available via the `runtime.proto` path (Encore Cloud hosted deployments). For self-hosted deployments, configure metrics via the `runtime.proto` `MetricsProvider.AzureMonitor` message directly rather than through `infra.proto`.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `location` | `string` | ✅ | Azure region of the target resource (e.g. `eastus`, `westeurope`). |
| `subscription_id` | `string` | ✅ | Azure subscription ID that owns the resource. |
| `resource_group` | `string` | ✅ | Resource group that contains the target resource. |
| `resource_namespace` | `string` | ✅ | Resource provider namespace and type, e.g. `Microsoft.ContainerService/managedClusters` or `Microsoft.App/containerApps`. |
| `resource_name` | `string` | ✅ | Name of the target Azure resource (the AKS cluster name, Container App name, etc.). |
| `namespace` | `string` | ✅ | Custom metrics namespace that Encore will write to in Azure Monitor (e.g. `Encore/App`). |

Authentication uses **DefaultAzureCredential**. In production the managed identity must be granted the `Monitoring Metrics Publisher` role on the target resource.

### Example

```json
{
  "metrics": {
    "collection_interval": 15000000000,
    "azure_monitor": {
      "location": "eastus",
      "subscription_id": "00000000-0000-0000-0000-000000000000",
      "resource_group": "my-app-prod-rg",
      "resource_namespace": "Microsoft.ContainerService/managedClusters",
      "resource_name": "my-app-prod-aks",
      "namespace": "Encore/App"
    }
  }
}
```

> **Note:** `collection_interval` is expressed in nanoseconds. `15000000000` = 15 seconds.

---

## `AzureKeyVaultSecretsProvider` — Secrets

Configures [Azure Key Vault][az-keyvault] as the remote secrets backend. Set this as the `secrets_provider.azure_key_vault` field in the **InfraConfig** (the self-hosting configuration JSON, distinct from the runtime config).

**Source:** `runtimes/go/appruntime/exported/config/infra/config.go` → `AzureKeyVaultSecretsProvider`  
**Proto:** Not yet present in `infra.proto` — configured exclusively via the JSON InfraConfig. A proto definition will be added in a future release.

> ⚠️ **Proto gap:** `SecretsProvider` (and `AzureKeyVaultSecretsProvider`) are not yet defined in `proto/encore/runtime/v1/infra.proto`. Until that is resolved, the Key Vault secrets provider is only available through the JSON-based `InfraConfig` used in the self-hosting / eject flow. Encore Cloud managed deployments configure secrets automatically.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `vault_url` | `string` | ✅ | Base URL of the Azure Key Vault, e.g. `https://my-vault.vault.azure.net/`. |

Secret names in the Encore application map **directly** to secret names in the Key Vault. Authentication uses **DefaultAzureCredential** — managed identity in production (the identity must be granted `Key Vault Secrets User` on the vault), Azure CLI credentials locally.

### Example (InfraConfig)

```json
{
  "metadata": {
    "app_id": "my-app",
    "env_name": "production",
    "env_type": "production",
    "cloud": "azure",
    "base_url": "https://api.my-app.example.com"
  },
  "secrets_provider": {
    "azure_key_vault": {
      "vault_url": "https://my-app-prod-kv.vault.azure.net/"
    }
  }
}
```

---

## `AzureMetadata` — IMDS Collector

When an Encore application starts on Azure, the runtime automatically queries the [Azure Instance Metadata Service (IMDS)][az-imds] at `http://169.254.169.254/metadata/instance?api-version=2021-02-01` to enrich traces and logs with cloud context.

**Source:** `runtimes/go/appruntime/infrasdk/metadata/azure_collector.go`  
**Proto:** Not a configurable field — the collector is registered automatically when `env_cloud` is `"azure"`.

### Fields collected from IMDS

| IMDS field | Mapped to | Notes |
|---|---|---|
| `compute.location` | Azure region (e.g. `eastus`) | Used for metrics and tracing context |
| `compute.resourceGroupName` | `ServiceID` in container metadata | Closest equivalent to an ECS service boundary |
| `compute.vmId` | `InstanceID` (last 8 chars) | Unique instance identifier for tracing |
| `compute.name` | VM / node name | Available but not currently surfaced in traces |
| `compute.subscriptionId` | Subscription context | Available but not currently surfaced in traces |

> **Note:** The IMDS endpoint is only reachable from within an Azure VM or container. Outside of Azure the collector returns empty metadata gracefully — it does not fail startup.

### Enabling the IMDS collector

No configuration is required. Set `cloud` to `"azure"` in the `metadata` block of your `InfraConfig` and the collector activates automatically:

```json
{
  "metadata": {
    "app_id": "my-app",
    "env_name": "production",
    "env_type": "production",
    "cloud": "azure"
  }
}
```

To **disable** the Azure IMDS collector at compile time (e.g. to reduce binary size in a non-Azure deployment), build your application with the `encore_no_azure` build tag:

```bash
go build -tags encore_no_azure ./...
```

---

## Full Self-Hosting Example

The following shows a complete `InfraConfig` JSON for a self-hosted Encore app on Azure using all four Azure providers:

```json
{
  "metadata": {
    "app_id": "my-app",
    "env_name": "production",
    "env_type": "production",
    "cloud": "azure",
    "base_url": "https://api.my-app.example.com"
  },
  "secrets_provider": {
    "azure_key_vault": {
      "vault_url": "https://my-app-prod-kv.vault.azure.net/"
    }
  },
  "metrics": {
    "collection_interval": 15000000000,
    "azure_monitor": {
      "location": "eastus",
      "subscription_id": "00000000-0000-0000-0000-000000000000",
      "resource_group": "my-app-prod-rg",
      "resource_namespace": "Microsoft.ContainerService/managedClusters",
      "resource_name": "my-app-prod-aks",
      "namespace": "Encore/App"
    }
  },
  "sql_servers": [
    {
      "host": "my-app-prod-pg.postgres.database.azure.com:5432",
      "tls_config": {
        "disable_tls_hostname_verification": false
      },
      "databases": {
        "users": {
          "name": "users",
          "username": { "value": "encore_users" },
          "password": { "$env": "DB_USERS_PASSWORD" }
        }
      }
    }
  ],
  "redis": {
    "sessions": {
      "host": "my-app-prod-redis.redis.cache.windows.net:6380",
      "database_index": 0,
      "auth": {
        "type": "auth_string",
        "auth_string": { "$env": "REDIS_AUTH_STRING" }
      },
      "tls_config": {}
    }
  },
  "pubsub": [
    {
      "type": "azure_service_bus",
      "azure_service_bus": {
        "namespace": "my-app-prod-sb.servicebus.windows.net",
        "topics": {
          "user-events": {
            "name": "user-events",
            "subscriptions": {
              "email-service": {
                "name": "user-events~email-service"
              }
            }
          }
        }
      }
    }
  ],
  "object_storage": [
    {
      "type": "azure_blob",
      "storage_account": "myappprodstg",
      "buckets": {
        "profile-images": {
          "name": "profile-images-a1b2c3"
        }
      }
    }
  ]
}
```

[infra-config]: /docs/platform/infrastructure/configuration
[az-servicebus]: https://learn.microsoft.com/en-us/azure/service-bus-messaging/service-bus-messaging-overview
[az-blob]: https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction
[az-monitor-custom]: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-custom-overview
[az-keyvault]: https://learn.microsoft.com/en-us/azure/key-vault/general/overview
[az-imds]: https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
