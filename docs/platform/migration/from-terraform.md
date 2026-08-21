---
seotitle: Coming to Encore from Terraform
seodesc: How Terraform resources map to Encore primitives, where each infrastructure setting is configured, and how to run Encore alongside existing Terraform.
title: Coming from Terraform
subtitle: How Terraform concepts map to Encore
lang: platform
---

Terraform and Encore both provision cloud infrastructure declaratively and both
end up calling the same cloud APIs, but they disagree about which artifact is
authoritative.

For a feature-level comparison, see [Encore compared to Terraform &
Pulumi](/docs/platform/other/vs-terraform).

## How it works

The same Postgres database, declared each way.

```hcl
resource "aws_db_instance" "site" {
  identifier              = "site"
  engine                  = "postgres"
  engine_version          = "16.3"
  instance_class          = "db.t4g.medium"
  allocated_storage       = 20
  storage_type            = "gp3"
  db_name                 = "site"
  username                = var.db_username
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.main.name
  vpc_security_group_ids  = [aws_security_group.db.id]
  backup_retention_period = 7
}
```

```ts
// Everything not declared here (instance class, storage, backups, networking)
// gets a default you can override per environment in the Encore Cloud dashboard.
const SiteDB = new SQLDatabase("site", {
  migrations: "./migrations",
});
```

The Terraform block has to spell out every property before it can create the
instance. Encore's declaration carries only the two the application depends on:
the database it connects to, and where its schema comes from.

## Where each setting is configured

Every setting lives in one of three places, and never in two at once.

