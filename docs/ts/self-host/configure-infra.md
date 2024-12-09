---
title: Configure Infrastructure
seotitle: Configure Infrastructure
seodesc: Learn how to configure infrastructure resources for your Encore app.
lang: ts
---

If you are using infrastructure resources, such as SQL databases, Pub/Sub, or metrics, you will need to configure your Docker image with the necessary configuration.
The `build` command lets you provide this by specifying a path to a config file using the `--config` flag.

```bash
encore build docker --config path/to/infra-config.json MY-IMAGE:TAG
```

The configuration file should be a JSON file using the [Encore Infra Config](https://encore.dev/schemas/infra.schema.json) schema.

This supports configuring things like:

- How to access infrastructure resources (what provider to use, what credentials to use, etc.)
- How to call other services over the network ("service discovery"),
  most notably their base URLs.
- Observability configuration (where to export metrics, etc.)
- Metadata about the environment the application is running in, to power Encore's metadata APIs.
- The values for any application-defined secrets.

This configuration is necessary for the application to behave correctly.

## Example

Here's an example configuration file you can use.

```json
{
  "$schema": "https://encore.dev/schemas/infra.schema.json",
  "metadata": {
    "app_id": "my-app",
    "env_name": "my-env",
    "env_type": "production",
    "cloud": "gcp",
    "base_url": "https://my-app.com"
  },
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
      "base_url": "https://myservice:8044"
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
  ],
  "object_storage": [
    {
      "type": "gcs",
      "buckets": {
          "my-gcs-bucket": {
            "name": "my-gcs-bucket",
          }
        }
    }
  ]
}
```

## Configuring Infrastructure
To use infrastructure resources, additional configuration must be added so that Encore is aware of how to access each infrastructure resource.
See below for examples of each type of infrastructure resource.

### 1. Basic Environment Metadata Configuration

```json
{
  "metadata": {
    "app_id": "my-encore-app",
    "env_name": "production",
    "env_type": "production",
    "cloud": "aws",
    "base_url": "https://api.myencoreapp.com"
  }
}
```

- `app_id`: The ID of your Encore application.
- `env_name`: The environment name, such as `production`, `staging`, or `development`.
- `env_type`: Specifies the type of environment (`production`, `test`, `development`, or `ephemeral`).
- `cloud`: The cloud provider hosting the infrastructure (e.g., `aws`, `gcp`, or `azure`).
- `base_url`: The base URL for services in the environment.

### 2. Graceful Shutdown Configuration

```json
{
  "graceful_shutdown": {
    "total": 30,
    "shutdown_hooks": 10,
    "handlers": 20
  }
}
```

- `total`: The total time allowed for the shutdown process in seconds.
- `shutdown_hooks`: The time allowed for executing shutdown hooks.
- `handlers`: The time allocated for processing request handlers during the shutdown.

### 3. Authentication Methods Configuration
Private endpoints will not require authentication if no authentication methods are specified. This is typically fine when services are deployed on a private network such as a VPC. But sometimes you might need to connect to other services over the public internet, in which case you'll want to ensure private endpoints are only accessible to other backend services. To do that you can configure authentication methods.
Encore currently supports authentication through a shared key, which you can specify in your infrastructure configuration file.
```json
{
  "auth": [
    {
      "type": "key",
      "id": 1,
      "key": {
        "$env": "SERVICE_API_KEY"
      }
    }
  ]
}
```

- `type`: The authentication method type (e.g., `key`).
- `id`: The ID associated with the authentication method.
- `key`: The authentication key, which can be set using an environment variable reference.

### 4. Service Discovery Configuration
Service discovery is used to access other services over the network. You can configure service discovery in the infrastructure configuration file.
If you export all services into the same docker image, you don't need to configure service discovery as it will be automatically
configured when the services are started.

```json
{
  "service_discovery": {
    "user-service": {
      "base_url": "https://user.myencoreapp.com",
      "auth": [
        {
          "type": "key",
          "id": 1,
          "key": {
            "$env": "USER_SERVICE_API_KEY"
          }
        }
      ]
    }
  }
}
```

- `user-service`: Configuration for a service named `user-service`.
- `base_url`: The base URL for the service.
- `auth`: Authentication methods used for accessing the service. If no authentication methods are specified, the service will use the auth methods defined in the `auth` section.

### 5. Metrics Configuration
Similarly to cloud infrastructure resources, Encore supports configurable metrics exports:

* Prometheus
* DataDog
* GCP Cloud Monitoring
* AWS CloudWatch

This is configured by setting the metrics field. Below are examples for each of the supported metrics providers:
#### 5.1. Prometheus Configuration

```json
{
  "metrics": {
    "type": "prometheus",
    "collection_interval": 15,
    "remote_write_url": {
      "$env": "PROMETHEUS_REMOTE_WRITE_URL"
    }
  }
}
```

#### 5.2. Datadog Configuration

```json
{
  "metrics": {
    "type": "datadog",
    "collection_interval": 30,
    "site": "datadoghq.com",
    "api_key": {
      "$env": "DATADOG_API_KEY"
    }
  }
}
```

#### 5.3. GCP Cloud Monitoring Configuration

```json
{
  "metrics": {
    "type": "gcp_cloud_monitoring",
    "collection_interval": 60,
    "project_id": "my-gcp-project",
    "monitored_resource_type": "gce_instance",
    "monitored_resource_labels": {
      "instance_id": "1234567890",
      "zone": "us-central1-a"
    },
    "metric_names": {
      "cpu_usage": "compute.googleapis.com/instance/cpu/usage_time"
    }
  }
}
```

#### 5.4. AWS CloudWatch Configuration

```json
{
  "metrics": {
    "type": "aws_cloudwatch",
    "collection_interval": 60,
    "namespace": "MyAppMetrics"
  }
}
```

### 6. SQL Database Configuration
The SQL databases you've declared in your Encore app must be configured in the infrastructure configuration file.
There must be exactly one database configuration for each declared database. You can configure multiple SQL servers if needed.

```json
{
  "sql_servers": [
    {
      "host": "db.myencoreapp.com:5432",
      "tls_config": {
        "disabled": false,
        "ca": "---BEGIN CERTIFICATE---\n..."
      },
      "databases": {
        "main_db": {
          "max_connections": 100,
          "min_connections": 10,
          "username": "db_user",
          "password": {
            "$env": "DB_PASSWORD"
          }
        }
      }
    }
  ]
}
```

- `host`: SQL server host, optionally including the port.
- `tls_config`: TLS configuration for secure connections. If the server uses TLS with a non-system CA root, or requires a client certificate, specify the appropriate fields as PEM-encoded strings. Otherwise, they can be left empty.
- `databases`: List of databases, each with connection settings.

### 7. Secrets Configuration

#### 7.1. Using Direct Secrets
You can set the secret value directly in the configuration file, or use an environment variable reference to set the secret value.

```json
{
  "secrets": {
    "API_TOKEN": "embedded-secret-value",
    "DB_PASSWORD": {
      "$env": "DB_PASSWORD"
    }
  }
}
```

#### 7.2. Using Environment Reference
As an alternative, you can use an environment variable reference to set the secret value. The env variable should be set in the environment where the application is running. The content
of the environment variable should be a JSON string where each key is the secret name and the value is the secret value.

```json
{
  "secrets": {
    "$env": "SECRET_JSON"
  }
}
```

### 8. Redis Configuration

```json
{
  "redis": {
    "cache": {
      "host": "redis.myencoreapp.com:6379",
      "database_index": 0,
      "auth": {
        "type": "auth",
        "auth_string": {
          "$env": "REDIS_AUTH_STRING"
        }
      },
      "max_connections": 50,
      "min_connections": 5
    }
  }
}
```

- `host`: Redis server host, optionally including the port.
- `auth`: Authentication configuration for the Redis server.
- `key_prefix`: Prefix applied to all keys.

### 9. Pub/Sub Configuration
Encore currently supports the following Pub/Sub providers:
- `nsq` for [NSQ](https://nsq.io/)
- `gcp` for [Google Cloud Pub/Sub](https://cloud.google.com/pubsub)
- `aws` for AWS [SNS](https://aws.amazon.com/sns/) + [SQS](https://aws.amazon.com/sqs/)
- `azure` for [Azure Service Bus](https://azure.microsoft.com/en-us/products/service-bus)

The configuration for each provider is different. Below are examples for each provider.
#### 9.1. GCP Pub/Sub

```json
{
  "pubsub": [
    {
      "type": "gcp_pubsub",
      "project_id": "my-gcp-project",
      "topics": {
        "user-events": {
          "name": "user-events-topic",
          "project_id": "my-gcp-project",
          "subscriptions": {
            "user-notification": {
              "name": "user-notification-subscription",
              "push_config": {
                "id": "user-push",
                "service_account": "service-account@my-gcp-project.iam.gserviceaccount.com"
              }
            }
          }
        }
      }
    }
  ]
}
```

#### 9.2. AWS SNS/SQS

```json
{
  "pubsub": [
    {
      "type": "aws_sns_sqs",
      "topics": {
        "user-notifications": {
          "arn": "arn:aws:sns:us-east-1:123456789012:user-notifications",
          "subscriptions": {
            "user-queue": {
              "arn": "arn:aws:sqs:us-east-1:123456789012:user-queue"
            }
          }
        }
      }
    }
  ]
}
```

#### 9.3. NSQ Configuration

```json
{
  "pubsub": [
    {
      "type": "nsq",
      "hosts": "nsq.myencoreapp.com:4150",
      "topics": {
        "order-events": {
          "name": "order-events-topic",
          "subscriptions": {
            "order-processor": {
              "name": "order-processor-subscription"
            }
          }
        }
      }
    }
  ]
}
```

### 10. Object Storage Configuration
Encore currently supports the following object storage providers:
- `gcs` for [Google Cloud Storage](https://cloud.google.com/storage)
- `s3` for [AWS S3](https://aws.amazon.com/s3/) or a custom S3-compatible provider

#### 10.1. GCS Configuration

```json
{
  "object_storage": [
    {
      "type": "gcs",
      "buckets": {
        "my-gcs-bucket": {
          "name": "my-gcs-bucket",
          "key_prefix": "my-optional-prefix/",
          "public_base_url": "https://my-gcs-bucket-cdn.example.com/my-optional-prefix"
        }
      }
    }
  ]
}
```

- `name`: The full name of the GCS bucket.
- `key_prefix`: An optional prefix to apply to all keys in the bucket.
- `public_base_url`: A URL to use for public access to the bucket. This field is required if you configure your bucket to be public. Encore will append the object key to this URL when generating public URLs. The optional prefix will not be appended.

#### 10.2. S3 Configuration

```json
{
  "object_storage": [
    {
      "type": "s3",
      "region": "us-east-1",
      "buckets": {
        "my-s3-bucket": {
          "name": "my-s3-bucket",
          "key_prefix": "my-optional-prefix/",
          "public_base_url": "https://my-gcs-bucket-cdn.example.com/my-optional-prefix"
        }
      }
    }
  ]
}
```

- `region`: The AWS region where the bucket is located.
- `name`: The full name of the S3 bucket.
- `key_prefix`: An optional prefix to apply to all keys in the bucket.
- `public_base_url`: A URL to use for public access to the bucket. This field is required if you configure your bucket to be public. Encore will append the object key to this URL when generating public URLs. The optional prefix will not be appended.

#### 10.3. Custom S3 Provider Configuration
You can also configure a custom S3 provider by specifying the endpoint, access key id, and secret access key. Custom S3 providers are useful if you are using a S3-compatible storage provider such as [Cloudflare R2](https://developers.cloudflare.com/r2/).
```json
{
  "object_storage": [
    {
      "type": "s3",
      "region": "auto",
      "endpoint": "https://...",
      "access_key_id": "...",
      "secret_access_key": {
          "$env": "BUCKET_SECRET_ACCESS_KEY"
      },
      "buckets": {
        "my-s3-bucket": {
          "name": "my-s3-bucket",
          "key_prefix": "my-optional-prefix/",
          "public_base_url": "https://my-gcs-bucket-cdn.example.com/my-optional-prefix"          
        }
      }
    }
  ]
}
```

- `region`: The region where the bucket is located.
- `name`: The full name of the bucket
- `key_prefix`: An optional prefix to apply to all keys in the bucket.
- `public_base_url`: A URL to use for public access to the bucket. This field is required if you configure your bucket to be public. Encore will append the object key to this URL when generating public URLs. The optional prefix will not be appended.

This guide covers typical infrastructure configurations. Adjust according to your specific requirements to optimize your Encore app's infrastructure setup.
