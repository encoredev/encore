---
seotitle: Self-hosted Encore deployments
seodesc: Learn how to self-host your Encore applications, using Encore's open-source tooling.
title: Self-hosted Encore deployments
subtitle: Deploy your Encore application to your own infrastructure, using Encore's open-source tooling.
---

Encore supports building Docker images directly from the CLI, which can then be self-hosted on your own infrastructure of choice.

This can be a good choice for when Encore's cloud platform isn't a good fit for your use case, or if you want to [migrate away](/docs/how-to/migrate-away).

## Building your own Docker image

To build your own Docker image, use `encore eject docker MY-IMAGE:TAG` from the CLI.

This will compile your application using the host machine and then produce a Docker image containing the compiled application. The base image defaults to `scratch` for GO apps and `node:slim` for TS, but can be customized with `--base`.

This is exactly the same code path that Encore's CI system uses to build Docker images, ensuring compatibility.

## Configuring your Docker image

If you are using any infrastructure resources, such as SQL databases, Pub/Sub, or metrics, you will need to configure your Docker image with the necessary configuration.
The `eject` command let's you provide this by specifying a path to a config file using the `--config` flag.

The configuration file should be a JSON file using the [Encore Infra Config](https://encore.dev/schemas/infra.schema.json) schema.

This includes configuring things like:

- How to access infrastructure resources (what provider to use, what credentials to use, etc.)
- How to call other services over the network ("service discovery"),
  most notably their base URLs.
- Observability configuration (where to export metrics, etc.)
- Metadata about the environment the application is running in, to power Encore's [metadata APIs](/docs/develop/metadata).
- The values for any application-defined secrets.

This configuration is necessary for the application to behave correctly.

## Example

Here's an example configuration file you can use.

```json
{
  "$schema": "https://encore.dev/schemas/infra.schema.json",
  "app_id": "my-app",
  "env_name": "my-env",
  "env_type": "production",
  "cloud": "gcp",
  "sql_servers": [
    {
      "host": "my-db-host:5432",
      "databases": {
        "my-db": {
          "username": "my-db-owner",
          "password": {"$env": "DB_PASSWORD"}
        }
      }
    }
  ],
  "service_discovery": {
    "myservice": {
      "base_url": "https://my-service:8044"
    }
  },
  "redis": {
    "encoreredis": {
      "database_index": 0,
      "auth": {
        "type": "acl",
        "username": "encoreredis",
        "password": {"$env": "REDIS_PASSWORD"}
      },
      "host": "my-redis-host",
    }
  },
  "metrics": {
    "type": "prometheus",
    "remote_write_url": "https://my-remote-write-url"
  },
  "graceful_shutdown": {
    "total": 30
  },
  "auth": [
    {
      "type": "key",
      "id": 1,
      "key": {"$env": "SVC_TO_SVC_KEY"}
    }
  ],
  "secrets": {
    "AppSecret": {"$env": "APP_SECRET"}
  },
  "pubsub": [
    {
      "type": "gcp_pubsub",
      "project_id": "my-project",
      "topics": {
        "encore-topic": {
          "name": "gcp-topic-name",
          "subscriptions": {
            "encore-subscription": {
              "name": "gcp-subscription-name"
            }
          }
        }
      }
    }
  ]
}
```

## Configuring infrastructure

To use infrastructure resources, additional configuration must be added,
so that Encore is aware how to access each infrastructure resource.

See below for examples for each type of infrastructure resource.

### SQL Databases

First, for each SQL database server, add an entry to the `sql_servers` array:

```json
{
  "host": "127.0.0.1:5432",
  "server_ca_cert": "",
  "client_cert": "",
  "client_key": ""
}
```

If the server uses TLS with a non-system CA root, or requires a client certificate, specify the appropriate fields as PEM-encoded strings. Otherwise they can be left empty.

Next, add a database to the `sql_databases` array:

```json
{
  "server_id": 0,
  "encore_name": "blog",
  "database_name": "blog",
  "user": "my-database-username",
  "password": "my-database-password",
  "min_connections": 0,
  "max_connections": 100
}
```

This specifies that the database known in the Encore application as `blog`
can be accessed via server 0 (an index into the `sql_servers` array),
using the provided credentials and connection pool configuration.

The `database_name` field specifies what the database name is on the database
server, in cases where it differs from the `encore_name`.

Since the password is listed in the configuration, the runtime configuration
must itself be treated as sensitive, and stored as a secret.

### Pub/Sub

Pub/Sub similarly consists of two fields: `pubsub_providers` and `pubsub_topics`.

The providers specify which different kinds of Pub/Sub providers are in
use by the application. Encore currently supports:

- `nsq` for [NSQ](https://nsq.io/)
- `gcp` for [Google Cloud Pub/Sub](https://cloud.google.com/pubsub)
- `aws` for AWS [SNS](https://aws.amazon.com/sns/) + [SQS](https://aws.amazon.com/sqs/)
- `azure` for [Azure Service Bus](https://azure.microsoft.com/en-us/products/service-bus)

First, configure the necessary Pub/Sub providers by adding entries to the `pubsub_providers` array. Below is a sample configuration for all of the supported providers:

```json
"pubsub_providers": [
  {"nsq": {"host": "localhost:4150"}},
  {"gcp": {}},
  {"aws": {}},
  {"azure": {"namespace": "my-namespace"}}
]
```

As you see, some of the providers (AWS, GCP) require no additional configuration, while others (NSQ, Azure) do.

Once the providers are configured, Pub/Sub topics are configured as key-value pairs in the `pubsub_topics` field. For example:

```json
{
  "my-topic": {
    "provider_id": 0,
    "encore_name": "my-topic",
    "provider_name": "my-topic",
    "subscriptions": {
      "my-subscription": {
        "encore_name": "my-subscription",
        "provider_name": "my-subscription"
      }
    }
  }
}
```

This configures a single Pub/Sub topic that uses NSQ (since `provider_id: 0` corresponds to NSQ in the `pubsub_providers` array). The topic is named `my-topic`, and has a single subscription named `my-subscription`.

Like with SQL Databases, the `provider_name` can be set to a different name than the `encore_name` if necessary.

#### Google Cloud Pub/Sub

When using Google Cloud Pub/Sub, Encore supports additional configuration
options that must be set, so that Encore is aware of which GCP project
contains the resources.

It looks like this:

```json
{
  "my-topic": {
    "provider_id": 1,
    "encore_name": "my-topic",
    "provider_name": "my-topic",
    "gcp": { "project_id": "my-gcp-project-id" },
    "subscriptions": {
      "my-subscription": {
        "encore_name": "my-subscription",
        "provider_name": "my-subscription",
        "gcp": { "project_id": "my-gcp-project-id" }
      }
    }
  }
}
```

### Metrics

Similarly to cloud infrastructure resources, Encore supports configurable
metrics exports:

- Prometheus
- DataDog
- GCP Cloud Monitoring
- AWS CloudWatch
- Logs-based metrics

This is configured by setting the `metrics` field. Below are examples for each of the supported metrics providers:

#### Prometheus

```json
{
  "collection_interval": "60s",
  "prometheus": { "RemoteWriteURL": "http://prometheus.example.com/write" }
}
```

#### DataDog

```json
{
  "collection_interval": "60s",
  "datadog": { "Site": "datadoghq.com", "APIKey": "my-api-key" }
}
```

Since the API Key is listed in the configuration, the runtime configuration
must itself be treated as sensitive, and stored as a secret.

#### GCP Cloud Monitoring

```json
{
  "collection_interval": "60s",
  "gcp_cloud_monitoring": {
    "ProjectID": "my-gcp-project-id",
    "MonitoredResourceType": "generic_node",
    "MonitoredResourceLabels": {
				"project_id": "my-gcp-project-id"
				"location":   "us-central1",
				"namespace":  "my-namespace",
				"node_id":    "my-node-id"
			}
  }
}
```

See [GCP's documentation](https://cloud.google.com/monitoring/api/resources) for information
about configuring the `MonitoredResourceType` and `MonitoredResourceLabels` fields.

#### AWS CloudWatch

```json
{
  "collection_interval": "60s",
  "aws_cloud_watch": {
    "Namespace": "my-namespace"
  }
}
```

#### Logs-based metrics

```json
{
  "collection_interval": "60s",
  "logs_based": {}
}
```