| Place | Holds | How you set it |
|---|---|---|
| **Code** | Properties the code depends on | Arguments to the resource constructor, in your service directory |
| **Dashboard** | Properties that vary per environment | Per environment in the [Encore Cloud dashboard](https://app.encore.dev), under [infrastructure configuration](/docs/platform/infrastructure/configuration) |
| **Infra config** | Self-hosted deployments only | An [`infra.config.json`](/docs/ts/self-host/configure-infra) file passed to `encore build docker --config` |

The third row applies only when you self-host. Encore Cloud environments use the
dashboard instead; the two are alternatives, not layers.

For a SQL database:

| Setting | Place |
|---|---|
| Database name, migrations directory | Code |
| Instance class, CPU and memory | Dashboard |
| Storage size and type, IOPS | Dashboard |
| Engine version, parameter group | Dashboard |
| Backup retention, point-in-time recovery | Dashboard |
| VPC, subnet group, security group | Dashboard |
| Host, credentials, TLS, pool sizes | Infra config |

## Converting Terraform resources

### Databases

```hcl
resource "aws_db_instance" "site" { ... }
```

```ts
import { SQLDatabase } from "encore.dev/storage/sqldb";

const SiteDB = new SQLDatabase("site", { migrations: "./migrations" });
```

Schema changes go in `./migrations` as numbered SQL files, applied on deploy.
See [Databases](/docs/ts/primitives/databases).

### Pub/Sub topics and subscriptions

A topic, its queue, the subscription, and the queue policy granting the topic
write access:

```hcl
resource "aws_sns_topic" "site_added" { name = "site-added" }
resource "aws_sqs_queue" "check_site" { name = "check-site" }

resource "aws_sns_topic_subscription" "check_site" {
  topic_arn = aws_sns_topic.site_added.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.check_site.arn
}

resource "aws_sqs_queue_policy" "check_site" { ... }
```

```ts
import { Topic, Subscription } from "encore.dev/pubsub";

export const SiteAdded = new Topic<Site>("site-added", {
  deliveryGuarantee: "at-least-once",
});

new Subscription(SiteAdded, "check-site", { handler: doCheck });
```

Encore derives the queue, the subscription wiring and the IAM policy from those
two declarations. See [Pub/Sub](/docs/ts/primitives/pubsub).

### Object storage

```hcl
resource "aws_s3_bucket" "profile_pictures" { bucket = "profile-pictures" }
resource "aws_s3_bucket_versioning" "profile_pictures" { ... }
```

```ts
import { Bucket } from "encore.dev/storage/objects";

export const profilePictures = new Bucket("profile-pictures", {
  versioned: false,
});
```

See [Object storage](/docs/ts/primitives/object-storage).

### Scheduled tasks

```hcl
resource "aws_cloudwatch_event_rule" "welcome_email" {
  schedule_expression = "rate(2 hours)"
}
resource "aws_cloudwatch_event_target" "welcome_email" { ... }
resource "aws_lambda_permission" "welcome_email" { ... }
```

```ts
import { CronJob } from "encore.dev/cron";

const _ = new CronJob("welcome-email", {
  title: "Send welcome emails",
  every: "2h",
  endpoint: sendWelcomeEmail,
});
```

The target is a reference to the endpoint function rather than an ARN. See [Cron
jobs](/docs/ts/primitives/cron-jobs).

### Secrets

```hcl
resource "aws_secretsmanager_secret" "github_token" { name = "GitHubAPIToken" }
```

```ts
import { secret } from "encore.dev/config";

const githubToken = secret("GitHubAPIToken");
```

Values are set per environment with `encore secret set`. See
[Secrets](/docs/ts/primitives/secrets).

## Concept mapping

**Declaring infrastructure**

| Terraform | Encore |
|---|---|
| `resource` block | Resource constructor in application code |
| `provider` block | Cloud provider is chosen per environment in the dashboard |
| Resource reference (`aws_sns_topic.x.arn`) | Language-level import of the resource object |
| `depends_on` | Derived from imports between resources |
| Modules | Services |
| `count` and `for_each` | No equivalent; see [Trade-offs](#trade-offs) |

**Configuration and state**

| Terraform | Encore |
|---|---|
| Input variables and `locals` | Ordinary constants in application code |
| `output` and `var` plumbing | Not required; the consumer imports the object |
| `tfvars` per environment | Per-environment settings in the Encore Cloud dashboard |
| Workspaces | Environments |
| State file | No equivalent; the model is derived from source each build |
| Remote state backend | No equivalent; there is no state to store |

**Running it**

| Terraform | Encore |
|---|---|
| `terraform plan` | Build-time validation against the derived model |
| `terraform apply` | `git push` to deploy, or `docker run` with an `infra.config.json` |
| `terraform import` | [Import an existing resource](/docs/platform/infrastructure/aws/import-rds) when creating an environment |
| `data` source for an Encore resource | [Encore Terraform Provider](/docs/platform/integrations/terraform) data sources |
| Provisioners for migrations | `migrations/` directory, applied on deploy |
| Hand-written IAM policies | Derived from the call graph |

## Using Encore with Terraform

Most teams run both, with Encore managing application infrastructure and
Terraform managing what is not an application, such as DNS, CDN, org-level
networking, and third-party providers. Which path applies depends on which side
owns the resource.

### Pointing Encore at resources you already have

When creating an environment you can import an existing RDS instance, Cloud SQL
instance, S3 or GCS bucket, SNS or Pub/Sub topic, Kubernetes cluster, or an
entire GCP project, rather than having Encore provision a new one. See [Import
an existing AWS RDS instance](/docs/platform/infrastructure/aws/import-rds).

### Reading Encore resources from Terraform

The [Encore Terraform Provider](/docs/platform/integrations/terraform) exposes
data sources such as `encore_database`, `encore_cache`, and
`encore_pubsub_topic`, so Terraform can reference infrastructure Encore created:

```hcl
data "encore_pubsub_topic" "topic" {
  name = "my-topic"
  env  = "my-env"
}

resource "aws_iot_topic_rule" "rule" {
  name = "my-rule"
  sql  = "SELECT * FROM 'my-topic'"
  sns {
    message_format = "RAW"
    role_arn       = aws_iam_role.role.arn
    target_arn     = data.encore_pubsub_topic.topic.aws_sns.arn
  }
}
```

### Self-hosting and binding manually

Build a Docker image with `encore build docker` and pass an
`infra.config.json` that maps each resource in the derived model to a physical
resource you provision however you like:

```bash
encore build docker myapp:latest --config ./infra.config.json
```

Running the build without `--config` prints the resources the file has to
account for. See [Configure
infrastructure](/docs/ts/self-host/configure-infra).

## Trade-offs

**Resource kinds are fixed.** Encore declares SQL databases, Pub/Sub topics and
subscriptions, object storage buckets, caches, cron jobs, and secrets. Each kind
is configurable in depth, but anything outside that set is provisioned with
Terraform and connected through the Encore Terraform Provider.

**Resource counts are fixed at build time.** Because the model is derived by
static analysis, resource names must be string literals and resources cannot be
created in a loop or conditionally, so there is no equivalent of `count` or
`for_each`. Infrastructure whose shape depends on runtime data belongs outside
the application model.

**Cloud support is narrower.** Encore provisions to AWS and GCP, where Terraform
reaches essentially every provider.

## Why configuration is split this way

A property goes in application code when the code depends on it. The database
name and the migrations directory decide which database the application connects
to and what schema it expects, so both belong with the code. Instance class only
changes cost and capacity, and it differs between environments by definition,
since a development environment sized like production would be a waste of money.
Terraform handles that difference with workspaces, variable files and
per-environment `tfvars`; Encore handles it by keeping those properties out of
the declaration in the first place.

That split has four consequences worth knowing about.

**New environments are close to free.** Because the declaration contains no
cloud specifics, creating an environment is a matter of choosing what backs it
rather than writing a new configuration. This is what makes [preview
environments](/docs/platform/deploy/preview-environments) practical on every
pull request.

**Capacity changes do not go through code review.** Resizing an instance or
adjusting scaling bounds happens in the Encore Cloud dashboard under
[infrastructure configuration](/docs/platform/infrastructure/configuration), so
it needs no pull request, no rebuild, and no deploy. The reverse holds too: a pull request that
adds a database shows up as one line of application code rather than a large
diff of provider configuration.

**Coding agents can add infrastructure without being able to break it.** An
agent writing application code can declare a new database or Pub/Sub topic, and
Encore Cloud provisions it with [guardrails](/docs/platform/ai-integration) such
as encryption at rest, automated backups, dead letter queues, and
least-privilege IAM. The same agent cannot resize a production instance or widen
a security group, because those settings are not reachable from the code it
edits. Infrastructure changes still arrive as reviewable pull requests, and
[infrastructure namespaces](/docs/ts/cli/infra-namespaces) give parallel agents
isolated local state.

**The application stays portable.** The same declarations run on AWS, on GCP,
and on a laptop, because nothing in them names a cloud service.

Keeping cloud specifics out of the declaration is also what makes the
infrastructure model derivable in the first place. Resource references are
ordinary imports, so Encore reads the application's own dependency graph to
determine which resources exist and which services use them, and uses that to
generate least-privilege IAM policies. See [Encore Application
Model](/docs/ts/concepts/application-model).
